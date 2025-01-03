#!/bin/bash

# Exit immediately if any command fails
set -e

# Define colors using tput
INFO="$(tput setaf 4)[INFO]$(tput sgr0)"      # Blue
WARNING="$(tput setaf 3)[WARNING]$(tput sgr0)"   # Yellow
WARN="$WARNING"                                # Alias for WARNING
ERROR="$(tput setaf 1)[ERROR]$(tput sgr0)"     # Red
DEBUG="$(tput setaf 2)[DEBUG]$(tput sgr0)"     # Green
RESET="$(tput sgr0)"                           # Reset color

# Logging function
log() {
  local level=$1
  shift
  echo "$(date +'%Y-%m-%d %H:%M:%S') ${level} $@"
}

# Colorize provider logs dynamically
colorize_logs() {
  while IFS= read -r line; do
    case "$line" in
      *INFO*) echo -e "$(date +'%Y-%m-%d %H:%M:%S') ${INFO} ${line}" ;;
      *WARNING*) echo -e "$(date +'%Y-%m-%d %H:%M:%S') ${WARNING} ${line}" ;;
      *ERROR*) echo -e "$(date +'%Y-%m-%d %H:%M:%S') ${ERROR} ${line}" ;;
      *DEBUG*) echo -e "$(date +'%Y-%m-%d %H:%M:%S') ${DEBUG} ${line}" ;;
      *) echo -e "$(date +'%Y-%m-%d %H:%M:%S') ${RESET} ${line}" ;;
    esac
  done
}

# Store the script's directory
SCRIPT_DIR=$(pwd)

# Ensure the input argument is provided
log "$INFO" "Select the migration type:"
log "$INFO" "1) Migrate based on a folder containing terraform.tfstate"
log "$INFO" "2) Migrate based on a specific resource name"

read -rp "Enter your choice (1 or 2): " CHOICE

if [[ "$CHOICE" == "1" ]]; then
  read -rp "Enter the path to the folder containing terraform.tfstate: " INPUT
  if [ ! -f "$INPUT/terraform.tfstate" ]; then
    log "$ERROR" "The specified folder does not contain a terraform.tfstate file."
    exit 1
  fi
  MIGRATION_FOLDER="${INPUT}_migration"
  GENERATE_FLAG="-folder"
elif [[ "$CHOICE" == "2" ]]; then
  log "$INFO" "Available resource types:"
  OPTIONS=("alert" "archive_logs" "archive_metrics" "archive_retentions" "custom_role" "dashboard"
           "dashboards_folder" "events2metrics" "group" "recording_rules_groups_set" "scope"
           "tco_policies_logs" "tco_policies_traces" "webhook")

  select RESOURCE in "${OPTIONS[@]}"; do
    if [[ -n "$RESOURCE" ]]; then
      INPUT="$RESOURCE"
      MIGRATION_FOLDER="./${RESOURCE}_migration"
      GENERATE_FLAG="-type"
      break
    else
      log "$WARNING" "Invalid selection. Please choose a valid resource type."
    fi
  done
else
  log "$ERROR" "Invalid choice. Exiting."
  exit 1
fi

# Ask for Terraform provider version
read -rp "Enter the Terraform provider version to migrate to (e.g., ~>2.0.0): " PROVIDER_VERSION
PROVIDER_VERSION="${PROVIDER_VERSION:->=2.0.0}" # Default to ">=2.0.0" if not provided

CLEANED_JSON_FILE="cleaned_config.json"

# Step 1: Create the migration folder
log "$INFO" "Creating migration folder: $MIGRATION_FOLDER"
mkdir -p "$MIGRATION_FOLDER"

# Step 2: Run the Go script to generate imports.tf
log "$INFO" "Running generate_imports.go with $GENERATE_FLAG..."
go run -ldflags "-X google.golang.org/protobuf/reflect/protoregistry.conflictPolicy=warn" generate_imports.go "$GENERATE_FLAG=$INPUT" -output="$MIGRATION_FOLDER/imports.tf"
if [ $? -ne 0 ]; then
  log "$ERROR" "Failed to run generate_imports.go."
  exit 1
