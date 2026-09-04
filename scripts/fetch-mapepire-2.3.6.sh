#!/usr/bin/env bash
set -euo pipefail

readonly REPOSITORY='Mapepire-IBMi/mapepire-server'
readonly TAG='v2.3.6'
readonly COMMIT='e39e54e7d3779ee513e0cab1f62acea095219c3b'
readonly JAR_URL="https://github.com/${REPOSITORY}/releases/download/${TAG}/mapepire-server.jar"
readonly SOURCE_URL="https://github.com/${REPOSITORY}/archive/${COMMIT}.tar.gz"
readonly JAR_SHA256='6371d64f5684fcbee96f27107512f712fc1676ffded00726f2752dcfc30977b7'
readonly JAR_SIZE=3972800
readonly SOURCE_SHA256='7eb28520ae7335574a8b5b486742a15c7be1878642ea3839e2d33683960803a9'
readonly SOURCE_SIZE=344700

if [[ $# != 1 || "$1" != /* ]]; then
  printf 'usage: %s ABSOLUTE_OUTPUT_DIRECTORY\n' "$0" >&2
  exit 2
fi
output="$1"
mkdir -p "$output"
writer="$(dirname "$0")/write-bounded.py"
download() {
  curl --fail --location --proto '=https' --tlsv1.2 --connect-timeout 10 --max-time 60 --max-filesize "$2" --retry 2 --retry-delay 1 --output - "$1" |
    "$writer" "$3" "$2"
}
download "https://api.github.com/repos/${REPOSITORY}/git/ref/tags/${TAG}" 4096 "${output}/tag.json"
download "$JAR_URL" "$JAR_SIZE" "${output}/mapepire-server.jar"
download "$SOURCE_URL" "$SOURCE_SIZE" "${output}/mapepire-server-${COMMIT}.tar.gz"
"$(dirname "$0")/verify-mapepire-2.3.6.py" "$output" "$COMMIT" "$JAR_SHA256" "$JAR_SIZE" "$SOURCE_SHA256" "$SOURCE_SIZE"
rm -f "${output}/tag.json"
