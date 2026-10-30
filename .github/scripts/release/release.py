#!/usr/bin/env python3
"""Version and changelog tool for provider releases. Merging a PR that bumps
internal/clientset/version.go releases that version (release-provider.yml).

  release.py bump [patch|minor|major]   Open a "Release X.Y.Z" PR from origin/master: set the
                                        next version in version.go and move the "# Unreleased"
                                        entries under "# Release X.Y.Z". Runs in a temporary
                                        worktree, so the current checkout is left as it is.
                                        `make bump`, `make bump BUMP=minor`.
  release.py check BASE_REF             Fail unless the change since BASE_REF keeps the version
                                        or is a valid release.
"""

import argparse
import os
import re
import subprocess
import sys
import tempfile

CHANGELOG = "CHANGELOG.md"
VERSION_GO = "internal/clientset/version.go"
UNRELEASED = "# Unreleased"
VERSION_LINE = re.compile(r'^const TF_PROVIDER_VERSION = "([^"]*)"$', re.M)
SEMVER = re.compile(r"(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)")
# A top-level Markdown heading. Subheadings ("#### resource/...") start with "##".
HEADING = re.compile(r"^ {0,3}#[ \t]")
RELEASE_HEADING = re.compile(r"# Release (" + SEMVER.pattern + ")")


def fail(message):
    sys.exit(f"::error::{message}")


def read(path):
    with open(path, encoding="utf-8") as f:
        return f.read()


def write(path, text):
    with open(path, "w", encoding="utf-8") as f:
        f.write(text)


def run(*args, cwd=None):
    return subprocess.run(args, check=True, capture_output=True, text=True, cwd=cwd).stdout.strip()


def git_show(ref, path):
    return run("git", "show", f"{ref}:{path}")


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


def entries(text, heading, keyed=False):
    """Return the "- " entries between `heading` and the next top-level heading, or None.
    With keyed, each entry is (subheading, entry): the same text recurs under different resources."""
    lines = text.splitlines()
    if heading not in lines:
        return None
    found, subheading = [], None
    for line in lines[lines.index(heading) + 1:]:
        if HEADING.match(line):
            break
        if line.startswith("#"):
            subheading = line.strip()
        elif line.lstrip().startswith("- "):
            found.append((subheading, line.strip()) if keyed else line.strip())
    return found


def section(text, heading):
    """Return the lines between `heading` and the next top-level heading."""
    lines = text.splitlines()
    rest = lines[lines.index(heading) + 1:]
    end = next((i for i, line in enumerate(rest) if HEADING.match(line)), len(rest))
    return "\n".join(rest[:end]).strip()


def headings(text):
    return [line for line in text.splitlines() if HEADING.match(line)]


def content(text, heading):
    """The non-blank lines of a section, ignoring trailing whitespace."""
    return [line.rstrip() for line in section(text, heading).splitlines() if line.strip()]


def bump(kind):
    try:
        run("gh", "auth", "status")
    except (OSError, subprocess.CalledProcessError):
        fail("`gh` is not installed or not logged in. Run `gh auth login` first.")
    run("git", "fetch", "--quiet", "origin", "master")
    source, changelog = git_show("origin/master", VERSION_GO), git_show("origin/master", CHANGELOG)
    current = version_of(source)
    new = next_versions(current)[kind]
    if headings(changelog).count(UNRELEASED) != 1:
        fail(f"{CHANGELOG} on master needs exactly one '{UNRELEASED}' heading.")
    if not entries(changelog, UNRELEASED):
        fail(f"'{UNRELEASED}' in {CHANGELOG} on master has no entries, so there is nothing to release.")
    if entries(changelog, f"# Release {new}") is not None:
        fail(f"{CHANGELOG} on master already has '# Release {new}'")
    branch = f"release-{new}"
    if run("git", "ls-remote", "--heads", "origin", branch) or run("git", "branch", "--list", branch):
        fail(f"Branch {branch} already exists. Merge or close its PR, delete the branch, and run `make bump` again.")

    changelog = changelog.replace(UNRELEASED + "\n", f"{UNRELEASED}\n\n# Release {new}\n", 1)
    source = VERSION_LINE.sub(f'const TF_PROVIDER_VERSION = "{new}"', source, count=1)
    worktree = tempfile.mkdtemp(prefix=f"{branch}-")
    pushed = False
    run("git", "worktree", "add", "--quiet", "-b", branch, worktree, "origin/master")
    try:
        write(os.path.join(worktree, CHANGELOG), changelog + "\n")
        write(os.path.join(worktree, VERSION_GO), source + "\n")
        run("git", "commit", "--quiet", "-am", f"Release {new}", cwd=worktree)
        run("git", "push", "--quiet", "-u", "origin", branch, cwd=worktree)
        pushed = True
        body = f"Cuts the release **{new}** ({current} -> {new}, {kind}).\n\n{section(changelog, f'# Release {new}')}\n"
        url = run("gh", "pr", "create", "--base", "master", "--head", branch,
                  "--title", f"Release {new}", "--body", body, cwd=worktree)
    except subprocess.CalledProcessError as e:
        hint = (f" The branch is pushed; open the PR with "
                f"`gh pr create --base master --head {branch} --title \"Release {new}\"`.") if pushed else ""
        fail(f"`{' '.join(e.cmd[:3])}` failed: {e.stderr.strip()}.{hint}")
    finally:
        run("git", "worktree", "remove", "--force", worktree)
        run("git", "branch", "-D", branch)
    # No auto-merge: it would merge on any approval once the required checks pass, before
    # the rest of CI finishes. approve-release.yml merges the PR after every check is green.
    print(f"{current} -> {new}. Once every check on {url} is green, approve-release.yml approves and "
          f"merges it; that releases v{new}.")