fi
log "$INFO" "Successfully generated imports.tf at $MIGRATION_FOLDER."

# Step 3: Generate provider.tf
log "$INFO" "Generating provider configuration in $MIGRATION_FOLDER/provider.tf..."
cat <<EOL > "$MIGRATION_FOLDER"/provider.tf
terraform {
  required_providers {
    coralogix = {
      version = "$PROVIDER_VERSION"
      source  = "coralogix/coralogix"
    }
  }
}

provider "coralogix" {
  #api_key = "<add your api key here or add env variable CORALOGIX_API_KEY>"
  #env = "<add the environment you want to work at or add env variable CORALOGIX_ENV>"
}
EOL
log "$INFO" "Provider configuration generated in $MIGRATION_FOLDER/provider.tf."

# Navigate to the migration folder
cd "$MIGRATION_FOLDER" || exit 1

# Step 4: Run Terraform init
log "$INFO" "Initializing Terraform in $MIGRATION_FOLDER..."
terraform init 2>&1 | colorize_logs
if [ $? -ne 0 ]; then
  log "$ERROR" "Failed to initialize Terraform."
  exit 1
fi
log "$INFO" "Terraform initialization completed."

# Step 5: Run Terraform plan
log "$INFO" "Running terraform plan in $MIGRATION_FOLDER..."
terraform plan -generate-config-out=generated.tf 2>&1 | colorize_logs
if [ $? -ne 0 ]; then
  log "$ERROR" "Failed to run terraform plan."
  exit 1
fi

# Step 6: Convert the Terraform file to JSON
log "$INFO" "Converting generated.tf to JSON..."
hcl2json < generated.tf > config.json
if [ $? -ne 0 ]; then
  log "$ERROR" "Failed to convert Terraform file to JSON."
  exit 1
fi

# Step 7: Remove null values from the JSON file
log "$INFO" "Removing null values from JSON..."
python3 <<EOF
import json

def remove_nulls(data):
    if isinstance(data, dict):
        return {k: remove_nulls(v) for k, v in data.items() if v is not None}
    elif isinstance(data, list):
        return [remove_nulls(v) for v in data if v is not None]
    else:
        return data

with open("config.json", "r") as f:
    data = json.load(f)

cleaned_data = remove_nulls(data)

with open("$CLEANED_JSON_FILE", "w") as f:
    json.dump(cleaned_data, f, indent=2)
EOF
if [ $? -ne 0 ]; then
  log "$ERROR" "Failed to clean JSON file."
  exit 1
fi
log "$INFO" "Cleaned JSON saved to $CLEANED_JSON_FILE."

# Step 8: Convert JSON back to HCL
log "$INFO" "Navigating back to script's directory: $SCRIPT_DIR"
cd "$SCRIPT_DIR" || exit 1

log "$INFO" "Converting cleaned JSON back to HCL using Go program..."
go run json_to_hcl.go "$MIGRATION_FOLDER/$CLEANED_JSON_FILE" "$MIGRATION_FOLDER/cleaned_config.tf"
if [ $? -ne 0 ]; then
  log "$ERROR" "Failed to convert cleaned JSON back to HCL."
  exit 1
fi

# Step 9: Replace the original Terraform file
mv "$MIGRATION_FOLDER/cleaned_config.tf" "$MIGRATION_FOLDER/generated.tf"
log "$INFO" "Cleaned Terraform file saved as generated.tf"

# Step 10: Run Terraform apply
cd "$MIGRATION_FOLDER" || exit 1
log "$INFO" "Running terraform apply..."
terraform apply 2>&1 | colorize_logs
log "$INFO" "Terraform apply completed."

# Step 11: Cleanup
log "$INFO" "Cleaning up temporary files..."
rm -f imports.tf config.json "$CLEANED_JSON_FILE"
log "$INFO" "Cleanup completed."

log "$INFO" "Script completed successfully."
