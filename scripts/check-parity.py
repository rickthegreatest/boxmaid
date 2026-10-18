#!/usr/bin/env python3
"""Check both compiled CLIs against shared fixtures and their public contract."""

import argparse
import base64
import json
import os
from pathlib import Path
import subprocess
import sys
import zlib


ROOT = Path(__file__).resolve().parents[1]
SOURCE = b"flowchart LR\nA[Source]-->B[Layout]-->C[Text]\n"


def invoke(binary, args, source=SOURCE):
    env = dict(os.environ, NO_COLOR="1", COLUMNS="80")
    result = subprocess.run(
        [str(binary), *args], input=source, capture_output=True,
        cwd=ROOT, env=env, timeout=20, check=False,
    )
    return result.returncode, result.stdout, result.stderr


def compare(binaries, args, source=SOURCE):
    results = [invoke(binary, args, source) for binary in binaries]
    if results[0] != results[1]:
        raise AssertionError(f"CLI mismatch for {args!r}\nGo: {results[0]!r}\nRust: {results[1]!r}")
    return results[0]


def check(binaries):
    count = 0
    for case in sorted((ROOT / "testdata/cases").glob("*.mmd")):
        path = case.relative_to(ROOT).as_posix()
        for width in (0, 30, 60, 120):
            for mode in ([], ["-ascii"]):
                compare(binaries, ["-w", str(width), "-color", "never", *mode, path])
                count += 1

    for theme in ("default", "mono", "dracula", "nord"):
        status, out, err = compare(binaries, ["-theme", theme, "-color", "always", "-w", "80"])
        assert status == 0 and b"\x1b[" in out and not err, (theme, status, out, err)

    for args, expected in [
        (["-help"], 0), (["--help"], 0), (["-version"], 0), (["-list"], 0),
        (["-w", "-1"], 2), (["-w", "invalid"], 2), (["-w"], 2),
        (["-color", "sometimes"], 2), (["-theme", "missing"], 2),
        (["-md", "-live"], 2), (["-md", "-require-fit"], 2),
        (["-require-fit"], 2), (["-require-fit", "-w", "1"], 3),
        (["-w", "1"], 0), (["-unknown"], 2),
    ]:
        for binary in binaries:
            status, out, err = invoke(binary, args)
            assert status == expected, (binary, args, status, out, err)
            if expected in (2, 3):
                assert not out and err, (binary, args, out, err)

    for args in (["-version"], ["-list"], ["-require-fit", "-w", "1"], ["-ascii", "-w", "80"], ["--", "-"]):
        compare(binaries, args)

    for source in [
        b"```mermaid\nflowchart LR\nA-->B\n```\n",
        b"````markdown\n```mermaid\nflowchart LR\nA-->B\n```\n````\n",
        b"```mermaid\nflowchart LR\nA-->B\n~~~\n",
        b"  ~~~mermaid\r\n  flowchart LR\r\n  A-->B\r\n  ~~~~\r\n",
        (ROOT / "README.md").read_bytes(),
    ]:
        compare(binaries, ["-md", "-w", "80"], source)

    for source in (b"", b"zenuml\nAlice->Bob: hello\n"):
        status, out, err = compare(binaries, [], source)
        assert status == 1 and not out and err, (status, out, err)

    status, out, err = compare(binaries, [
        "docs/examples/pipeline.mmd", "-", "docs/examples/pipeline.mmd",
    ], b"zenuml\n")
    assert status == 1 and out.count(b"Mermaid source") == 2 and err, (status, out, err)

    for binary in binaries:
        # Close stdout before delivering stdin, so the renderer actually hits EPIPE.
        with subprocess.Popen([str(binary)], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, cwd=ROOT) as process:
            process.stdout.close()
            process.stdin.write(SOURCE)
            process.stdin.close()
            diagnostic = process.stderr.read()
            status = process.wait(timeout=20)
            assert status == 0 and not diagnostic, (binary, status, diagnostic)
        status, out, err = invoke(binary, [], b"flowchart LR\n\xff")
        assert status == 2 and not out and err, (binary, status, out, err)
        status, out, err = invoke(binary, ["-live"])
        assert status == 0 and not err
        payload = out.decode().strip().split("#pako:", 1)[1]
        state = json.loads(zlib.decompress(base64.urlsafe_b64decode(payload + "=" * (-len(payload) % 4))))
        assert state["code"] == SOURCE.decode(), state

    print(f"Passed: {count} fixture/width/ASCII comparisons, themes, CLI contracts, Markdown, and browser-link round trips.")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--go", required=True, type=Path, help="compiled Go CLI")
    parser.add_argument("--rust", required=True, type=Path, help="compiled Rust CLI")
    args = parser.parse_args()
    binaries = [args.go.resolve(), args.rust.resolve()]
    for binary in binaries:
        if not binary.is_file():
            parser.error(f"binary does not exist: {binary}")
    try:
        check(binaries)
    except (AssertionError, subprocess.TimeoutExpired, OSError, ValueError) as error:
        print(error, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
