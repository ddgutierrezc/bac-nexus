#!/usr/bin/env bash
set -euo pipefail

readonly VERSION='2.3.6'
readonly COMMIT='e39e54e7d3779ee513e0cab1f62acea095219c3b'
readonly JAR_SHA256='6371d64f5684fcbee96f27107512f712fc1676ffded00726f2752dcfc30977b7'
readonly JAR_SIZE=3972800
readonly SOURCE_SHA256='7eb28520ae7335574a8b5b486742a15c7be1878642ea3839e2d33683960803a9'
readonly SOURCE_SIZE=344700

fixture=false
if [[ ${1:-} == --fixture ]]; then fixture=true; shift; fi
if [[ $# != 5 ]]; then
  printf 'usage: %s [--fixture] PLATFORM_ROOT JAR SOURCE LICENSE OUTPUT_ZIP\n' "$0" >&2
  exit 2
fi
python3 - "$fixture" "$VERSION" "$COMMIT" "$JAR_SHA256" "$JAR_SIZE" "$SOURCE_SHA256" "$SOURCE_SIZE" "$@" <<'PY'
import hashlib, pathlib, shutil, stat, sys, tempfile, zipfile

fixture, version, commit, jar_hash, jar_size, source_hash, source_size, platform_root, jar, source, license_file, output = sys.argv[1:]
fixture = fixture == "true"
platform_root, jar, source, license_file, output = map(pathlib.Path, (platform_root, jar, source, license_file, output))
targets = (("linux", "amd64"), ("linux", "arm64"), ("darwin", "amd64"), ("darwin", "arm64"), ("windows", "amd64"), ("windows", "arm64"))
def regular(path):
    return path.is_file() and not path.is_symlink() and stat.S_ISREG(path.stat().st_mode)
def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()
if not platform_root.is_dir() or platform_root.is_symlink() or not all(regular(path) for path in (jar, source, license_file)):
    raise SystemExit("handoff inputs must be regular files below a real platform root")
if not fixture and (jar.stat().st_size != int(jar_size) or source.stat().st_size != int(source_size) or digest(jar) != jar_hash or digest(source) != source_hash):
    raise SystemExit("Mapepire provenance verification failed")
expected = []
for goos, goarch in targets:
    directory = platform_root / f"{goos}-{goarch}"
    binary = directory / ("nexus.exe" if goos == "windows" else "nexus")
    manifest = directory / "nexus.manifest.json"
    if not all(regular(path) for path in (binary, manifest)):
        raise SystemExit(f"missing or unsafe platform payload: {goos}-{goarch}")
    expected.extend((binary, manifest))
if any(path.is_symlink() for path in platform_root.rglob("*")):
    raise SystemExit("platform root contains a symlink")
notice = "Mapepire Server v2.3.6 is a separate GPLv3 component. BAC Nexus is not relicensed by this notice.\n"
source_note = ("Corresponding Source: this bundle includes the exact upstream source archive for Mapepire Server v2.3.6.\n"
               f"Repository: https://github.com/Mapepire-IBMi/mapepire-server\nTag: v{version}\nCommit: {commit}\n"
               f"JAR: https://github.com/Mapepire-IBMi/mapepire-server/releases/download/v{version}/mapepire-server.jar\n"
               f"JAR SHA-256: {jar_hash}\nJAR bytes: {jar_size}\n"
               f"Source: https://github.com/Mapepire-IBMi/mapepire-server/archive/{commit}.tar.gz\n"
               f"Source SHA-256: {source_hash}\n"
               "The source archive is an exact-commit GitHub archive, not an immutable release asset; release builds re-verify its observed digest.\n")
if fixture:
    notice = "Fixture-only handoff. Not for redistribution.\n"
    source_note = "Fixture-only handoff. Not for redistribution.\n"
with tempfile.TemporaryDirectory() as temporary:
    root = pathlib.Path(temporary) / "nexus-handoff"
    for path in expected:
        relative = pathlib.Path("platforms") / path.relative_to(platform_root)
        destination = root / relative
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(path, destination)
    component = root / "components" / "mapepire" / version
    component.mkdir(parents=True)
    shutil.copyfile(jar, component / "mapepire-server.jar")
    shutil.copyfile(source, component / f"mapepire-server-source-{commit}.tar.gz")
    (root / "LICENSE-GPL-3.0.txt").write_bytes(license_file.read_bytes())
    (root / "NOTICE.txt").write_text(notice, encoding="utf-8")
    (root / "SOURCE.md").write_text(source_note, encoding="utf-8")
    entries = sorted(path for path in root.rglob("*") if path.is_file())
    (root / "SHA256SUMS").write_text("".join(f"{digest(path)}  {path.relative_to(root).as_posix()}\n" for path in entries), encoding="utf-8")
    output.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(output, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9) as archive:
        for path in sorted(root.rglob("*")):
            if path.is_file():
                info = zipfile.ZipInfo(path.relative_to(root).as_posix(), (1980, 1, 1, 0, 0, 0))
                info.external_attr = (0o100644 << 16)
                archive.writestr(info, path.read_bytes(), compress_type=zipfile.ZIP_DEFLATED, compresslevel=9)
    pathlib.Path(f"{output}.sha256").write_text(f"{digest(output)}  {output.name}\n", encoding="utf-8")
PY
