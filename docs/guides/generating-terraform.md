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

# Summary


1. **Get the scripts**:
   - 
2. **Terraform Installed**:
   - Ensure you have Terraform installed. You can download it [here](https://www.terraform.io/downloads).
3. **Go Installed**:
   - Install Go from [golang.org](https://golang.org/dl/).
4. **Python Installed**:
   - The script uses Python 3 for JSON processing, so make sure you have Python 3 installed.
5. **`hcl2json` Installed**:
   - Install the `hcl2json` utility. You can find it [here]().

---

## Usage

### 1. Script Purpose
The script allows you to:
- Migrate Terraform configurations based on:
   - A folder containing a `terraform.tfstate` file.
   - A specific resource type (e.g., `alert`, `dashboard`).
- Generate a migration folder with cleaned and updated configurations.
- Specify the provider version interactively during the process.

---

### 2. Running the Script
Before running the script, ensure you defined the required environment variables -  


Use the script as follows:
```bash
./generate_and_migrate.sh
```

---

### 3. Interactive Steps

#### Step 1: Select Migration Type
You will be prompted to choose the migration type:
- **Option 1**: Migrate based on a folder containing a `terraform.tfstate` file.
   - Provide the path to the folder.
   - The script ensures that the folder contains a valid `terraform.tfstate` file.
- **Option 2**: Migrate based on a specific resource type.
   - A list of resource types will be displayed. Choose from options like:
      - `alert`, `dashboard`, `archive_logs`, `events2metrics`, etc.
   - Select the desired resource type from the list.

#### Step 2: Specify Provider Version
After selecting the migration type, you will be prompted to specify the Terraform provider version:
- Example: `~>1.19.0`.
- The script will default to `>=2.0.0` if no input is provided.

---

### 4. What Happens Next

#### Step 3: Generate Migration Folder
- The script creates a new migration folder based on your input:
   - For a folder, it appends `_migration` to the folder name.
   - For a resource type, it creates a folder like `./<resource_type>_migration`.

#### Step 4: Run `generate_imports.go`
- The script runs a Go program (`generate_imports.go`) to generate an `imports.tf` file inside the migration folder.

#### Step 5: Generate `provider.tf`
- A `provider.tf` file is generated in the migration folder with the specified provider version.

#### Step 6: Run `terraform init`
- The script initializes Terraform inside the migration folder using `terraform init`.

#### Step 7: Run `terraform plan`
- The script runs `terraform plan` with the `-generate-config-out` flag to generate a new configuration file (`generated.tf`).

#### Step 8: Remove Null Values
- Python is used to clean the JSON file by removing null values, generating a cleaned JSON file (`cleaned_config.json`).

#### Step 9: Apply the Configuration
- The script applies the cleaned configuration using `terraform apply`.
**Note**: The script will prompt you to confirm the apply action and will override your existing resources with the new configuration. 
If you choose not to apply, the script will exit.

#### Step 10: Cleanup
- Temporary files are deleted.

---

### 5. Example Outputs

#### Migration Type Selection
```plaintext
[INFO] Select the migration type:
[INFO] 1) Migrate based on a folder containing terraform.tfstate
[INFO] 2) Migrate based on a specific resource name
Enter your choice (1 or 2): 2
```

#### Provider Version Prompt
```plaintext
Enter the Terraform provider version to migrate to (e.g., ~>1.19.0): >=2.0.0
```

#### Logs During Execution
```plaintext
2024-12-01 15:45:22 [INFO] Creating migration folder: ./alert_migration
2024-12-01 15:45:22 [INFO] Running generate_imports.go with -type...
2024-12-01 15:45:22 [INFO] Successfully generated imports.tf at ./alert_migration.
2024-12-01 15:45:22 [INFO] Generating provider configuration in ./alert_migration/provider.tf...
2024-12-01 15:45:22 [INFO] Provider configuration generated in ./alert_migration/provider.tf.
2024-12-01 15:45:22 [INFO] Initializing Terraform in ./alert_migration...
2024-12-01 15:45:22 [INFO] Running terraform plan in ./alert_migration...
...
2024-12-01 15:45:22 [INFO] Terraform apply completed.
2024-12-01 15:45:22 [INFO] Cleanup completed.
2024-12-01 15:45:22 [INFO] Script completed successfully.
```

---

### 6. Notes
- **Customization**:
   - Update the resource types in the script if new ones are added.
   - Adjust the default provider version if needed.
- **Error Handling**:
   - The script will exit if any step fails (`set -e`).
   - Logs are color-coded for better visibility (`INFO`, `ERROR`, `WARNING`, etc.).

---