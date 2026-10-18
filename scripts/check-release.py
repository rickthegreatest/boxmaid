#!/usr/bin/env python3
"""Reject release tags that disagree with package metadata or release notes."""

import argparse
import json
from pathlib import Path
import re
import subprocess
import sys


def check(tag, allow_unreleased=False):
    root = Path(__file__).resolve().parents[1]
    if not re.fullmatch(r"v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?", tag):
        raise ValueError("expected a semantic version tag such as v0.1.0")
    version = tag[1:]
    metadata = json.loads(subprocess.check_output([
        "cargo", "metadata", "--locked", "--no-deps", "--format-version", "1",
        "--manifest-path", str(root / "rust/Cargo.toml"),
    ], text=True))
    package = next(p for p in metadata["packages"] if p["name"] == "boxmaid")
    if package["version"] != version:
        raise ValueError("tag and Rust package version differ")
    go_cli = (root / "cmd/boxmaid/main.go").read_text()
    if f'var version = "{version}"' not in go_cli:
        raise ValueError("tag and Go CLI version differ")
    tui = (root / "tui/go.mod").read_text()
    if f"github.com/rickthegreatest/boxmaid v{version}" not in tui:
        raise ValueError("TUI dependency must point to this core release")
    changelog = (root / "CHANGELOG.md").read_text()
    heading = re.search(rf"^## {re.escape(version)} — (.+)$", changelog, re.MULTILINE)
    if not heading or not (re.fullmatch(r"\d{4}-\d{2}-\d{2}", heading[1]) or allow_unreleased and heading[1] == "unreleased"):
        raise ValueError("replace the changelog's unreleased marker with the release date")
    suffix = " (not yet dated for publication)" if heading[1] == "unreleased" else ""
    print(f"Release metadata agrees on {tag}{suffix}.")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("tag", help="core release tag, e.g. v0.1.0")
    parser.add_argument("--allow-unreleased", action="store_true", help="check metadata during preparation, before dating the release")
    args = parser.parse_args()
    try:
        check(args.tag, args.allow_unreleased)
    except (ValueError, OSError, subprocess.CalledProcessError, StopIteration) as error:
        sys.exit(str(error))
