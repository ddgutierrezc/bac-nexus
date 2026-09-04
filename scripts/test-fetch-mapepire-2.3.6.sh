#!/usr/bin/env bash
set -euo pipefail

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT
commit='e39e54e7d3779ee513e0cab1f62acea095219c3b'
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
