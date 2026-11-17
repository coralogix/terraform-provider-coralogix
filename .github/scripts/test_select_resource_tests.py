#!/usr/bin/env python3
import importlib.util
import pathlib
import tempfile
import unittest
from unittest import mock


path = pathlib.Path(__file__).with_name("select-resource-tests.py")
spec = importlib.util.spec_from_file_location("select_resource_tests", path)
selector = importlib.util.module_from_spec(spec)
spec.loader.exec_module(selector)


class SelectorTests(unittest.TestCase):
    def test_zero_match_fails(self):
        with mock.patch.object(selector, "resources", return_value={"Thing": [("thing", "coralogix_thing")]}), \
             mock.patch.object(selector, "listed_names", return_value=[]), \
             mock.patch.object(selector, "root_names", return_value=[]):
            with self.assertRaisesRegex(ValueError, "no listed acceptance tests"):
                selector.select("Thing")

    def test_new_resource_requires_explicit_mapping(self):
        with mock.patch.object(selector, "resources", return_value={}):
            with self.assertRaisesRegex(ValueError, "unknown generated resource"):
                selector.select("Thing")
            with self.assertRaisesRegex(ValueError, "invalid output directory"):
                selector.select("Thing", "../thing", "coralogix_thing")

    def test_ambiguous_existing_mapping_fails(self):
        mapping = {"Thing": [("thing", "coralogix_thing"),
                             ("other", "coralogix_other")]}
        with mock.patch.object(selector, "resources", return_value=mapping):
            with self.assertRaisesRegex(ValueError, "ambiguous resource mapping"):
                selector.select("Thing")

    def test_dashboard_includes_handwritten_and_every_generated_subtest(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            generated = root / "internal/provider/generated/dashboard"
            generated.mkdir(parents=True)
            (generated / "acceptance_test.go").write_text(
                't.Run("full-lifecycle", nil)\nt.Run("import", nil)\n'
            )
            parent = "TestAccCoralogixResourceDashboardGenerated"
            patterns = [
                ("core", "^TestAccCoralogixResourceDashboard$"),
                ("generated-full", f"^{parent}$/full-lifecycle$"),
                ("generated-import", f"^{parent}$/import$"),
            ]
            with mock.patch.object(selector, "ROOT", root), \
                 mock.patch.object(selector, "resources", return_value={"Dashboard": [("dashboard", "coralogix_dashboard")]}), \
                 mock.patch.object(selector, "root_names", return_value=["TestAccCoralogixResourceDashboard"]), \
                 mock.patch.object(selector, "listed_names", side_effect=lambda package: [parent] if "generated" in package else []), \
                 mock.patch.object(selector, "dashboard_patterns", return_value=patterns):
                selected = selector.select("Dashboard")
                self.assertEqual(len(selected["shards"]), 3)
                self.assertEqual(sum(len(shard.get("expected_subtests", []))
                                     for shard in selected["shards"]), 2)
                with mock.patch.object(selector, "dashboard_patterns", return_value=patterns[:-1]):
                    with self.assertRaisesRegex(ValueError, "generated subtest import has 0 shards"):
                        selector.select("Dashboard")


if __name__ == "__main__":
    unittest.main()
