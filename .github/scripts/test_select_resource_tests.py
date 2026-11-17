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

    def test_generated_tests_follow_the_selected_resource_package(self):
        def listed(package):
            if package.endswith("/generated/newthing"):
                return ["TestAccNewThingGenerated", "TestGeneratedUnit"]
            if package.endswith("/generated/other"):
                return ["TestAccOtherGenerated"]
            return []

        with mock.patch.object(selector, "resources", return_value={}), \
             mock.patch.object(selector, "root_names", return_value=[]), \
             mock.patch.object(selector, "listed_names", side_effect=listed):
            selected = selector.select("NewThing", "newthing", "coralogix_new_thing")
        self.assertEqual(["TestAccNewThingGenerated"],
                         selected["shards"][0]["expected"])
        self.assertEqual(selector.MODULE + "/internal/provider/generated/newthing",
                         selected["shards"][0]["package"])
        self.assertNotIn("TestAccOtherGenerated", selected["shards"][0]["expected"])

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
        patterns = re.findall(r"^\s+[A-Z0-9_]+:.*'(\^Test[^']*)'", workflow, re.M)
        selected = set()
        for path in provider.glob("*_test.go"):
            source = path.read_text()
            tests = list(re.finditer(r"^func (Test\w+)\(", source, re.M))
            for index, match in enumerate(tests):
                name = match.group(1)
                if name.startswith("TestAcc"):
                    continue
                end = tests[index + 1].start() if index + 1 < len(tests) else len(source)
                body = source[match.start():end]
                if "resource.Test(t," in body and any(re.search(pattern, name) for pattern in patterns):
                    selected.add(name)
        mapped = {name for files in selector.EXTRA_ROOT_TESTS.values()
                  for expected in files.values() for name in expected
                  if not name.startswith("TestAcc")}
        self.assertEqual(selected, mapped)

    def test_ip_access_live_test_requires_its_extra_inputs(self):
        required = selector.required_inputs("ipaccess", ["TestIpAccessGeneratedLiveAcceptance"])
        self.assertIn("CORALOGIX_ENV", required)
        self.assertNotIn("CORALOGIX_DOMAIN", required)
        self.assertIn("TF_ACC_PROVIDER_NAMESPACE", required)
        self.assertIn("IP_ACCESS_GENERATED_LIVE_ALLOW_MUTATION", required)

    def test_upgrade_and_migration_gates_follow_selected_tests(self):
        required = selector.required_inputs("apikey", ["TestAccCoralogixResourceApiKeyGeneratedUpgrade"])
        self.assertIn("CORALOGIX_GENERATED_UPGRADE_ACC", required)
        self.assertIn("TF_ACC_PROVIDER_NAMESPACE", required)
        required = selector.required_inputs("dashboard", ["TestAccCoralogixResourceDashboardMigrationFromV360"])
        self.assertIn("CORALOGIX_DASHBOARD_MIGRATION_ACC", required)
        self.assertIn("TF_ACC_PROVIDER_NAMESPACE", required)
        required = selector.required_inputs("dashboard", ["TestAccCoralogixResourceDashboardGenerated"])
        self.assertNotIn("CORALOGIX_DASHBOARD_MIGRATION_ACC", required)

    def test_singleton_tests_share_one_serial_shard(self):
        root = ["TestAccCoralogixResourceQuotaAllocationRuleSet"]
        generated = ["TestAccCoralogixResourceQuotaAllocationRuleSetGenerated"]
        def listed(package):
            return generated if package.endswith("/generated/quotaallocationruleset") else root
        with mock.patch.object(selector, "resources", return_value={
            "QuotaAllocationRuleSet": [("quotaallocationruleset", "coralogix_quota_allocation_rule_set")]}), \
             mock.patch.object(selector, "root_names", return_value=root), \
             mock.patch.object(selector, "listed_names", side_effect=listed):
            selected = selector.select("QuotaAllocationRuleSet")
        self.assertEqual(1, len(selected["shards"]))
        self.assertEqual("serial", selected["shards"][0]["name"])
        self.assertEqual([root, generated],
                         [step["expected"] for step in selected["shards"][0]["steps"]])

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
