#!/usr/bin/env bash
set -euo pipefail

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT
commit='e39e54e7d3779ee513e0cab1f62acea095219c3b'
fetch="$(dirname "$0")/fetch-mapepire-2.3.6.sh"
printf '{"object":{"type":"commit","sha":"%s"}}\n' "$commit" >"$root/tag.json"
printf 'jar\n' >"$root/mapepire-server.jar"
python3 - "$root/mapepire-server-${commit}.tar.gz" <<'PY'
import io, sys, tarfile
with tarfile.open(sys.argv[1], "w:gz") as archive:
    data = b"GNU GENERAL PUBLIC LICENSE\nVersion 3\n"
    member = tarfile.TarInfo("mapepire-server/LICENSE")
    member.size = len(data)
    archive.addfile(member, io.BytesIO(data))
PY
jar_hash="$(sha256sum "$root/mapepire-server.jar" | cut -d ' ' -f 1)"
source="$root/mapepire-server-${commit}.tar.gz"
source_hash="$(sha256sum "$source" | cut -d ' ' -f 1)"
source_size="$(wc -c <"$source")"
verify="$(dirname "$0")/verify-mapepire-2.3.6.py"
writer="$(dirname "$0")/write-bounded.py"
printf 'four' | "$writer" "$root/bounded" 4
test "$(wc -c <"$root/bounded")" = 4
if printf 'five!' | "$writer" "$root/oversized-download" 4; then exit 1; fi
test ! -e "$root/oversized-download"
"$verify" "$root" "$commit" "$jar_hash" 4 "$source_hash" "$source_size"
test -f "$root/LICENSE-GPL-3.0.txt"
if "$verify" "$root" "$commit" "$jar_hash" 3 "$source_hash" "$source_size"; then exit 1; fi
if "$verify" "$root" "$commit" "$jar_hash" 4 "$source_hash" "$((source_size - 1))"; then exit 1; fi

