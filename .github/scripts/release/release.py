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
        if line.startswith("# "):
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
    end = next((i for i, line in enumerate(rest) if line.startswith("# ")), len(rest))
    return "\n".join(rest[:end]).strip()


def bump(kind):
    try:
        run("gh", "auth", "status")
    except (OSError, subprocess.CalledProcessError):
        fail("`gh` is not installed or not logged in. Run `gh auth login` first.")
    run("git", "fetch", "--quiet", "origin", "master")
    source, changelog = git_show("origin/master", VERSION_GO), git_show("origin/master", CHANGELOG)
    current = version_of(source)
    new = next_versions(current)[kind]
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
    print(f"{current} -> {new}. Merging {url} releases v{new}.")


def check(base_ref):
    changelog = read(CHANGELOG)
    headings = re.findall(r"^# Release (\S+)$", changelog, re.M)
    redo = "Close this release PR, delete its branch, and run `make bump` again."
    duplicates = sorted({h for h in headings if headings.count(h) > 1})
    if duplicates:
        fail(f"{CHANGELOG} has '# Release {duplicates[0]}' more than once: another release merged first. {redo}")
    fork = run("git", "merge-base", base_ref, "HEAD")
    base, tip = version_of(git_show(fork, VERSION_GO)), version_of(git_show(base_ref, VERSION_GO))
    current = version_of(read(VERSION_GO))
    if current != base and tip != base:
        fail(f"master released v{tip} after this branch started from v{base}. {redo}")
    # Released sections are immutable: an entry added to one would never be released.
    # Rewording an entry keeps the count and is fine.
    base_changelog = git_show(base_ref, CHANGELOG)
    base_headings = set(re.findall(r"^# Release (\S+)$", base_changelog, re.M))
    released = set()
    for version in base_headings:
        before, after = entries(base_changelog, f"# Release {version}"), entries(changelog, f"# Release {version}") or []
        released.update(entries(base_changelog, f"# Release {version}", keyed=True))
        if len(after) > len(before):
            fail(f"This change adds entries to '# Release {version}', which is already released. "
                 f"Add them under '{UNRELEASED}' instead.")
    # Only the bump adds a release section; any other new one would never be tagged.
    for version in sorted(set(headings) - base_headings - ({current} if current != base else set())):
        fail(f"This change adds '# Release {version}' but {VERSION_GO} is {current}. Add the entries "
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
        # Changes merged after the release branch was cut ship in this version too.
        print(f"::warning::'{UNRELEASED}' has entries that merged after this release branch was cut. "
              f"Their code ships in v{current}; move the entries under '# Release {current}' to list them there.")
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
