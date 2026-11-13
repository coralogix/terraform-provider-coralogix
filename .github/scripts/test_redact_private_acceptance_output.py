#!/usr/bin/env python3

from pathlib import Path
import os
import subprocess
import sys
import tempfile
import unittest


WRAPPER = Path(__file__).with_name("run-tests-with-summary.sh")
REDACTOR = Path(__file__).with_name("redact-private-acceptance-output.py")


def run_private(output, status, directory):
    environment = {
        "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
        "PRIVATE_ACC_OUTPUT": "true",
        "CI_PREP_LOOKUP": "false",
        "CORALOGIX_API_KEY": "SENSITIVE_SENTINEL",
        "FAILED_TESTS_FILE": str(directory / "failed.txt"),
        "RAN_TESTS_FILE": str(directory / "ran.txt"),
    }
    result = subprocess.run(
        ["bash", str(WRAPPER), sys.executable, "-c", f"import sys; print({output!r}); sys.exit({status})"],
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
            self.assertIn("Failure category: Dataset unavailable", result.stdout)
            self.assertIn("Failed tests (1):\nTestAccCoralogixResourceTCOPoliciesLogs_targets", result.stdout)
            for private in ("cicd", "SENSITIVE_SENTINEL", "8b2a8d42", "actor@example.invalid", "private.invalid", "123456"):
                self.assertNotIn(private, result.stdout + result.stderr)
            self.assertEqual(failed, "TestAccCoralogixResourceTCOPoliciesLogs_targets\n")
            self.assertEqual(ran, "TestAccCoralogixResourceTCOPoliciesLogs_targets\tFAIL\n")

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
            self.assertIn("TestAccCoralogixResourceSLOV2APMError\tPASS", ran)

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
                f"AWS {aws_id} PagerDuty {pagerduty_key} Coralogix {generated_key}\n"
                "application=private-example-ai-02\n"
                "request failed for team 321; wss://internal.example.invalid/a\n"
                "team 432 has no archive\n"
                "HTTP 403\n"
                "Dataset 'private-dataset' not found; integration abc-123\n"
                "::warning::Private value from test output",
                0,
                Path(directory),
            )
            self.assertEqual(result.returncode, 0)
            self.assertIn("Failure category: Dataset unavailable", result.stdout)
            self.assertIn("Failure category: Authorization failed", result.stdout)
            for value in (aws_id, pagerduty_key, generated_key, "private-example-ai",
                          "321", "432", "403", "internal.example", "private-dataset", "abc-123",
                          "::warning::", "Private value"):
                self.assertNotIn(value, result.stdout + result.stderr)

    def test_result_repeats_only_public_test_names(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "ran-tests").mkdir()
            (root / "failed-tests").mkdir()
            (root / "ran-tests" / "shard.txt").write_text(
                "TestAccCoralogixResourceSLOV2APMError\tPASS\n"
                "TestSENSITIVE_SENTINEL\tPASS\n",
                encoding="utf-8",
            )
            (root / "failed-tests" / "shard.txt").write_text(
                "TestAccCoralogixResourceTCOPoliciesLogs_targets\n"
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
            self.assertNotIn("SENSITIVE_SENTINEL", result.stdout + result.stderr)


if __name__ == "__main__":
    unittest.main()