mkdir "$root/tmp"
fake_curl="$root/fake-curl"
fake_verify="$root/fake-verify"
python3 - "$fake_curl" "$fake_verify" <<'PY'
import pathlib, sys
pathlib.Path(sys.argv[1]).write_text('''#!/usr/bin/env bash
set -euo pipefail
url="${!#}"
config=''
for ((index = 1; index <= $#; index++)); do
  if [[ "${!index}" == '--config' ]]; then
    next=$((index + 1))
    config="${!next}"
  fi
done
case "$url" in
  *api.github.com*)
    kind=api
    ;;
  *mapepire-server.jar)
    kind=jar
    ;;
  *)
    kind=source
    ;;
esac
record="$FAKE_CURL_RECORDS/$kind"
printf '%q\\n' "$@" >"$record.argv"
tr '\\0' '\\n' </proc/self/environ >"$record.env"
case "$kind" in
  api)
    [[ -n "$config" && "$(stat -c '%a' "$config")" == 600 ]]
    header="$(<"$config")"
    [[ "$(printf '%s' "$header" | sha256sum | cut -d ' ' -f 1)" == "$FAKE_HEADER_SHA" ]]
    printf 'config=authenticated\\n' >"$record.state"
    [[ "${FAKE_CURL_FAIL:-}" != api ]] || exit 22
    if [[ "${FAKE_CURL_BLOCK:-}" == api ]]; then
      ps -o sid= -p "$$" | tr -d ' ' >"$FAKE_SESSION_ID"
      : >"$FAKE_CURL_READY"
      while kill -0 "$PPID" 2>/dev/null; do read -r -t 0.1 _ || :; done
      exit 22
    fi
    printf '{"object":{"type":"commit","sha":"e39e54e7d3779ee513e0cab1f62acea095219c3b"}}\\n'
    ;;
  jar)
    [[ -z "$config" ]]
    printf 'config=absent\\n' >"$record.state"
    [[ "${FAKE_CURL_FAIL:-}" != jar ]] || exit 22
    printf 'jar\\n'
    ;;
  source)
    [[ -z "$config" ]]
    printf 'config=absent\\n' >"$record.state"
    [[ "${FAKE_CURL_FAIL:-}" != source ]] || exit 22
    printf 'source\\n'
    ;;
esac
''')
pathlib.Path(sys.argv[2]).write_text('''#!/usr/bin/env bash
set -euo pipefail
[[ "${FAKE_VERIFY_FAIL:-}" != 1 ]] || exit 1
''')
PY
chmod +x "$fake_curl" "$fake_verify"
canary='canary-release-provenance-token'
header_sha="$(printf 'header = \"Authorization: Bearer %s\"' "$canary" | sha256sum | cut -d ' ' -f 1)"
run_fetch() {
  GITHUB_ACTIONS="${1:-}" GITHUB_TOKEN="${2:-}" TMPDIR="$root/tmp" MAPEPIRE_CURL_BIN="$fake_curl" MAPEPIRE_VERIFY_BIN="$fake_verify" FAKE_HEADER_SHA="$header_sha" FAKE_CURL_RECORDS="$root/records" FAKE_CURL_READY="$root/ready" FAKE_SESSION_ID="$root/session" FAKE_CURL_BLOCK="${3:-}" "$fetch" "$root/output" >"$root/stdout" 2>"$root/stderr"
}
assert_clean() {
  python3 - "$root" <<'PY'
import pathlib, sys
root, token = pathlib.Path(sys.argv[1]), "canary-release-provenance-token"
for path in root.rglob("*"):
    if path.is_file() and token in path.read_text(errors="ignore"):
        raise SystemExit(f"token leaked to {path.name}")
if any((root / "tmp").iterdir()):
    raise SystemExit("temporary authentication file was not removed")
PY
}
assert_curl_records() {
  python3 - "$root/records" <<'PY'
import pathlib, sys
records, token = pathlib.Path(sys.argv[1]), "canary-release-provenance-token"
for kind, state in {"api": "config=authenticated", "jar": "config=absent", "source": "config=absent"}.items():
    assert (records / f"{kind}.state").read_text() == state + "\n"
    for suffix in ("argv", "env"):
        text = (records / f"{kind}.{suffix}").read_text()
        assert token not in text and "GITHUB_TOKEN=" not in text and "api_token=" not in text
PY
}

mkdir "$root/records"
run_fetch true "$canary"
assert_curl_records
assert_clean
rm -rf "$root/output" "$root/records"; mkdir "$root/records"
if run_fetch true ''; then exit 1; fi
test ! -e "$root/records/api.argv"
assert_clean
rm -rf "$root/output"
if FAKE_CURL_FAIL=api run_fetch true "$canary"; then exit 1; fi
assert_clean
rm -rf "$root/output" "$root/records"; mkdir "$root/records"
if FAKE_VERIFY_FAIL=1 run_fetch true "$canary"; then exit 1; fi
assert_clean
rm -rf "$root/output" "$root/records"; mkdir "$root/records"
GITHUB_ACTIONS=true GITHUB_TOKEN="$canary" TMPDIR="$root/tmp" MAPEPIRE_CURL_BIN="$fake_curl" MAPEPIRE_VERIFY_BIN="$fake_verify" FAKE_HEADER_SHA="$header_sha" FAKE_CURL_RECORDS="$root/records" FAKE_CURL_READY="$root/ready" FAKE_SESSION_ID="$root/session" FAKE_CURL_BLOCK=api setsid "$fetch" "$root/output" >"$root/stdout" 2>"$root/stderr" & fetch_pid=$!
for _ in {1..100}; do [[ -e "$root/ready" ]] && break; sleep 0.01; done
test -e "$root/ready"
kill -TERM -- "-$(<"$root/session")"
if wait "$fetch_pid"; then exit 1; fi
assert_clean
