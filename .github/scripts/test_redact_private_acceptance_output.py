#!/usr/bin/env python3

from pathlib import Path
import os
import subprocess
import sys
import tempfile
import unittest


WRAPPER = Path(__file__).with_name("run-tests-with-summary.sh")
REDACTOR = Path(__file__).with_name("redact-private-acceptance-output.py")
COVERAGE = Path(__file__).with_name("check-test-coverage.sh")


def run_private(output, status, directory, package="internal/provider"):
    package_name = f"github.com/coralogix/terraform-provider-coralogix/{package}"
    package_line = f"{'ok' if status == 0 else 'FAIL'}\t{package_name}\t0.1s"
    environment = {
        "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
        "PRIVATE_ACC_OUTPUT": "true",
        "CI_PREP_LOOKUP": "false",
        "CORALOGIX_API_KEY": "SENSITIVE_SENTINEL",
        "FAILED_TESTS_FILE": str(directory / "failed.txt"),
        "RAN_TESTS_FILE": str(directory / "ran.txt"),
    }
    result = subprocess.run(
        ["bash", str(WRAPPER), sys.executable, "-c", f"import sys; print({output!r}); print({package_line!r}); sys.exit({status})"],
        env=environment,
        capture_output=True,
        text=True,
        check=False,
    )
    return result, (directory / "failed.txt").read_text(), (directory / "ran.txt").read_text()


