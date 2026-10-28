#!/usr/bin/env python3
"""Version and changelog tool for provider releases. Every merge that bumps
internal/clientset/version.go is released (release-provider.yml).

  release.py bump [patch|minor|major]   Set the next version in version.go and move the
                                        "# Unreleased" entries under "# Release X.Y.Z".
                                        `make bump`, `make bump BUMP=minor`.
  release.py check BASE_REF             Fail unless the change since BASE_REF is a valid
                                        release, or adds no unreleased entries.
"""

import argparse
import re
import subprocess
import sys

CHANGELOG = "CHANGELOG.md"
VERSION_GO = "internal/clientset/version.go"
UNRELEASED = "# Unreleased"
VERSION_LINE = re.compile(r'^const TF_PROVIDER_VERSION = "([^"]*)"$', re.M)
SEMVER = re.compile(r"(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)")


def fail(message):
    sys.exit(f"::error::{message}")


def read(path):
    with open(path, encoding="utf-8") as f:
        return f.read()


def git_show(ref, path):
    return subprocess.run(["git", "show", f"{ref}:{path}"], check=True, capture_output=True, text=True).stdout


def version_of(source):
    m = VERSION_LINE.search(source)
    if not m or not SEMVER.fullmatch(m.group(1)):
        fail(f"{VERSION_GO} has no MAJOR.MINOR.PATCH TF_PROVIDER_VERSION")
    return m.group(1)


def next_versions(version):
    major, minor, patch = (int(p) for p in version.split("."))
    return {
        "patch": f"{major}.{minor}.{patch + 1}",
        "minor": f"{major}.{minor + 1}.0",
        "major": f"{major + 1}.0.0",
    }


def entries(text, heading):
    """Return the "- " entries between `heading` and the next top-level heading, or None."""
    lines = text.splitlines()
    if heading not in lines:
        return None
    found = []
    for line in lines[lines.index(heading) + 1:]:
        if line.startswith("# "):
            break
        if line.lstrip().startswith("- "):
            found.append(line.strip())
    return found


def bump(kind):
    source, changelog = read(VERSION_GO), read(CHANGELOG)
    current = version_of(source)
    new = next_versions(current)[kind]
    if not entries(changelog, UNRELEASED):
        fail(f"'{UNRELEASED}' in {CHANGELOG} has no entries. Add them first, then bump.")
    if entries(changelog, f"# Release {new}") is not None:
        fail(f"{CHANGELOG} already has '# Release {new}'")
    changelog = changelog.replace(UNRELEASED + "\n", f"{UNRELEASED}\n\n# Release {new}\n", 1)
    source = VERSION_LINE.sub(f'const TF_PROVIDER_VERSION = "{new}"', source, count=1)
    with open(CHANGELOG, "w", encoding="utf-8") as f:
        f.write(changelog)
    with open(VERSION_GO, "w", encoding="utf-8") as f:
        f.write(source)
    print(f"{current} -> {new}. Merging this PR releases v{new}.")


def check(base_ref):
    changelog = read(CHANGELOG)
    headings = re.findall(r"^# Release (\S+)$", changelog, re.M)
    duplicates = sorted({h for h in headings if headings.count(h) > 1})
    if duplicates:
        fail(f"{CHANGELOG} has '# Release {duplicates[0]}' more than once. Another release merged first: "
             f"move this change's entries back under '{UNRELEASED}', reset {VERSION_GO}, and run `make bump` again.")
    fork = subprocess.run(["git", "merge-base", base_ref, "HEAD"], check=True, capture_output=True, text=True).stdout.strip()
    base, tip = version_of(git_show(fork, VERSION_GO)), version_of(git_show(base_ref, VERSION_GO))
    current = version_of(read(VERSION_GO))
    if current != base and tip != base:
        fail(f"master released v{tip} after this branch started from v{base}. Update the branch, "
             f"move this change's entries back under '{UNRELEASED}', reset {VERSION_GO} to {tip}, and run `make bump` again.")
    # Released sections are immutable: an entry added to one would never be released.
    # Rewording an entry keeps the count and is fine.
    base_changelog = git_show(base_ref, CHANGELOG)
    for version in set(re.findall(r"^# Release (\S+)$", base_changelog, re.M)):
        before, after = entries(base_changelog, f"# Release {version}"), entries(changelog, f"# Release {version}") or []
        if len(after) > len(before):
            fail(f"This change adds entries to '# Release {version}', which is already released. Move them "
                 f"under '{UNRELEASED}' and run `make bump`.")
    unreleased = entries(changelog, UNRELEASED) or []
    if current == base:
        # Entries that were already unreleased at the fork point are not this change's to release.
        added = set(unreleased) - set(entries(git_show(fork, CHANGELOG), UNRELEASED) or [])
        if added:
            fail(f"This change adds entries under '{UNRELEASED}' but keeps version {current}. "
                 "Every merge with changelog entries is a release: run `make bump` "
                 "(or `make bump BUMP=minor` / `BUMP=major`) and commit the result.")
        print(f"Version stays {current}; no release.")
        return
    allowed = next_versions(base)
    if current not in allowed.values():
        fail(f"Version {base} -> {current} is not the next patch, minor, or major version "
             f"({', '.join(allowed.values())}). Reset {VERSION_GO} and run `make bump` once.")
    if not entries(changelog, f"# Release {current}"):
        fail(f"{CHANGELOG} has no entries under '# Release {current}'. Run `make bump` instead of editing by hand.")
    if unreleased:
        fail(f"'{UNRELEASED}' still has entries; they would not be part of v{current}. Run `make bump` after adding them.")
    kind = next(k for k, v in allowed.items() if v == current)
    print(f"{base} -> {current} ({kind}). Merging releases v{current}.")


def main():
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sub = parser.add_subparsers(dest="command", required=True)
    sub.add_parser("bump").add_argument("kind", nargs="?", default="patch", choices=["patch", "minor", "major"])
    sub.add_parser("check").add_argument("base_ref")
    args = parser.parse_args()
    if args.command == "bump":
        bump(args.kind)
    else:
        check(args.base_ref)


if __name__ == "__main__":
    main()
