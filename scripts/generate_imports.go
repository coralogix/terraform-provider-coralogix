package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
)

var envToGrpcUrl = map[string]string{
	"APAC1":   "ng-api-grpc.app.coralogix.in:443",
	"AP1":     "ng-api-grpc.app.coralogix.in:443",
	"APAC2":   "ng-api-grpc.coralogixsg.com:443",
	"AP2":     "ng-api-grpc.coralogixsg.com:443",
	"APAC3":   "ng-api-grpc.ap3.coralogix.com:443",
	"AP3":     "ng-api-grpc.ap3.coralogix.com:443",
	"EUROPE1": "ng-api-grpc.coralogix.com:443",
	"EU1":     "ng-api-grpc.coralogix.com:443",
	"EUROPE2": "ng-api-grpc.eu2.coralogix.com:443",
	"EU2":     "ng-api-grpc.eu2.coralogix.com:443",
	"USA1":    "ng-api-grpc.coralogix.us:443",
	"US1":     "ng-api-grpc.coralogix.us:443",
	"USA2":    "ng-api-grpc.cx498.coralogix.com:443",
	"US2":     "ng-api-grpc.cx498.coralogix.com:443",
}

// Resource represents a resource in the Terraform state file
type Resource struct {
	Type      string `json:"type"`
	Name      string `json:"name"`
	Instances []struct {
		Attributes struct {
			ID string `json:"id"`
		} `json:"attributes"`
	} `json:"instances"`
}

// TFState represents the structure of the Terraform state file
type TFState struct {
	Resources []Resource `json:"resources"`
}

// findStateFile searches for a .tfstate file in the specified folder
func findStateFile(folderPath string) (string, error) {
	files, err := ioutil.ReadDir(folderPath)
	if err != nil {
		return "", fmt.Errorf("error reading folder: %v", err)
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".tfstate" {
			return filepath.Join(folderPath, file.Name()), nil
		}
	}

	return "", errors.New("no .tfstate file found in the specified folder")
}

// generateImports reads a Terraform state file and generates an imports.tf file
func generateImportsFromState(tfstatePath string, outputPath string) error {
	// Read the tfstate file
	tfstateData, err := ioutil.ReadFile(tfstatePath)
	if err != nil {
		return fmt.Errorf("error reading tfstate file: %v", err)
	}

	// Parse the JSON data
	var tfstate TFState
	err = json.Unmarshal(tfstateData, &tfstate)
	if err != nil {
		return fmt.Errorf("error parsing tfstate JSON: %v", err)
	}

	// Prepare the imports content
	importsContent := ""

	for _, resource := range tfstate.Resources {
		// Process only coralogix resources
		if strings.HasPrefix(resource.Type, "coralogix_") {
			for _, instance := range resource.Instances {
				// Add the import block to the content
				importsContent += fmt.Sprintf(`import {
  to = %s.%s
  id = "%s"
}

`, resource.Type, resource.Name, instance.Attributes.ID)
			}
		}
	}

	// Write the imports.tf file
	err = ioutil.WriteFile(outputPath, []byte(importsContent), 0644)
	if err != nil {
		return fmt.Errorf("error writing imports.tf file: %v", err)
	}

	return nil
}

type IdAndName struct {
	Id   string
	Name string
}

func main() {
	// Parse the folder path from the command-line arguments
	resourceType := flag.String("type", "", "Type of the resource to import")
	folderPath := flag.String("folder", "", "Path to the folder containing the .tfstate file")
	outputPath := flag.String("output", "imports.tf", "Path to the output file")
	flag.Parse()

	if *resourceType != "" {
		var idsAndNames []IdAndName
		switch *resourceType {
		case "alert":
			apiKey := os.Getenv("CORALOGIX_API_KEY")
			region := os.Getenv("CORALOGIX_ENV")
			url := envToGrpcUrl[region]
			alertClient := cxsdk.NewAlertsClient(cxsdk.NewCallPropertiesCreator(url, cxsdk.NewAuthContext(apiKey, apiKey)))
			alerts, err := alertClient.List(context.Background(), &cxsdk.ListAlertDefsRequest{})
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				os.Exit(1)
			}
			for _, alert := range alerts.GetAlertDefs() {
				alertName := toValidResourceName(alert.GetAlertDefProperties().GetName().GetValue())
				idsAndNames = append(idsAndNames, IdAndName{Id: alert.GetId().GetValue(), Name: alertName})
			}
		}
		err := generateImportsFromIds(*resourceType, *outputPath, idsAndNames)
		if err != nil {
			fmt.Printf("Error generating imports.tf: %v\n", err)
			os.Exit(1)
		}
	} else {
		if *folderPath == "" {
			fmt.Println("Error: Please provide a folder path using the -folder flag")
			os.Exit(1)
		}

		// Find the .tfstate file in the folder
		tfstatePath, err := findStateFile(*folderPath)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}

		// Generate the imports.tf file
		err = generateImportsFromState(tfstatePath, *outputPath)
		if err != nil {
			fmt.Printf("Error generating imports.tf: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("`imports.tf` file has been generated at: %s\n", outputPath)
}

func toValidResourceName(value string) string {
	return strings.ReplaceAll(strings.ToLower(value), " ", "_")
}

func generateImportsFromIds(resourceType, outputFilePath string, idsAndNames []IdAndName) error {
	importsContent := ""

	for _, idAndName := range idsAndNames {
		importsContent += fmt.Sprintf(`import {
  to = coralogix_%s.%s
  id = "%s"
}

`, resourceType, idAndName.Name, idAndName.Id)
	}

	err := ioutil.WriteFile(outputFilePath, []byte(importsContent), 0644)
	if err != nil {
		return fmt.Errorf("error writing imports.tf file: %v", err)
	}

	return nil

}
