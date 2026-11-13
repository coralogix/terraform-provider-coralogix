#!/usr/bin/env python3
"""Print only public test names and fixed diagnostics from private test output."""

import os
from pathlib import Path
import re
import subprocess
import sys


TEST_RESULT = re.compile(r"^\s*--- (PASS|FAIL|SKIP): (Test[A-Za-z0-9_]+)(?:/|\s|$)")
TEST_FUNCTION = re.compile(r"^func (Test[A-Za-z0-9_]+)\(", re.MULTILINE)
HTTP_STATUS = re.compile(r"\bHTTP\s+([45][0-9]{2})\b")


def public_test_names():
    root = Path(__file__).resolve().parents[2]
    result = subprocess.run(
        ["git", "-C", str(root), "ls-files", "--", "*_test.go"],
        capture_output=True, text=True, check=False,
    )
    if result.returncode:
        raise RuntimeError("cannot read public test names")
    names = set()
    for path in result.stdout.splitlines():
        names.update(TEST_FUNCTION.findall((root / path).read_text(encoding="utf-8")))
    return names


def write_test_names(passed, failed):
    for variable, rows in (
        ("FAILED_TESTS_FILE", [f"{name}\n" for name in sorted(failed)]),
        ("RAN_TESTS_FILE", [f"{name}\tPASS\n" for name in sorted(passed)]
         + [f"{name}\tFAIL\n" for name in sorted(failed)]),
    ):
        if path_value := os.environ.get(variable):
            path = Path(path_value)
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("".join(rows), encoding="utf-8")


def print_names(passed, failed):
    print(f"Passed tests ({len(passed)}):")
    for name in sorted(passed):
        print(name)
    print(f"Failed tests ({len(failed)}):")
    for name in sorted(failed):
        print(name)


def print_result(known_tests):
    passed, failed = set(), set()
    unreported = False
    for path in Path("ran-tests").glob("*.txt"):
        for line in path.read_text(encoding="utf-8").splitlines():
            name, separator, status = line.partition("\t")
            if separator and name in known_tests:
                if status == "PASS":
                    passed.add(name)
                elif status == "FAIL":
                    failed.add(name)
    for path in Path("failed-tests").glob("*.txt"):
        for line in path.read_text(encoding="utf-8").splitlines():
            if line in known_tests:
                failed.add(line)
            elif line == "Test command failed before reporting a test name":
                unreported = True
    passed.difference_update(failed)
    print_names(passed, failed)
    if unreported:
        print("A test command failed before reporting a test name")


def diagnostic(line):
    # Never copy API or test output into the runner log.
    if re.search(r"(?i)\bdataset\b.{0,100}\bnot found\b", line):
        return "Dataset unavailable"
    if re.search(r"(?i)\b(?:unused|free) evaluation target", line):
        return "Evaluation target unavailable"
    if re.search(r"(?i)\b(?:compile|undefined:|build failed)\b", line):
        return "Build failed"
    if re.search(r"(?i)\b(?:timed? out|deadline exceeded)\b", line):
        return "Request timed out"
    if match := HTTP_STATUS.search(line):
        if match.group(1) in ("401", "403"):
            return "Authorization failed"
        return "API request failed"
    return None


def filter_output(known_tests):
    collect_names = os.environ.get("PRIVATE_TEST_NAMES") == "true"
    passed, failed, diagnostics = set(), set(), set()
    for line in sys.stdin:
        if collect_names and (match := TEST_RESULT.search(line)):
            outcome, name = match.groups()
            if name in known_tests:
                if outcome == "FAIL":
                    failed.add(name)
                    passed.discard(name)
                elif outcome == "PASS" and name not in failed:
                    passed.add(name)
        if reason := diagnostic(line):
            diagnostics.add(reason)
    if collect_names:
        write_test_names(passed, failed)
    for reason in sorted(diagnostics):
        print(f"Failure category: {reason}")
    if collect_names:
        print_names(passed, failed)


def main():
    known_tests = public_test_names()
    if sys.argv[1:] == ["result"]:
        print_result(known_tests)
    else:
        filter_output(known_tests)


if __name__ == "__main__":
    try:
        main()
    except Exception:
        print("Acceptance output processing failed", file=sys.stderr)
        raise SystemExit(1)
