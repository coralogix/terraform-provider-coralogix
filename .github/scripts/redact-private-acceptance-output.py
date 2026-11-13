#!/usr/bin/env python3
"""Print acceptance output after removing private values.

The job log keeps the test text. Keys, emails, IDs, URLs, and application
names are replaced. Public test names are still recorded for the summary.
"""

import os
from pathlib import Path
import re
import subprocess
import sys


TEST_RESULT = re.compile(r"^\s*--- (PASS|FAIL|SKIP): (Test[A-Za-z0-9_]+(?:/[^\s]+)*)\s")
TEST_START = re.compile(r"^=== RUN\s+(Test[A-Za-z0-9_]+(?:/[^\s]+)*)\s*$")
TEST_FUNCTION = re.compile(r"^func (Test[A-Za-z0-9_]+)\(", re.MULTILINE)
SUBTEST_LITERAL = re.compile(r'"([A-Za-z0-9_-]+)"')
PACKAGE_RESULT = re.compile(r"^(?:ok|FAIL)\s+([^\s]+)(?:\s|$)")
SAFE_DASHBOARD_SUBTEST = re.compile(
    r"^(?:line-and-table|gauge-and-bars|horizontal-and-hexagon|"
    r"resolution-and-time-frames|bar-chart-axes|horizontal-bar-options|"
    r"colors-by-branches|full-lifecycle|minimal-lifecycle|values-and-choices-[0-9]+)$"
)
SAFE_SCHEDULER_SUBTEST = re.compile(r"^(?:full-lifecycle|minimal-lifecycle|values-and-choices)$")
HTTP_STATUS = re.compile(r"\bHTTP\s+([45][0-9]{2})\b")
STATUS_TEXT = re.compile(
    r"\b([45][0-9]{2})\s+(?:Bad Request|Unauthorized|Forbidden|Not Found|Conflict|"
    r"Precondition Failed|Unprocessable Entity|Internal Server Error|Service Unavailable|Gateway Timeout)\b"
)
RESOURCE_OPERATION = re.compile(r"\bUnable to (create|read|update|delete)\b")
SAFE_REASON = re.compile(
    r"^(?:Dataset unavailable|Evaluation target unavailable|Build failed|"
    r"Request timed out|Authorization failed|API returned HTTP [45][0-9]{2}|"
    r"Resource (?:create|read|update|delete) failed|Provider returned inconsistent state|"
    r"Plan was not empty after apply|Terraform (?:apply|plan) failed|Import verification failed)$"
)
REDACTED = "[redacted]"
EMAIL = re.compile(r"\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b")
URL = re.compile(r"\b(?:https?|wss?)://\S+")
AWS_ARN = re.compile(r"\barn:[A-Za-z0-9:/_-]+")
UUID = re.compile(r"\b[0-9a-fA-F]{8}(?:-[0-9a-fA-F]{4}){3}-[0-9a-fA-F]{12}\b")
AWS_KEY = re.compile(r"\b(?:AKIA|ASIA|AROA|AIDA)[A-Z0-9]{16}\b")
HEX_ID = re.compile(r"\b[0-9a-fA-F]{32,}\b")
REPEATED_TOKEN = re.compile(r"\b([A-Za-z0-9])\1{15,}\b")
LONG_TOKEN = re.compile(r"\b(?=[A-Za-z0-9]*\d)[A-Za-z0-9+/=]{20,}\b")
IPV4 = re.compile(r"\b(?:\d{1,3}\.){3}\d{1,3}\b")
FIELD_VALUE = re.compile(
    r"(?i)([\"']?(?:application|app|subsystem|apmService|dataset|teamId|team_id|"
    r"integrationId|connectorKey|api[_-]?key|token|secret|password|authorization|"
    r"bucket|email)[\"']?\s*[:=]\s*)(?:[\"'][^\"']*[\"']|[^\s,;}{]+)"
)
DATASET_QUOTE = re.compile(r"(?i)(\bdataset\s+['\"])[^'\"]*(['\"])")
TEAM_NUMBER = re.compile(r"(?i)(\bteam(?:Id)?\s*[:=]?\s*)\d+\b")
INTEGRATION_TOKEN = re.compile(r"(?i)(\bintegration(?:Id)?\s+)([A-Za-z0-9_-]*\d[A-Za-z0-9_-]*)")
SECRET_ENV_MARKERS = ("KEY", "TOKEN", "SECRET", "PASSWORD", "EMAIL", "PREFIX", "TEAM_ID", "INTEGRATION", "BUCKET", "CHANNEL", "CONNECTOR")