class RedactPrivateAcceptanceOutputTest(unittest.TestCase):
    def test_failure_reports_fixed_reason_and_public_test_name(self):
        with tempfile.TemporaryDirectory() as directory:
            result, failed, ran = run_private(
                "--- FAIL: TestAccCoralogixResourceTCOPoliciesLogs_targets (0.2s)\n"
                "Error: 412 Precondition Failed: Dataset 'cicd' not found; "
                "SENSITIVE_SENTINEL; "
                "8b2a8d42-1234-4abc-8def-123456789abc; "
                "actor@example.invalid; "
                "https://private.invalid/path; "
                "teamId: 123456",
                1,
                Path(directory),
            )
            self.assertEqual(result.returncode, 1)
            self.assertIn("412 Precondition Failed", result.stdout)
            self.assertIn("not found", result.stdout)
            self.assertIn("Failure category: Dataset unavailable", result.stdout)
            self.assertIn("Failed tests (1):\nTestAccCoralogixResourceTCOPoliciesLogs_targets", result.stdout)
            for private in ("cicd", "SENSITIVE_SENTINEL", "8b2a8d42", "actor@example.invalid", "private.invalid", "123456"):
                self.assertNotIn(private, result.stdout + result.stderr)
            self.assertEqual(
                failed,
                "TestAccCoralogixResourceTCOPoliciesLogs_targets\n"
                "Failure category: API returned HTTP 412\n"
                "Failure category: Dataset unavailable\n",
            )
            self.assertEqual(ran, "github.com/coralogix/terraform-provider-coralogix/internal/provider\tTestAccCoralogixResourceTCOPoliciesLogs_targets\tFAIL\n")

    def test_passed_names_and_failed_names_are_repeated_at_bottom(self):
        with tempfile.TemporaryDirectory() as directory:
            result, failed, ran = run_private(
                "--- PASS: TestAccCoralogixResourceSLOV2APMError (0.1s)\n"
                "--- FAIL: TestAccCoralogixResourceTCOPoliciesLogs_targets (0.2s)",
                1,
                Path(directory),
            )
            self.assertEqual(result.returncode, 1)
            self.assertIn("Passed tests (1):\nTestAccCoralogixResourceSLOV2APMError", result.stdout)
            self.assertTrue(result.stdout.rstrip().endswith("Failed tests (1):\nTestAccCoralogixResourceTCOPoliciesLogs_targets\nAcceptance test failed"))
            self.assertEqual(failed, "TestAccCoralogixResourceTCOPoliciesLogs_targets\n")
            self.assertIn("internal/provider\tTestAccCoralogixResourceSLOV2APMError\tPASS", ran)

    def test_unknown_names_do_not_enter_artifacts(self):
        with tempfile.TemporaryDirectory() as directory:
            result, failed, ran = run_private("--- FAIL: TestSENSITIVE_SENTINEL (0.1s)", 1, Path(directory))
            self.assertEqual(result.returncode, 1)
            self.assertNotIn("SENSITIVE_SENTINEL", result.stdout + result.stderr + failed + ran)
            self.assertEqual(failed, "Test command failed before reporting a test name\n")

    def test_arbitrary_private_values_never_reach_logs(self):
        with tempfile.TemporaryDirectory() as directory:
            aws_id = "AKIA" + "A" * 16
            pagerduty_key = "B" * 20
            generated_key = "C" * 30
            result, _, _ = run_private(
                "--- FAIL: TestAccCoralogixResourceTCOPoliciesLogs_targets (0.1s)\n"
                f"AWS {aws_id} PagerDuty {pagerduty_key} Coralogix {generated_key}\n"
                "application=private-example-ai-02\n"
                "request failed for team 321; wss://internal.example.invalid/a\n"
                "team 432 has no archive\n"
                "HTTP 403\n"
                "Dataset 'private-dataset' not found; integration abc-123\n"
                "::warning::Private value from test output",
                1,
                Path(directory),
            )
            self.assertEqual(result.returncode, 1)
            self.assertIn("HTTP 403", result.stdout)
            self.assertIn("request failed", result.stdout)
            self.assertIn("not found", result.stdout)
            self.assertIn("Private value from test output", result.stdout)
            self.assertIn("Failure category: Dataset unavailable", result.stdout)
            self.assertIn("Failure category: Authorization failed", result.stdout)
            for value in (aws_id, pagerduty_key, generated_key, "private-example-ai",
                          "321", "432", "internal.example", "private-dataset", "abc-123",
                          "::"):
                self.assertNotIn(value, result.stdout + result.stderr)

    def test_result_repeats_only_public_test_names(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "ran-tests").mkdir()
            (root / "failed-tests").mkdir()
            (root / "ran-tests" / "shard.txt").write_text(
                "github.com/coralogix/terraform-provider-coralogix/internal/provider\tTestAccCoralogixResourceSLOV2APMError\tPASS\n"
                "github.com/coralogix/terraform-provider-coralogix/internal/provider\tTestSENSITIVE_SENTINEL\tPASS\n",
                encoding="utf-8",
            )
            (root / "failed-tests" / "shard.txt").write_text(
                "TestAccCoralogixResourceTCOPoliciesLogs_targets\n"
                "Failure category: Resource update failed\n"
                "Failure category: token SENSITIVE_SENTINEL\n"
                "TestSENSITIVE_SENTINEL\n",
                encoding="utf-8",
            )
            result = subprocess.run(
                [sys.executable, str(REDACTOR), "result"],
                cwd=root,
                env={"PATH": os.environ.get("PATH", "/usr/bin:/bin")},
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(result.returncode, 0)
            self.assertIn("Passed tests (1):\nTestAccCoralogixResourceSLOV2APMError", result.stdout)
            self.assertIn("Failed tests (1):\nTestAccCoralogixResourceTCOPoliciesLogs_targets", result.stdout)
            self.assertIn("Failure category: Resource update failed", result.stdout)
            self.assertNotIn("SENSITIVE_SENTINEL", result.stdout + result.stderr)

    def test_coverage_keeps_only_public_dashboard_subtest_names(self):
        with tempfile.TemporaryDirectory() as directory:
            result, failed, ran = run_private(
                "=== RUN   TestAccCoralogixResourceDashboardGenerated\n"
                "=== RUN   TestAccCoralogixResourceDashboardGenerated/values-and-choices-1\n"
                "--- PASS: TestAccCoralogixResourceDashboardGenerated/values-and-choices-1 (0.1s)\n"
                "=== RUN   TestAccCoralogixResourceDashboardGenerated/SENSITIVE_SENTINEL\n"
                "--- PASS: TestAccCoralogixResourceDashboardGenerated/SENSITIVE_SENTINEL (0.1s)\n"
                "--- PASS: TestAccCoralogixResourceDashboardGenerated (0.2s)",
                0,
                Path(directory),
                "internal/provider/generated/dashboard",
            )
            self.assertEqual(result.returncode, 0)
            self.assertEqual(failed, "")
            self.assertIn("\tTestAccCoralogixResourceDashboardGenerated/values-and-choices-1\tPASS\n", ran)
            self.assertIn("\tTestAccCoralogixResourceDashboardGenerated\tPASS\n", ran)
            self.assertNotIn("SENSITIVE_SENTINEL", ran + result.stdout + result.stderr)

    def test_safe_coverage_rows_accept_split_dashboard_test(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            package = "github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/dashboard"
            parent = "TestAccCoralogixResourceDashboardGenerated"
            (root / "listed.txt").write_text(f"{package}\t{parent}\tLISTED\n")
            files = []
            for step in (1, 2):
                shard = root / f"shard-{step}"
                shard.mkdir()
                subtest = f"{parent}/values-and-choices-{step}"
                result, _, ran = run_private(
                    f"=== RUN   {parent}\n=== RUN   {subtest}\n"
                    f"--- PASS: {subtest} (0.1s)\n--- PASS: {parent} (0.1s)",
                    0,
                    shard,
                    "internal/provider/generated/dashboard",
                )
                self.assertEqual(result.returncode, 0)
                file = root / f"ran-{step}.txt"
                file.write_text(ran)
                files.append(str(file))
            checked = subprocess.run(
                ["bash", str(COVERAGE), str(root / "listed.txt"), *files],
                capture_output=True,
                text=True,
                check=False,
            )
            self.assertEqual(checked.returncode, 0, checked.stdout + checked.stderr)

    def test_scheduler_failure_prints_safe_subtest_operation_and_status(self):
        with tempfile.TemporaryDirectory() as directory:
            result, failed, ran = run_private(
                "=== RUN   TestAccCoralogixResourceAlertsSchedulerGenerated\n"
                "=== RUN   TestAccCoralogixResourceAlertsSchedulerGenerated/values-and-choices\n"
                "Error: Unable to update alerts_scheduler: 409 Conflict; "
                "integrationId=SENSITIVE_SENTINEL; actor@example.invalid\n"
                "--- FAIL: TestAccCoralogixResourceAlertsSchedulerGenerated/values-and-choices (0.1s)\n"
                "--- FAIL: TestAccCoralogixResourceAlertsSchedulerGenerated (0.1s)",
                1,
                Path(directory),
                "internal/provider/generated/alertsscheduler",
            )
            self.assertEqual(result.returncode, 1)
            self.assertIn("Unable to update alerts_scheduler", result.stdout)
            self.assertIn("409 Conflict", result.stdout)
            self.assertIn("Failed subtest: TestAccCoralogixResourceAlertsSchedulerGenerated/values-and-choices", result.stdout)
            self.assertIn("Failure category: API returned HTTP 409", result.stdout)
            self.assertIn("Failure category: Resource update failed", result.stdout)
            self.assertIn("\tTestAccCoralogixResourceAlertsSchedulerGenerated/values-and-choices\tFAIL\n", ran)
            self.assertIn("Failure category: Resource update failed\n", failed)
            self.assertNotIn("SENSITIVE_SENTINEL", result.stdout + result.stderr + failed + ran)
            self.assertNotIn("actor@example.invalid", result.stdout + result.stderr + failed + ran)


if __name__ == "__main__":
    unittest.main()
