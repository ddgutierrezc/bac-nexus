#!/usr/bin/env bash
set -euo pipefail

root="$(mktemp -d)"
trap 'rm -rf "$root"' EXIT
for target in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64 windows-arm64; do
  mkdir -p "$root/platforms/$target"
  binary=nexus; [[ $target == windows-* ]] && binary=nexus.exe
  printf '%s\n' "$target" >"$root/platforms/$target/$binary"
  printf '{}\n' >"$root/platforms/$target/nexus.manifest.json"
done
printf 'jar\n' >"$root/jar"
printf 'source\n' >"$root/source"
printf 'license\n' >"$root/license"
"$(dirname "$0")/package-handoff.sh" --fixture "$root/platforms" "$root/jar" "$root/source" "$root/license" "$root/handoff.zip"
python3 - "$root/handoff.zip" <<'PY'
import hashlib, sys, zipfile
with zipfile.ZipFile(sys.argv[1]) as archive:
    names = set(archive.namelist())
    required = {"LICENSE-GPL-3.0.txt", "NOTICE.txt", "SOURCE.md", "SHA256SUMS", "components/mapepire/2.3.6/mapepire-server.jar", "components/mapepire/2.3.6/mapepire-server-source-e39e54e7d3779ee513e0cab1f62acea095219c3b.tar.gz"}
    required |= {f"platforms/{target}/{'nexus.exe' if target.startswith('windows') else 'nexus'}" for target in ("linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64", "windows-amd64", "windows-arm64")}
    if names != required | {name.rsplit('/', 1)[0] + "/nexus.manifest.json" for name in required if name.startswith("platforms/")}:
        raise SystemExit("fixture archive topology is not exact")
    if any(info.is_dir() or (info.external_attr >> 16) & 0o170000 == 0o120000 for info in archive.infolist()):
        raise SystemExit("archive contains a directory or symlink entry")
    for record in archive.read("SHA256SUMS").decode().splitlines():
        digest, name = record.split("  ", 1)
        if hashlib.sha256(archive.read(name)).hexdigest() != digest:
            raise SystemExit(f"checksum mismatch: {name}")
PY
