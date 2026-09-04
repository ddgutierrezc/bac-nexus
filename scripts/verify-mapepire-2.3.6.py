#!/usr/bin/env python3
import hashlib
import json
import pathlib
import sys
import tarfile

TAG_MAX_BYTES = 4096
LICENSE_MAX_BYTES = 65536


def fail(message):
    raise SystemExit(message)


def read_limited(path, maximum):
    if path.stat().st_size > maximum:
        fail(f"unexpected size for {path.name}")
    with path.open("rb") as file:
        return file.read(maximum + 1)


def verify_payload(path, expected_hash, expected_size):
    if path.stat().st_size != expected_size:
        fail(f"unexpected size for {path.name}")
    digest = hashlib.sha256()
    with path.open("rb") as file:
        for chunk in iter(lambda: file.read(65536), b""):
            digest.update(chunk)
    if digest.hexdigest() != expected_hash:
        fail(f"unexpected SHA-256 for {path.name}")


def main(arguments):
    if len(arguments) != 6:
        fail("usage: verify-mapepire-2.3.6.py ROOT COMMIT JAR_SHA256 JAR_SIZE SOURCE_SHA256 SOURCE_SIZE")
    root = pathlib.Path(arguments[0])
    commit, jar_hash, jar_size, source_hash, source_size = arguments[1:]
    try:
        tag = json.loads(read_limited(root / "tag.json", TAG_MAX_BYTES).decode("utf-8"))
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as error:
        fail(f"Mapepire tag metadata is invalid: {error}")
    if tag.get("object", {}).get("type") != "commit" or tag["object"].get("sha") != commit:
        fail("Mapepire tag does not resolve to the pinned commit")
    jar = root / "mapepire-server.jar"
    source = root / f"mapepire-server-{commit}.tar.gz"
    try:
        verify_payload(jar, jar_hash, int(jar_size))
        verify_payload(source, source_hash, int(source_size))
        with tarfile.open(source, "r:gz") as archive:
            members = archive.getmembers()
            unsafe = any(member.issym() or member.islnk() or member.isdev() or member.name.startswith("/") or ".." in pathlib.PurePosixPath(member.name).parts for member in members)
            license_member = next((member for member in members if member.name.endswith("/LICENSE") and member.isfile()), None)
            if not members or unsafe or license_member is None or license_member.size > LICENSE_MAX_BYTES:
                fail("Mapepire source archive is unsafe or has no bounded LICENSE file")
            license_data = archive.extractfile(license_member).read(LICENSE_MAX_BYTES + 1)
    except (OSError, tarfile.TarError) as error:
        fail(f"Mapepire payload verification failed: {error}")
    if b"GNU GENERAL PUBLIC LICENSE" not in license_data or b"Version 3" not in license_data:
        fail("Mapepire source archive LICENSE is not GPLv3")
    (root / "LICENSE-GPL-3.0.txt").write_bytes(license_data)


main(sys.argv[1:])
