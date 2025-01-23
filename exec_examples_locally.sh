#!/bin/bash

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to log with timestamp
log() {
    echo -e "[$(date +'%Y-%m-%d %H:%M:%S')] $1"
}

# Function to replace provider block in terraform files
replace_provider() {
    local file=$1
    sed -i.bak '
        /terraform {/,/}/ {
            /required_providers {/,/}/ {
                /coralogix = {/,/}/ {
                    s/version = ".*"/version = "1.5"/
                    s/source  = "coralogix\/coralogix"/source  = "locally\/debug\/coralogix"/
                }
            }
        }
    ' "$file"
    rm -f "${file}.bak"
}

# Function to revert provider block in terraform files
revert_provider() {
    local file=$1
    sed -i.bak '
        /terraform {/,/}/ {
            /required_providers {/,/}/ {
                /coralogix = {/,/}/ {
                    s/version = ".*"/version = "~> 2.0"/
                    s/source  = "locally\/debug\/coralogix"/source  = "coralogix\/coralogix"/
                }
            }
        }
    ' "$file"
    rm -f "${file}.bak"
}

# Make and install provider
log "${YELLOW}Building and installing provider...${NC}"
make install
if [ $? -ne 0 ]; then
    log "${RED}Failed to build and install provider${NC}"
    exit 1
fi

# Find all terraform files in examples directory
log "${YELLOW}Finding all terraform files...${NC}"
TERRAFORM_FILES=$(find examples/resources -type f -name "*.tf")

# Arrays to store results
declare -a SUCCESSFUL_EXAMPLES
declare -a FAILED_EXAMPLES

# Process each terraform file
for tf_file in $TERRAFORM_FILES; do
    dir=$(dirname "$tf_file")
    
    log "${YELLOW}Processing $tf_file${NC}"
    
    # Replace provider source and version
    replace_provider "$tf_file"
    
    # Create working directory and copy files
    working_dir="/tmp/terraform-test-$(basename $dir)"
    rm -rf "$working_dir"
    mkdir -p "$working_dir"
    cp -r "$dir"/* "$working_dir"/
    
    # Try to initialize and apply
    cd "$working_dir"
    
    log "Initializing terraform in $working_dir"
    terraform init -no-color > /dev/null 2>&1
    
    if [ $? -eq 0 ]; then
        log "Running terraform plan"
        terraform plan -no-color > /dev/null 2>&1
        
        if [ $? -eq 0 ]; then
            SUCCESSFUL_EXAMPLES+=("$tf_file")
            log "${GREEN}✓ Success: $tf_file${NC}"
        else
            FAILED_EXAMPLES+=("$tf_file")
            log "${RED}✗ Failed plan: $tf_file${NC}"
        fi
    else
        FAILED_EXAMPLES+=("$tf_file")
        log "${RED}✗ Failed init: $tf_file${NC}"
    fi
    
    # Cleanup
    cd - > /dev/null
    rm -rf "$working_dir"
done

# Revert provider changes
log "${YELLOW}Reverting provider changes...${NC}"
for tf_file in $TERRAFORM_FILES; do
    revert_provider "$tf_file"
done

# Print summary
log "\n${YELLOW}=== Test Summary ===${NC}"
log "${GREEN}Successful examples (${#SUCCESSFUL_EXAMPLES[@]}):"
for success in "${SUCCESSFUL_EXAMPLES[@]}"; do
    log "  ✓ $success"
done

log "\n${RED}Failed examples (${#FAILED_EXAMPLES[@]}):"
for failure in "${FAILED_EXAMPLES[@]}"; do
    log "  ✗ $failure"
done

# Exit with error if any failures
if [ ${#FAILED_EXAMPLES[@]} -gt 0 ]; then
    exit 1
fi
