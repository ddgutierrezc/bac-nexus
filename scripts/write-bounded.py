#!/usr/bin/env python3
import pathlib
import sys


def main(arguments):
    if len(arguments) != 2:
        raise SystemExit("usage: write-bounded.py OUTPUT MAXIMUM_BYTES")
    output = pathlib.Path(arguments[0])
    maximum = int(arguments[1])
    if maximum < 0:
        raise SystemExit("maximum bytes must not be negative")
    written = 0
    try:
        with output.open("xb") as destination:
            while True:
                chunk = sys.stdin.buffer.read(min(65536, maximum - written + 1))
                if not chunk:
                    break
                written += len(chunk)
                if written > maximum:
                    raise ValueError("download exceeds byte cap")
                destination.write(chunk)
    except (OSError, ValueError) as error:
        output.unlink(missing_ok=True)
        raise SystemExit(str(error))


main(sys.argv[1:])