def check(base_ref):
    changelog, base_changelog = read(CHANGELOG), git_show(base_ref, CHANGELOG)
    redo = "Close this release PR, delete its branch, and run `make bump` again."
    heads, base_heads = headings(changelog), headings(base_changelog)
    if heads.count(UNRELEASED) != 1:
        fail(f"{CHANGELOG} needs exactly one '{UNRELEASED}' heading; entries under another one are never released.")
    # A heading the tooling doesn't recognize (a typo, trailing whitespace) would hide its
    # entries from the release; headings already on master are grandfathered.
    for heading in heads:
        if heading != UNRELEASED and not RELEASE_HEADING.fullmatch(heading) and heading not in base_heads:
            fail(f"{CHANGELOG} has the heading '{heading}'. Top-level headings are '{UNRELEASED}' and "
                 "'# Release X.Y.Z' (added by `make bump`); put entries under '# Unreleased'.")
    duplicates = sorted({h for h in heads if heads.count(h) > 1})
    if duplicates:
        fail(f"{CHANGELOG} has '{duplicates[0]}' more than once: another release merged first. {redo}")
    fork = run("git", "merge-base", base_ref, "HEAD")
    base, tip = version_of(git_show(fork, VERSION_GO)), version_of(git_show(base_ref, VERSION_GO))
    current = version_of(read(VERSION_GO))
    if current != base and tip != base:
        fail(f"master released v{tip} after this branch started from v{base}. {redo}")
    # Released sections are immutable: an entry added to one would never be released, and
    # a removed or replaced one rewrites published history.
    released = set()
    for heading in base_heads:
        if heading == UNRELEASED:
            continue
        released.update(entries(base_changelog, heading, keyed=True))
        if heading not in heads:
            fail(f"This change removes '{heading}', which is already released. Restore it.")
        if content(changelog, heading) != content(base_changelog, heading):
            fail(f"This change edits '{heading}', which is already released. Restore it, and add new "
                 f"entries under '{UNRELEASED}'.")
    # Only the bump adds a release section; any other new one would never be tagged.
    for heading in heads:
        if heading not in base_heads and heading != UNRELEASED and (current == base or heading != f"# Release {current}"):
            fail(f"This change adds '{heading}' but {VERSION_GO} is {current}. Add the entries "
                 f"under '{UNRELEASED}'; `make bump` opens the release PR.")
    if current == base:
        print(f"Version stays {current}; no release.")
        return
    allowed = next_versions(base)
    if current not in allowed.values():
        fail(f"Version {base} -> {current} is not the next patch, minor, or major version "
             f"({', '.join(allowed.values())}). {redo}")
    new_entries = entries(changelog, f"# Release {current}", keyed=True)
    if not new_entries:
        fail(f"{CHANGELOG} has no entries under '# Release {current}'. {redo}")
    # After updating the branch, the merge base is master's tip, so a stale bump can look
    # valid; entries that master already released give it away.
    if set(new_entries) & released:
        fail(f"'# Release {current}' repeats entries that master already released. {redo}")
    if entries(changelog, UNRELEASED):
        # Changes merged after the release branch was cut ship in this version too, and
        # release PRs are approved without a human, so the changelog must say so.
        fail(f"'{UNRELEASED}' has entries that merged after this release branch was cut. Their code "
             f"ships in v{current}: move the entries under '# Release {current}'.")
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