def sensitive_values():
    values = []
    for name, value in os.environ.items():
        if not value or len(value) < 3:
            continue
        upper = name.upper()
        if not any(marker in upper for marker in SECRET_ENV_MARKERS):
            continue
        if len(value) < 8 and "TEAM" not in upper and "ID" not in upper and "PREFIX" not in upper:
            continue
        values.append(value)
    values.sort(key=len, reverse=True)
    return values


def redact(line, secrets):
    for secret in secrets:
        if len(secret) >= 12:
            line = line.replace(secret, REDACTED)
            continue
        line = re.sub(
            r"(?<![A-Za-z0-9])" + re.escape(secret) + r"(?:-\d+)?(?![A-Za-z0-9])",
            REDACTED,
            line,
        )
    line = URL.sub(REDACTED, line)
    line = AWS_ARN.sub(REDACTED, line)
    line = EMAIL.sub(REDACTED, line)
    line = AWS_KEY.sub(REDACTED, line)
    line = UUID.sub(REDACTED, line)
    line = HEX_ID.sub(REDACTED, line)
    line = IPV4.sub(REDACTED, line)
    line = REPEATED_TOKEN.sub(REDACTED, line)
    line = LONG_TOKEN.sub(REDACTED, line)
    line = FIELD_VALUE.sub(r"\1" + REDACTED, line)
    line = DATASET_QUOTE.sub(r"\1" + REDACTED + r"\2", line)
    line = TEAM_NUMBER.sub(r"\1" + REDACTED, line)
    line = INTEGRATION_TOKEN.sub(r"\1" + REDACTED, line)
    # A test line must not become a GitHub workflow command.
    line = line.replace("::", ":")
    return line


def public_test_names():
    root = Path(__file__).resolve().parents[2]
    module = next(line.removeprefix("module ") for line in (root / "go.mod").read_text().splitlines() if line.startswith("module "))
    result = subprocess.run(
        ["git", "-C", str(root), "ls-files", "--", "*_test.go"],
        capture_output=True, text=True, check=False,
    )
    if result.returncode:
        raise RuntimeError("cannot read public test names")
    names = set()
    test_packages = set()
    subtests = set()
    for path in result.stdout.splitlines():
        directory = Path(path).parent.as_posix()
        package = module if directory == "." else f"{module}/{directory}"
        source = (root / path).read_text(encoding="utf-8")
        subtests.update(SUBTEST_LITERAL.findall(source))
        for name in TEST_FUNCTION.findall(source):
            names.add(name)
            test_packages.add((package, name))
    return names, test_packages, subtests


def public_result_name(name, subtests):
    parent, separator, subtest = name.partition("/")
    if not separator:
        return parent
    if (parent.startswith("TestAccCoralogixResourceDashboard")
            and SAFE_DASHBOARD_SUBTEST.fullmatch(subtest) and subtest in subtests):
        return name
    if (parent == "TestAccCoralogixResourceAlertsSchedulerGenerated"
            and SAFE_SCHEDULER_SUBTEST.fullmatch(subtest) and subtest in subtests):
        return name
    return None


