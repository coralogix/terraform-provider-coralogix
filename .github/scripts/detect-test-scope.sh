#!/usr/bin/env bash
# Chooses which acceptance test jobs a pull request runs.
#
# Reads the changed file paths on stdin. Prints GitHub step outputs:
#   code=true|false        any file that is not docs or an agent file changed
#   dashboards=true|false  run the dashboard jobs
#   rest=true|false        run every other job
#   deps=true|false        run only the other jobs that use dashboard code (see below)
#
# The two buckets:
#   dashboards  the dashboard resource: its code, tests, test data, and examples.
#   rest        every other resource, listed by name below.
# A file in neither bucket, or in the shared list, runs both buckets. So does a pull request
# that changes files of both buckets. Every push to master and the nightly run run everything.
#
# A pull request that changes only dashboard files also runs the jobs that use dashboard code:
# the alert shards (alert_schema imports dashboard_schema), the catch-all job, and the
# generated-upgrade shard. Without them, a change to dashboard_schema would skip alert tests.

set -euo pipefail

ignored='^(\.claude/|\.cursor/)|\.md$'

dashboards='^(internal/provider/(dashboards/|generated/dashboardsfolder/|testdata/dashboard|dashboard|(resource|data_source)_coralogix_dashboard)|examples/(resources|data-sources)/coralogix_dashboard)'

# Files that the dashboard tests use but that are not dashboard files. Keep this list in step
# with the tests: when a dashboard test starts to use a helper of another file, add that file.
shared='^internal/provider/(provider_test|mux_test|provider_domain_test|data_source_baseline_test|data_source_baseline_fake_test|global_router_baseline_test|global_router_baseline_harness_test|global_router_baseline_fake_test|global_router_fidelity_test)\.go$'

# One directory per other domain. A new directory is not listed, so it runs every job.
domains='aaa|actions|ai|alerts|apm|data_exploration|dataengine|dataplans|enrichment_rules|events2metrics|fleet|integrations|logs|metrics|notifications|parsing_rules|recording_rules|slo_mgmt|views|webhook'
rest="^(internal/provider/(($domains)/|generated/[a-z0-9]+/|testdata/|[a-z0-9_]+_test\.go\$)|examples/(resources|data-sources)/)"

code=false dash=false other=false
while IFS= read -r file; do
  [[ -z $file || $file =~ $ignored ]] && continue
  code=true
  if [[ $file =~ $dashboards ]]; then
    dash=true
  elif [[ $file =~ $shared ]]; then
    dash=true other=true
  elif [[ $file =~ $rest ]]; then
    other=true
  else
    dash=true other=true
  fi
done

deps=false
if [[ $dash == true && $other == false ]]; then
  deps=true
fi

echo "code=$code"
echo "dashboards=$dash"
echo "rest=$other"
echo "deps=$deps"
