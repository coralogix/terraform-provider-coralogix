---
page_title: "Migration Guide for Alerts (from pre 2.0)" 
---

# Guide to Using the Terraform Migration Script

!> Before following this guide, please reach out to your customer support representative for more information.

This guide provides step-by-step instructions on how to use the Terraform migration scripts effectively. 

---

## Prerequisites
0. **Get the scripts**:
   - Download from [https://github.com/coralogix/coralogix-management-sdk/tree/master/tools/terraform-importer]()
1. **Terraform Installed**:
   - Ensure you have Terraform installed. You can download it [here](https://www.terraform.io/downloads).
2. **Go Installed**:
   - Install Go from [golang.org](https://golang.org/dl/).
3. **Python Installed**:
   - The script uses Python 3 for JSON processing, so make sure you have Python 3 installed.
4. **`hcl2json` Installed**:
   - Install the `hcl2json` utility. You can find it [here](https://github.com/tmccombs/hcl2json).

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
`CORALOGIX_API_KEY` and `CORALOGIX_ENV` or `CORALOGIX_DOMAIN`.

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