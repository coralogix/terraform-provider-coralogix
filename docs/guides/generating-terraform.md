---
page_title: "Generating Coralogix Resources As Terraform" 
---

# Generating Terraform Code From Coralogix

Moving to an infrastructure as code based approach can be difficult. What if the environment has thousands of alerts? Lovingly handcrafted dashboards? Meticulously organized TCO rules? Similarly, if the existing Terraform schemas changed over the versions, it may proof to be a lot of work to upgrade. 

This is why this guide and the accompanying migration script exists. Here we will go over the process of how to generate Terraform configs using the migration script along with the limitations it comes with. 

## Installing the script

The script is a wrapper around [Terraform's config generator](https://developer.hashicorp.com/terraform/language/import/generating-configuration) with several helpful additions that require the following dependencies to be installed: 

---

1. Terraform
2. Go (1.23 or higher) 
3. Python 3
4. hcl2json from [this repository](https://github.com/tmccombs/hcl2json)

Once installed, get the migration script from [github.com/coralogix/coralogix-management-sdk](https://github.com/coralogix/coralogix-management-sdk/tree/master/tools/terraform-importer). 

## Peeking Under The Hood

The migration script provides two modes of operation, advancing an existing state or generating (migrating) existing Coralogix resources as Terraform. In this guide we are describing option nr. 2, importing resources from Coralogix:

```bash
$ ./generate_and_migrate.sh
2025-02-28 09:55:48 [INFO] Select the migration type:
2025-02-28 09:55:48 [INFO] 1) Migrate based on a folder containing terraform.tfstate
2025-02-28 09:55:48 [INFO] 2) Migrate based on a specific resource name
Enter your choice (1 or 2):
```

Selecting menu option 2 will ask for the resource and using the provided API key (see below) and cluster, create a Terraform configuration using the selected provider version, remove any empty values from the config, and finally ask to apply the config (which should result in no changes). Let's dive deep into these parts one-by-one. 

# Generating Terraform

Let's get into it: Creating Terraform config for Coralogix resources. Skip to the summary for a video version - otherwise follow along in the following steps.

## Limitations

Before starting, let's set some expectations. The script is not using any magic, so in all but the most basic cases there is some manual work required. Here is a list of limitations that are known and cannot be addressed (unfortunately):

   - **References, variables:** Generated files will always use concrete values
   - **Blocks, modules, loops:** Generated files will produce a naive view of the resources without the assisstance of control structures
   - **Unexpected backend changes:** Mismatches between what the backend returns and the provider expect can happen and are considered bugs. Please create an issue [here](https://github.com/coralogix/terraform-provider-coralogix/issues) if you encounter one.
   - **Not "listable":** Resources that don't support a list operation can't be part of the script. In this case use the regular import declarations in Terraform. 

With these in mind, it's still possible to significantly speed up transitioning to an infrastructure as code setup using the migration script. Let's start by preparing the environment.

## Preparing the Environment

Whatever operating system (WSL on Windows is recommended) you use, make sure to run the script from within the `terraform-importer` directory from the SDK repository. Then, within that environment, set the following environment variables, just like with the Terraform provider itself:

   - `CORALOGIX_API_KEY`
   - `CORALOGIX_ENV` or `CORALOGIX_DOMAIN`

!> Note that the provided API key has to have the required permissions for accessing the resources. 

Once these are set, proceed to running the script. 

## Running the Script

The script itself needs the terminal with the environment variables set up. Make sure they exist using the `env` command:

```bash
$ env | grep "CORALOGIX_"
CORALOGIX_API_KEY=cxup_ap1K3yap1K3yap1K3yap1K3yap1b5
CORALOGIX_ENV=EU2
```

Then, run the script and select "migration type 2":

```bash
$ cd tools/terraform-importer/
$ ./generate_and_migrate.sh
2025-03-03 09:32:19 [INFO] Select the migration type:
2025-03-03 09:32:19 [INFO] 1) Migrate based on a folder containing terraform.tfstate
2025-03-03 09:32:19 [INFO] 2) Migrate based on a specific resource name
Enter your choice (1 or 2): 2
```

Then, select the resource type you want to migrate. In this example we are picking `recording_rules_groups_set`, or nr 10:

```bash
2025-03-03 09:32:21 [INFO] Available resource types:
1) alert			 8) events2metrics
2) archive_logs			 9) group
3) archive_metrics		10) recording_rules_groups_set
4) archive_retentions		11) scope
5) custom_role			12) tco_policies_logs
6) dashboard			13) tco_policies_traces
7) dashboards_folder		14) webhook
#? 10
```

Once selected the script pulls all readable resources of that type. However, to output the right Terraform config, one other input is required: the version of the provider. In this example, we are using `~>2.0.0` which means that the output conforms to that provider version. Note that after generating the output, the script requires to confirm the apply step by typing `yes`, however applying is actually optional, the generated file will exist regardless:

```bash
Enter the Terraform provider version to migrate to (e.g., ~>2.0.0): ~>2.0.0
2025-03-03 09:32:35 [INFO] Creating migration folder: ./recording_rules_groups_set_migration
2025-03-03 09:32:35 [INFO] Running generate_imports.go with -type...

`imports.tf` file has been generated at: %!s(*string=0x1400044a330)
2025-03-03 09:32:36 [INFO] Successfully generated imports.tf at ./recording_rules_groups_set_migration.
2025-03-03 09:32:36 [INFO] Generating provider configuration in ./recording_rules_groups_set_migration/provider.tf...
2025-03-03 09:32:36 [INFO] Provider configuration generated in ./recording_rules_groups_set_migration/provider.tf.
2025-03-03 09:32:36 [INFO] Initializing Terraform in ./recording_rules_groups_set_migration...
-e 2025-03-03 09:32:37
-e 2025-03-03 09:32:37  Initializing the backend...
-e 2025-03-03 09:32:37
-e 2025-03-03 09:32:37  Initializing provider plugins...
-e 2025-03-03 09:32:37  - Finding coralogix/coralogix versions matching "~> 2.0.0"...
-e 2025-03-03 09:32:39  - Installing coralogix/coralogix v2.0.9...
-e 2025-03-03 09:32:41  - Installed coralogix/coralogix v2.0.9 (self-signed, key ID 020F3E2CF567DACB)
-e 2025-03-03 09:32:41
-e 2025-03-03 09:32:41  Partner and community providers are signed by their developers.
-e 2025-03-03 09:32:41  If you'd like to know more about provider signing, you can read about it here:
-e 2025-03-03 09:32:41  https://www.terraform.io/docs/cli/plugins/signing.html
-e 2025-03-03 09:32:41
-e 2025-03-03 09:32:41  Terraform has created a lock file .terraform.lock.hcl to record the provider
-e 2025-03-03 09:32:41  selections it made above. Include this file in your version control repository
-e 2025-03-03 09:32:41  so that Terraform can guarantee to make the same selections by default when
-e 2025-03-03 09:32:41  you run "terraform init" in the future.
-e 2025-03-03 09:32:41
-e 2025-03-03 09:32:41  Terraform has been successfully initialized!
-e 2025-03-03 09:32:41
-e 2025-03-03 09:32:41  You may now begin working with Terraform. Try running "terraform plan" to see
-e 2025-03-03 09:32:41  any changes that are required for your infrastructure. All Terraform commands
-e 2025-03-03 09:32:41  should now work.
-e 2025-03-03 09:32:41
-e 2025-03-03 09:32:41  If you ever set or change modules or backend configuration for Terraform,
-e 2025-03-03 09:32:41  rerun this command to reinitialize your working directory. If you forget, other
-e 2025-03-03 09:32:41  commands will detect it and remind you to do so if necessary.
2025-03-03 09:32:41 [INFO] Terraform initialization completed.
2025-03-03 09:32:41 [INFO] Running terraform plan in ./recording_rules_groups_set_migration...
-e 2025-03-03 09:32:42  coralogix_recording_rules_groups_set.examplee: Preparing import... [id=01JNDFBB4YFYYB1B9AF5W273WP]
-e 2025-03-03 09:32:42  coralogix_recording_rules_groups_set.examplee: Refreshing state... [id=01JNDFBB4YFYYB1B9AF5W273WP]
-e 2025-03-03 09:32:42
-e 2025-03-03 09:32:42  Terraform will perform the following actions:
-e 2025-03-03 09:32:42
-e 2025-03-03 09:32:42    # coralogix_recording_rules_groups_set.examplee will be imported
-e 2025-03-03 09:32:42    # (config will be generated)
-e 2025-03-03 09:32:42      resource "coralogix_recording_rules_groups_set" "examplee" {
-e 2025-03-03 09:32:42          groups = [
-e 2025-03-03 09:32:42              {
-e 2025-03-03 09:32:42                  interval = 180
-e 2025-03-03 09:32:42                  limit    = 0
-e 2025-03-03 09:32:42                  name     = "Foo"
-e 2025-03-03 09:32:42                  rules    = [
-e 2025-03-03 09:32:42                      {
-e 2025-03-03 09:32:42                          expr   = "sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)"
-e 2025-03-03 09:32:42                          record = "ts3db_live_ingester_write_latency:3m"
-e 2025-03-03 09:32:42                      },
-e 2025-03-03 09:32:42                      {
-e 2025-03-03 09:32:42                          expr   = "sum(rate(http_requests_total[5m])) by (job)"
-e 2025-03-03 09:32:42                          record = "job:http_requests_total:sum"
-e 2025-03-03 09:32:42                      },
-e 2025-03-03 09:32:42                  ]
-e 2025-03-03 09:32:42              },
-e 2025-03-03 09:32:42              {
-e 2025-03-03 09:32:42                  interval = 60
-e 2025-03-03 09:32:42                  limit    = 0
-e 2025-03-03 09:32:42                  name     = "Bar"
-e 2025-03-03 09:32:42                  rules    = [
-e 2025-03-03 09:32:42                      {
-e 2025-03-03 09:32:42                          expr   = "sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)"
-e 2025-03-03 09:32:42                          record = "ts3db_live_ingester_write_latency:3m"
-e 2025-03-03 09:32:42                      },
-e 2025-03-03 09:32:42                      {
-e 2025-03-03 09:32:42                          expr   = "sum(rate(http_requests_total[5m])) by (job)"
-e 2025-03-03 09:32:42                          record = "job:http_requests_total:sum"
-e 2025-03-03 09:32:42                      },
-e 2025-03-03 09:32:42                  ]
-e 2025-03-03 09:32:42              },
-e 2025-03-03 09:32:42          ]
-e 2025-03-03 09:32:42          id     = "01JNDFBB4YFYYB1B9AF5W273WP"
-e 2025-03-03 09:32:42          name   = "Examplee"
-e 2025-03-03 09:32:42      }
-e 2025-03-03 09:32:42
-e 2025-03-03 09:32:42  Plan: 1 to import, 0 to add, 0 to change, 0 to destroy.
-e 2025-03-03 09:32:42  ╷
-e 2025-03-03 09:32:42  │ Warning: Config generation is experimental
-e 2025-03-03 09:32:42  │
-e 2025-03-03 09:32:42  │ Generating configuration during import is currently experimental, and the
-e 2025-03-03 09:32:42  │ generated configuration format may change in future versions.
-e 2025-03-03 09:32:42  ╵
-e 2025-03-03 09:32:42
-e 2025-03-03 09:32:42  ─────────────────────────────────────────────────────────────────────────────
-e 2025-03-03 09:32:42
-e 2025-03-03 09:32:42  Terraform has generated configuration and written it to generated.tf. Please
-e 2025-03-03 09:32:42  review the configuration and edit it as necessary before adding it to version
-e 2025-03-03 09:32:42  control.
-e 2025-03-03 09:32:42
-e 2025-03-03 09:32:42  Note: You didn't use the -out option to save this plan, so Terraform can't
-e 2025-03-03 09:32:42  guarantee to take exactly these actions if you run "terraform apply" now.
2025-03-03 09:32:42 [INFO] Converting generated.tf to JSON...
2025-03-03 09:32:42 [INFO] Removing null values from JSON...
2025-03-03 09:32:42 [INFO] Cleaned JSON saved to cleaned_config.json.
2025-03-03 09:32:42 [INFO] Navigating back to script's directory: /Users/cm/workspace/coralogix/coralogix-management-sdk/tools/terraform-importer
2025-03-03 09:32:42 [INFO] Converting cleaned JSON back to HCL using Go program...
Terraform configuration written to ./recording_rules_groups_set_migration/cleaned_config.tf
2025-03-03 09:32:43 [INFO] Cleaned Terraform file saved as generated.tf
2025-03-03 09:32:43 [INFO] Running terraform apply...
-e 2025-03-03 09:32:43  coralogix_recording_rules_groups_set.examplee: Preparing import... [id=01JNDFBB4YFYYB1B9AF5W273WP]
-e 2025-03-03 09:32:43  coralogix_recording_rules_groups_set.examplee: Refreshing state... [id=01JNDFBB4YFYYB1B9AF5W273WP]
-e 2025-03-03 09:32:43
-e 2025-03-03 09:32:43  Terraform will perform the following actions:
-e 2025-03-03 09:32:43
-e 2025-03-03 09:32:43    # coralogix_recording_rules_groups_set.examplee will be imported
-e 2025-03-03 09:32:43      resource "coralogix_recording_rules_groups_set" "examplee" {
-e 2025-03-03 09:32:43          groups = [
-e 2025-03-03 09:32:43              {
-e 2025-03-03 09:32:43                  interval = 180
-e 2025-03-03 09:32:43                  limit    = 0
-e 2025-03-03 09:32:43                  name     = "Foo"
-e 2025-03-03 09:32:43                  rules    = [
-e 2025-03-03 09:32:43                      {
-e 2025-03-03 09:32:43                          expr   = "sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)"
-e 2025-03-03 09:32:43                          record = "ts3db_live_ingester_write_latency:3m"
-e 2025-03-03 09:32:43                      },
-e 2025-03-03 09:32:43                      {
-e 2025-03-03 09:32:43                          expr   = "sum(rate(http_requests_total[5m])) by (job)"
-e 2025-03-03 09:32:43                          record = "job:http_requests_total:sum"
-e 2025-03-03 09:32:43                      },
-e 2025-03-03 09:32:43                  ]
-e 2025-03-03 09:32:43              },
-e 2025-03-03 09:32:43              {
-e 2025-03-03 09:32:43                  interval = 60
-e 2025-03-03 09:32:43                  limit    = 0
-e 2025-03-03 09:32:43                  name     = "Bar"
-e 2025-03-03 09:32:43                  rules    = [
-e 2025-03-03 09:32:43                      {
-e 2025-03-03 09:32:43                          expr   = "sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)"
-e 2025-03-03 09:32:43                          record = "ts3db_live_ingester_write_latency:3m"
-e 2025-03-03 09:32:43                      },
-e 2025-03-03 09:32:43                      {
-e 2025-03-03 09:32:43                          expr   = "sum(rate(http_requests_total[5m])) by (job)"
-e 2025-03-03 09:32:43                          record = "job:http_requests_total:sum"
-e 2025-03-03 09:32:43                      },
-e 2025-03-03 09:32:43                  ]
-e 2025-03-03 09:32:43              },
-e 2025-03-03 09:32:43          ]
-e 2025-03-03 09:32:43          id     = "01JNDFBB4YFYYB1B9AF5W273WP"
-e 2025-03-03 09:32:43          name   = "Examplee"
-e 2025-03-03 09:32:43      }
-e 2025-03-03 09:32:43
-e 2025-03-03 09:32:43  Plan: 1 to import, 0 to add, 0 to change, 0 to destroy.
-e 2025-03-03 09:32:43
-e 2025-03-03 09:32:43  Do you want to perform these actions?
-e 2025-03-03 09:32:43    Terraform will perform the actions described above.
-e 2025-03-03 09:32:43    Only 'yes' will be accepted to approve.
-e 2025-03-03 09:32:43
yes
-e 2025-03-03 09:35:22    Enter a value:
-e 2025-03-03 09:35:22  coralogix_recording_rules_groups_set.examplee: Importing... [id=01JNDFBB4YFYYB1B9AF5W273WP]
-e 2025-03-03 09:35:22  coralogix_recording_rules_groups_set.examplee: Import complete [id=01JNDFBB4YFYYB1B9AF5W273WP]
-e 2025-03-03 09:35:22
-e 2025-03-03 09:35:22  Apply complete! Resources: 1 imported, 0 added, 0 changed, 0 destroyed.
2025-03-03 09:35:22 [INFO] Terraform apply completed.
2025-03-03 09:35:22 [INFO] Cleaning up temporary files...
2025-03-03 09:35:22 [INFO] Cleanup completed.
2025-03-03 09:35:22 [INFO] Script completed successfully.
```

After acknowledging the terraform apply (by typing `yes`), the resulting files will be in a subdirectory named after the resource:

```bash
$ tree recording_rules_groups_set_migration/
recording_rules_groups_set_migration/
├── generated.tf
├── provider.tf
└── terraform.tfstate

1 directory, 3 files
```

The generated files looks like this (after formatting):

```bash
$ cat recording_rules_groups_set_migration/generated.tf
resource "coralogix_recording_rules_groups_set" "examplee" {
  groups = [{
    interval = 180
    limit    = 0
    name     = "Foo"
    rules = [{
      expr   = "sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)"
      record = "ts3db_live_ingester_write_latency:3m"
      }, {
      expr   = "sum(rate(http_requests_total[5m])) by (job)"
      record = "job:http_requests_total:sum"
    }]
    }, {
    interval = 60
    limit    = 0
    name     = "Bar"
    rules = [{
      expr   = "sum(rate(ts3db_live_ingester_write_latency_seconds_count{CX_LEVEL=\"staging\",pod=~\"ts3db-live-ingester.*\"}[2m])) by (pod)"
      record = "ts3db_live_ingester_write_latency:3m"
      }, {
      expr   = "sum(rate(http_requests_total[5m])) by (job)"
      record = "job:http_requests_total:sum"
    }]
  }]
  name = "Examplee"
}
```

As a next step, the generated resource can be used within a larger Terraform context or be the starting point to build one. Regardless, it's highly recommended to see if there are some improvements and adjustments for the specifics of the platform (variables, ...) that are not covered by the tool. 

# Summary

This tool was built to ease the migration to infrastructure as code style management of Coralogix, as well as an easy path to migrating between versions. Using the built-in feature of the Terraform CLI it allows for a quick and easy migrations between versions and onboarding infrastructure as code. Additionally, this can be an easy way to use the web UI to set preferred options and let the script handle the translation into Terraform. 

To recap, here is a video showing the importing of multiple alerts:

<script src="https://asciinema.org/a/DhwLrzpB3XKuVyS7e906F69wG.js" id="asciicast-DhwLrzpB3XKuVyS7e906F69wG" async="true" data-cols="120" data-rows="30"></script>

The resulting alerts are:

```hcl
resource "coralogix_alert" "updated-app-latency" {
  notification_group = {

  }
  enabled  = true
  group_by = ["destination_workload", "le"]
  labels = {
    severity = "critical"
  }
  name         = "updated-app-latency"
  phantom_mode = false
  priority     = "P1"
  type_definition = {
    metric_threshold = {
      custom_evaluation_delay = 0
      metric_filter = {
        promql = "histogram_quantile(0.99, sum(irate(istio_request_duration_seconds_bucket{reporter=\"source\",destination_service=~\"ingress-annotation-test-svc.example-app.svc.cluster.local\"}[1m])) by (le, destination_workload)) > 0.2"
      }
      missing_values = {
        min_non_null_values_pct = 0
      }
      rules = [{
        condition = {
          for_over_pct   = 100
          of_the_last    = "5_MINUTES"
          threshold      = 0
          condition_type = "MORE_THAN"
        }
        override = {
          priority = "P5"
        }
      }]
      undetected_values_management = {
        auto_retire_timeframe     = "NEVER"
        trigger_undetected_values = false
      }
    }
  }
  description = "This is an updated alert"
  incidents_settings = {
    notify_on = "Triggered Only"
    retriggering_period = {
      minutes = 10
    }
  }
}

resource "coralogix_alert" "updated-app-latency_2" {
  enabled = true
  incidents_settings = {
    notify_on = "Triggered Only"
    retriggering_period = {
      minutes = 10
    }
  }
  type_definition = {
    metric_threshold = {
      custom_evaluation_delay = 0
      metric_filter = {
        promql = "histogram_quantile(0.99, sum(irate(istio_request_duration_seconds_bucket{reporter=\"source\",destination_service=~\"ingress-annotation-test-svc.example-app.svc.cluster.local\"}[1m])) by (le, destination_workload)) > 0.2"
      }
      missing_values = {
        min_non_null_values_pct = 0
      }
      rules = [{
        condition = {
          threshold      = 0
          condition_type = "MORE_THAN"
          for_over_pct   = 100
          of_the_last    = "15_MINUTES"
        }
        override = {
          priority = "P5"
        }
      }]
      undetected_values_management = {
        auto_retire_timeframe     = "NEVER"
        trigger_undetected_values = false
      }
    }
  }
  description = "This is an updated alert"
  group_by    = ["destination_workload", "le"]
  labels = {
    severity = "info"
  }
  name = "updated-app-latency"
  notification_group = {

  }
  phantom_mode = false
  priority     = "P4"
}

resource "coralogix_alert" "updated-app-latency_3" {
  description = "This is an updated alert"
  enabled     = true
  labels = {
    severity = "critical"
  }
  name = "updated-app-latency"
  notification_group = {

  }
  group_by = ["destination_workload", "le"]
  incidents_settings = {
    notify_on = "Triggered Only"
    retriggering_period = {
      minutes = 10
    }
  }
  phantom_mode = false
  priority     = "P1"
  type_definition = {
    metric_threshold = {
      custom_evaluation_delay = 0
      metric_filter = {
        promql = "histogram_quantile(0.99, sum(irate(istio_request_duration_seconds_bucket{reporter=\"source\",destination_service=~\"ingress-annotation-test-svc.example-app.svc.cluster.local\"}[1m])) by (le, destination_workload)) > 0.2"
      }
      missing_values = {
        min_non_null_values_pct = 0
      }
      rules = [{
        condition = {
          threshold      = 0
          condition_type = "MORE_THAN"
          for_over_pct   = 100
          of_the_last    = "5_MINUTES"
        }
        override = {
          priority = "P5"
        }
      }]
      undetected_values_management = {
        trigger_undetected_values = false
        auto_retire_timeframe     = "NEVER"
      }
    }
  }
}

resource "coralogix_alert" "updated-app-latency_4" {
  group_by = ["destination_workload", "le"]
  labels = {
    severity = "info"
  }
  phantom_mode = false
  description  = "This is an updated alert"
  enabled      = true
  incidents_settings = {
    notify_on = "Triggered Only"
    retriggering_period = {
      minutes = 10
    }
  }
  name = "updated-app-latency"
  notification_group = {

  }
  priority = "P4"
  type_definition = {
    metric_threshold = {
      custom_evaluation_delay = 0
      metric_filter = {
        promql = "histogram_quantile(0.99, sum(irate(istio_request_duration_seconds_bucket{reporter=\"source\",destination_service=~\"ingress-annotation-test-svc.example-app.svc.cluster.local\"}[1m])) by (le, destination_workload)) > 0.2"
      }
      missing_values = {
        min_non_null_values_pct = 0
      }
      rules = [{
        condition = {
          threshold      = 0
          condition_type = "MORE_THAN"
          for_over_pct   = 100
          of_the_last    = "15_MINUTES"
        }
        override = {
          priority = "P5"
        }
      }]
      undetected_values_management = {
        auto_retire_timeframe     = "NEVER"
        trigger_undetected_values = false
      }
    }
  }
}
```

Thank you for reading! Let us know any issues you encounter at https://github.com/coralogix/terraform-provider-coralogix/ 