def write_test_names(passed, failed, ran, diagnostics, package_failed):
    for variable, rows in (
        ("FAILED_TESTS_FILE", [f"{name}\n" for name in sorted(failed)]
         + ([f"Failure category: {reason}\n" for reason in sorted(diagnostics)] if failed or package_failed else [])),
        ("RAN_TESTS_FILE", [f"{package}\t{name}\t{status}\n" for package, name, status in sorted(ran)]),
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


def print_result(known_tests, subtests):
    passed, failed, failed_subtests, diagnostics = set(), set(), set(), set()
    unreported = False
    for path in Path("ran-tests").glob("*.txt"):
        for line in path.read_text(encoding="utf-8").splitlines():
            fields = line.split("\t")
            if len(fields) != 3:
                continue
            _, full_name, status = fields
            name = full_name.split("/", 1)[0]
            if name in known_tests:
                if status == "PASS":
                    passed.add(name)
                elif status == "FAIL":
                    failed.add(name)
                    if full_name != name and public_result_name(full_name, subtests) == full_name:
                        failed_subtests.add(full_name)
    for path in Path("failed-tests").glob("*.txt"):
        for line in path.read_text(encoding="utf-8").splitlines():
            if line in known_tests:
                failed.add(line)
            elif line.startswith("Failure category: ") and SAFE_REASON.fullmatch(line.removeprefix("Failure category: ")):
                diagnostics.add(line.removeprefix("Failure category: "))
            elif line == "Test command failed before reporting a test name":
                unreported = True
    passed.difference_update(failed)
    print_names(passed, failed)
    for name in sorted(failed_subtests):
        print(f"Failed subtest: {name}")
    for reason in sorted(diagnostics):
        print(f"Failure category: {reason}")
    if unreported:
        print("A test command failed before reporting a test name")


def diagnostic(line):
    # Never copy API or test output into the runner log.
    reasons = set()
    if re.search(r"(?i)\bdataset\b.{0,100}\bnot found\b", line):
        reasons.add("Dataset unavailable")
    if re.search(r"(?i)\b(?:unused|free) evaluation target", line):
        reasons.add("Evaluation target unavailable")
    if re.search(r"(?i)\b(?:compile|undefined:|build failed)\b", line):
        reasons.add("Build failed")
    if re.search(r"(?i)\b(?:timed? out|deadline exceeded)\b", line):
        reasons.add("Request timed out")
    if match := HTTP_STATUS.search(line) or STATUS_TEXT.search(line):
        reasons.add(f"API returned HTTP {match.group(1)}")
        if match.group(1) in ("401", "403"):
            reasons.add("Authorization failed")
    if match := RESOURCE_OPERATION.search(line):
        reasons.add(f"Resource {match.group(1)} failed")
    if "Provider produced inconsistent result after apply" in line:
        reasons.add("Provider returned inconsistent state")
    if "After applying this test step, the plan was not empty" in line:
        reasons.add("Plan was not empty after apply")
    if "Error running apply" in line:
        reasons.add("Terraform apply failed")
    if "Error running plan" in line:
        reasons.add("Terraform plan failed")
    if "ImportStateVerify" in line:
        reasons.add("Import verification failed")
    return reasons


def filter_output(known_tests, test_packages, subtests):
    collect_names = os.environ.get("PRIVATE_TEST_NAMES") == "true"
    secrets = sensitive_values()
    passed, failed, diagnostics = set(), set(), set()
    pending = {}
    ran = set()
    failed_subtests = set()
    package_failed = False
    for line in sys.stdin:
        raw = line.rstrip("\n")
        print(redact(raw, secrets), flush=True)
        line = raw
        if collect_names and (match := TEST_START.search(line)):
            pending[match.group(1)] = "NONE"
        if collect_names and (match := TEST_RESULT.search(line)):
            outcome, full_name = match.groups()
            pending[full_name] = outcome
            name = full_name.split("/", 1)[0]
            if name in known_tests and "/" not in full_name:
                if outcome == "FAIL":
                    failed.add(name)
                    passed.discard(name)
                elif outcome == "PASS" and name not in failed:
                    passed.add(name)
            elif name in known_tests and outcome == "FAIL" and public_result_name(full_name, subtests) == full_name:
                failed_subtests.add(full_name)
        if collect_names and (match := PACKAGE_RESULT.search(line)):
            package_failed |= line.startswith("FAIL")
            package = match.group(1)
            for full_name, outcome in pending.items():
                parent = full_name.split("/", 1)[0]
                safe_name = public_result_name(full_name, subtests)
                if safe_name and (package, parent) in test_packages:
                    ran.add((package, safe_name, outcome))
            pending.clear()
        diagnostics.update(diagnostic(line))
    if collect_names:
        write_test_names(passed, failed, ran, diagnostics, package_failed)
    if collect_names:
        print_names(passed, failed)
        for name in sorted(failed_subtests):
            print(f"Failed subtest: {name}")
    if not collect_names or failed or package_failed or "Build failed" in diagnostics:
        for reason in sorted(diagnostics):
            print(f"Failure category: {reason}")


def main():
    known_tests, test_packages, subtests = public_test_names()
    if sys.argv[1:] == ["result"]:
        print_result(known_tests, subtests)
    else:
        filter_output(known_tests, test_packages, subtests)


if __name__ == "__main__":
    try:
        main()
    except Exception:
        print("Acceptance output processing failed", file=sys.stderr)
        raise SystemExit(1)
