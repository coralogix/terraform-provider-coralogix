#!/usr/bin/env python3
"""CHANGELOG.md helpers for the release-provider workflow.

  changelog.py cut VERSION              Turn "# Unreleased" into "# Release VERSION" and
                                        add a fresh empty "# Unreleased" above it.
  changelog.py check VERSION            Fail unless "# Release VERSION" has entries.
"""

import argparse
import re
import sys

UNRELEASED = "# Unreleased"


def read(path):
    with open(path, encoding="utf-8") as f:
        return f.read()


def section(text, heading):
    """Return the lines between `heading` and the next top-level heading, or None."""
    lines = text.splitlines()
    try:
        start = lines.index(heading)
    except ValueError:
        return None
    body = []
    for line in lines[start + 1:]:
        if line.startswith("# "):
            break
        body.append(line)
    return body


def has_entries(body):
    return any(line.lstrip().startswith("- ") for line in body)


def cut(path, version):
    text = read(path)
    release = f"# Release {version}"
    if section(text, release) is not None:
        sys.exit(f"{path} already has a '{release}' section")
    body = section(text, UNRELEASED)
    if body is None:
        sys.exit(f"{path} has no '{UNRELEASED}' section")
    if not has_entries(body):
        sys.exit(f"'{UNRELEASED}' in {path} has no entries; there is nothing to release")
    text = text.replace(UNRELEASED + "\n", f"{UNRELEASED}\n\n{release}\n", 1)
    with open(path, "w", encoding="utf-8") as f:
        f.write(text)


def check(path, version):
    body = section(read(path), f"# Release {version}")
    if body is None or not has_entries(body):
        sys.exit(f"{path} has no entries under '# Release {version}'")


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--file", default="CHANGELOG.md")
    sub = parser.add_subparsers(dest="command", required=True)
    for name in ("cut", "check"):
        sub.add_parser(name).add_argument("version")
    args = parser.parse_args()
    if not re.fullmatch(r"(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)", args.version):
        sys.exit(f"version {args.version!r} is not MAJOR.MINOR.PATCH")
    if args.command == "cut":
        cut(args.file, args.version)
    else:
        check(args.file, args.version)


if __name__ == "__main__":
    main()
