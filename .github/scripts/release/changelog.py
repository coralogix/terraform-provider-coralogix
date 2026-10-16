#!/usr/bin/env python3
"""CHANGELOG.md helpers for the release-provider workflow.

  changelog.py cut VERSION              Turn "# Unreleased" into "# Release VERSION" and
                                        add a fresh empty "# Unreleased" above it.
  changelog.py check VERSION            Fail unless "# Release VERSION" has entries.
  changelog.py slack VERSION URL...     Print a Slack webhook payload that summarizes
                                        the release (FEAT, FIX and BREAKING entries).
"""

import argparse
import json
import re
import sys

UNRELEASED = "# Unreleased"
MAX_SLACK_ENTRIES = 25
MAX_ENTRY_CHARS = 200


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


def summarize(body):
    """Group FEAT, FIX and BREAKING entries by their #### component heading."""
    groups, component = [], None
    for line in body:
        if line.startswith("#### "):
            component = line[5:].strip()
            continue
        m = re.match(r"\s*-\s*(BREAKING[^:]*|FEAT|FIX):\s*(.+)", line)
        # Internal tooling (tools/iac-codegen) does not change what users see.
        if not m or component is None or component.startswith("tools/"):
            continue
        entry = m.group(2).strip()
        # The first sentence is the headline; the rest is detail for the release notes.
        entry = re.split(r"(?<=[.!?])\s", entry, maxsplit=1)[0]
        if len(entry) > MAX_ENTRY_CHARS:
            entry = entry[: MAX_ENTRY_CHARS - 1].rstrip() + "…"
        if not groups or groups[-1][0] != component:
            groups.append((component, []))
        groups[-1][1].append(f"{m.group(1)}: {entry}")
    return groups


def slack(path, version, release_url, registry_url):
    body = section(read(path), f"# Release {version}")
    if body is None:
        sys.exit(f"{path} has no '# Release {version}' section")
    lines = [f"*Coralogix Terraform provider v{version} is on the Terraform Registry*", ""]
    shown = total = 0
    for component, entries in summarize(body):
        total += len(entries)
        if shown >= MAX_SLACK_ENTRIES:
            continue
        lines.append(f"*{component}*")
        for entry in entries[: MAX_SLACK_ENTRIES - shown]:
            lines.append(f"• {entry}")
            shown += 1
    if total == 0:
        lines.append("Maintenance release: no new features or fixes.")
    elif shown < total:
        lines.append(f"_…and {total - shown} more._")
    lines += ["", f"<{release_url}|Release notes> · <{registry_url}|Registry docs>"]
    print(json.dumps({"text": "\n".join(lines)}))


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--file", default="CHANGELOG.md")
    sub = parser.add_subparsers(dest="command", required=True)
    for name in ("cut", "check", "slack"):
        p = sub.add_parser(name)
        p.add_argument("version")
        if name == "slack":
            p.add_argument("release_url")
            p.add_argument("registry_url")
    args = parser.parse_args()
    if not re.fullmatch(r"(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)", args.version):
        sys.exit(f"version {args.version!r} is not MAJOR.MINOR.PATCH")
    if args.command == "cut":
        cut(args.file, args.version)
    elif args.command == "check":
        check(args.file, args.version)
    else:
        slack(args.file, args.version, args.release_url, args.registry_url)


if __name__ == "__main__":
    main()
