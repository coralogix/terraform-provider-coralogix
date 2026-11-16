package generator

import (
	"fmt"
	"os"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
)

func TestDebugFromList(t *testing.T) {
	data, _ := os.ReadFile("/Users/ryan/.paseo-work-cx65729-stack2/candidate-spec/sdk-probe.yaml")
	file, _ := readOverrides("/Users/ryan/.paseo-work-cx65729-stack2/user-overrides.yaml")
	vr, err := validateOpenAPIWith(data, "rbac.v2.User", model.OperationIDs{}, "", "", file)
	if err != nil {
		return
	}
	r := vr.resource
	fmt.Println("GetFromList:", r.GetFromList, "| NextPage:", r.NextPage != nil)
}
