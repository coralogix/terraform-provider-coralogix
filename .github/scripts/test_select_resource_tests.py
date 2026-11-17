#!/usr/bin/env python3
import importlib.util
import pathlib
import re
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
        with mock.patch.object(selector, "resources", return_value=mapping), \
             mock.patch.object(selector, "root_names", return_value=[]), \
             mock.patch.object(selector, "listed_names", return_value=["TestAccThing"]):
            with self.assertRaisesRegex(ValueError, "ambiguous resource mapping"):
                selector.select("Thing")
            self.assertEqual("other", selector.select("Thing", "other", "coralogix_other")["out"])
            with self.assertRaisesRegex(ValueError, "mapping differs"):
                selector.select("Thing", "missing", "coralogix_other")

    def test_noncanonical_handwritten_test_files(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            provider = root / "internal/provider"
            provider.mkdir(parents=True)
            for files in selector.EXTRA_ROOT_TESTS.values():
                for file_name, names in files.items():
                    (provider / file_name).write_text(
                        "".join(f"func {name}(t *testing.T) {{}}\n" for name in names)
                    )
            with mock.patch.object(selector, "ROOT", root):
                for tf_name, files in selector.EXTRA_ROOT_TESTS.items():
                    names = [name for expected in files.values() for name in expected]
                    self.assertEqual(selector.root_names(tf_name, names), sorted(names))
                with self.assertRaisesRegex(ValueError, "mapped acceptance test is missing"):
                    selector.root_names("coralogix_ip_access", [])

    def test_workflow_acceptance_names_without_testacc_are_mapped(self):
        provider = selector.ROOT / "internal/provider"
        workflow = (selector.ROOT / ".github/workflows/acceptance-tests.yml").read_text()
        patterns = re.findall(r"^\s+[A-Z0-9_]*PATTERN:.*'([^']+)'$", workflow, re.M)
        names = {name for path in provider.glob("*_test.go")
                 for name in re.findall(r"^func (Test\w+)\(", path.read_text(), re.M)
                 if not name.startswith("TestAcc")}
        selected = {name for name in names
                    if any(re.search(pattern, name) for pattern in patterns)}
        mapped = {name for files in selector.EXTRA_ROOT_TESTS.values()
                  for expected in files.values() for name in expected
                  if not name.startswith("TestAcc")}
        self.assertEqual(selected, mapped)

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
