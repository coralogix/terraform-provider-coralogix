#!/usr/bin/env python3
"""List one generated resource's acceptance tests and safe Go test shards.

The output is JSON. Run it from the provider root after generating the resource.
It uses `go test -list` so a zero-match selector fails before a live run.
"""
from __future__ import annotations

import argparse
import json
import pathlib
import re
import subprocess
import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]
MODULE = "github.com/coralogix/terraform-provider-coralogix"
EXTRA_ROOT_TEST_FILES = {
    "coralogix_ip_access": ("data_source_ip_access_test.go",),
    "coralogix_recording_rules_groups_set": ("data_source_coralogix_recording_rules_group_test.go",),
    "coralogix_webhook": ("webhook_generated_lifecycle_test.go",),
}


def resources() -> dict[str, list[tuple[str, str]]]:
    output = subprocess.check_output(
        [str(ROOT / "tools/iac-codegen/scripts/codegen-list.sh"), "--names"],
        cwd=ROOT, text=True,
    )
    mapping: dict[str, list[tuple[str, str]]] = {}
    for component, out, name in (line.split() for line in output.splitlines()):
        mapping.setdefault(component, []).append((out, name))
    return mapping


def listed_names(package: str) -> list[str]:
    result = subprocess.run(
        ["go", "test", "-list", "^TestAcc", package], cwd=ROOT,
        text=True, capture_output=True, check=False,
    )
    if result.returncode:
        raise ValueError(f"list tests in {package}: {result.stderr.strip()}")
    return [line for line in result.stdout.splitlines()
            if re.fullmatch(r"TestAcc\w+", line)]


def root_names(tf_name: str, available: list[str]) -> list[str]:
    stem = tf_name.removeprefix("coralogix_")
    provider = ROOT / "internal/provider"
    paths = []
    for kind in ("resource", "data_source"):
        prefix = f"{kind}_coralogix_{stem}"
        paths.extend(path for path in provider.glob(prefix + "*_test.go")
                     if path.name[len(prefix):].startswith(("_", ".")))
    for name in EXTRA_ROOT_TEST_FILES.get(tf_name, ()):
        path = provider / name
        if not path.is_file():
            raise ValueError(f"missing mapped acceptance test file: {path}")
        paths.append(path)
    declared = set()
    for path in paths:
        declared.update(re.findall(r"^func (TestAcc\w+)\(", path.read_text(), re.M))
    return sorted(declared.intersection(available))


def run_pattern(names: list[str]) -> str:
    return "^(?:" + "|".join(re.escape(name) for name in names) + ")$"


def dashboard_patterns() -> list[tuple[str, str]]:
    workflow = (ROOT / ".github/workflows/acceptance-tests.yml").read_text()
    entries = re.findall(
        r"^  DASHBOARD_SHARD_([A-Z0-9_]+): &dashboard_shard_\w+ '([^']+)'$",
        workflow, re.M,
    )
    return [(name.lower().replace("_", "-"), pattern) for name, pattern in entries
            if name != "GENERATED"]


def parent_matches(pattern: str, name: str) -> bool:
    # A Go -run slash separates a parent from a subtest. -list shows parents only.
    parent = pattern.split("/", 1)[0]
    return re.search(parent, name) is not None


def subtest_matches(pattern: str, parent: str, child: str) -> bool:
    parts = pattern.split("/", 1)
    return (len(parts) == 2 and re.search(parts[0], parent) is not None
            and re.search(parts[1], child) is not None)


def required_inputs(out: str) -> list[str]:
    acceptance = ROOT / "internal/provider/generated" / out / "acceptance.yaml"
    values = ["CORALOGIX_API_KEY", "CORALOGIX_DOMAIN"]
    if acceptance.exists():
        match = re.search(r"^env:\s*\[([^]]*)\]", acceptance.read_text(), re.M)
        if match:
            values.extend(item.strip() for item in match.group(1).split(",") if item.strip())
    if out == "team":
        values.append("CORALOGIX_ORG_API_KEY")
    return sorted(set(values))


def select(component: str, new_out: str | None = None,
           new_tf_name: str | None = None) -> dict:
    known = resources()
    if component in known:
        if len(known[component]) != 1:
            raise ValueError(f"ambiguous resource mapping in codegen-list.sh: {component}")
        out, tf_name = known[component][0]
        if (new_out and new_out != out) or (new_tf_name and new_tf_name != tf_name):
            raise ValueError(f"resource mapping differs from codegen-list.sh: {component}")
    else:
        if not new_out or not new_tf_name:
            raise ValueError(f"unknown generated resource: {component}")
        if not re.fullmatch(r"[a-z][a-z0-9_]*", new_out):
            raise ValueError(f"invalid output directory: {new_out}")
        if not re.fullmatch(r"coralogix_[a-z][a-z0-9_]*", new_tf_name):
            raise ValueError(f"invalid Terraform name: {new_tf_name}")
        out, tf_name = new_out, new_tf_name
    root_package = MODULE + "/internal/provider"
    generated_package = root_package + "/generated/" + out
    root = root_names(tf_name, listed_names("./internal/provider"))
    generated = listed_names("./internal/provider/generated/" + out)
    if out == "dashboard":
        root = [name for name in root if not re.search(r"Dashboards?Folder", name)]
    else:
        generated = [name for name in generated if name.startswith("TestAcc")]
    shards: list[dict] = []
    assigned_root: set[str] = set()
    assigned_generated: set[str] = set()
    if out == "dashboard":
        for shard, pattern in dashboard_patterns():
            # The generated parent has many subtests. Keep its existing shards.
            pkg = generated_package if shard.startswith("generated-") else root_package
            candidates = generated if pkg == generated_package else root
            matches = [name for name in candidates if parent_matches(pattern, name)]
            if not matches:
                continue
            shards.append({"name": shard, "package": pkg, "run": pattern,
                           "expected": matches})
            (assigned_generated if pkg == generated_package else assigned_root).update(matches)
        generated_file = ROOT / "internal/provider/generated/dashboard/acceptance_test.go"
        subtests = re.findall(r'\bt\.Run\("([^"]+)"', generated_file.read_text())
        parent = "TestAccCoralogixResourceDashboardGenerated"
        for child in subtests:
            covering = [shard for shard in shards if shard["package"] == generated_package
                        and subtest_matches(shard["run"], parent, child)]
            if len(covering) != 1:
                raise ValueError(f"Dashboard generated subtest {child} has {len(covering)} shards")
            covering[0].setdefault("expected_subtests", []).append(parent + "/" + child)
    for label, package, names in (
        ("handwritten", root_package, sorted(set(root) - assigned_root)),
        ("generated", generated_package, sorted(set(generated) - assigned_generated)),
    ):
        if names:
            shards.append({"name": label, "package": package,
                           "run": run_pattern(names), "expected": names})
    if not shards:
        raise ValueError(f"no listed acceptance tests for {component}")
    return {"resource": component, "out": out, "tf_name": tf_name,
            "required_inputs": required_inputs(out), "shards": shards}


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--resource", required=True)
    parser.add_argument("--out", help="output directory for a new resource")
    parser.add_argument("--tf-name", help="Terraform name for a new resource")
    args = parser.parse_args()
    try:
        print(json.dumps(select(args.resource, args.out, args.tf_name), indent=2))
    except (ValueError, subprocess.CalledProcessError) as error:
        print(f"select-resource-tests: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
