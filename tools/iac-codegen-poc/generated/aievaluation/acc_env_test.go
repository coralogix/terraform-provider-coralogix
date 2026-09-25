// This file is handwritten. It is the environment hook of the generated
// acceptance test (acc_test.go): it finds where the test can create an AI
// evaluation, and fills the ${name} placeholders of spec/acc/AiEvaluation.yaml.

package aievaluation_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	aiapplications "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_applications_service"
	aievaluations "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_evaluations_service"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/provider"
)

// targets are the targets the test can use, in order. The server may allow
// only one evaluation per application, subsystem, and target.
var targets = []string{"PROMPT", "RESPONSE"}

// slot is where the test creates its evaluation.
type slot struct {
	application, subsystem, target string
}

func accSetup(t *testing.T) accEnv {
	t.Helper()
	cs, err := provider.ClientSetFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	s := freeSlot(t, cs)
	start := time.Now()
	t.Cleanup(func() { deleteLeftovers(t, cs, s, start) })
	return accEnv{
		Provider: provider.TypeName,
		Factories: map[string]func() (tfprotov6.ProviderServer, error){
			provider.TypeName: providerserver.NewProtocol6WithError(provider.New()),
		},
		Client: cs,
		Values: map[string]string{"application": s.application, "subsystem": s.subsystem, "target": s.target},
	}
}

// freeSlot returns the first AI application and a target that no evaluation
// on it uses yet. An evaluation with no target may block every target, so
// the test does not use such an application.
func freeSlot(t *testing.T, cs *cxsdk.ClientSet) slot {
	t.Helper()
	ctx := context.Background()
	apps, httpResp, err := cs.AIApplications().AiApplicationsServiceListAiApplications(ctx).
		PageSize(10).PageOffset(0).Execute()
	if err != nil {
		t.Fatalf("list AI applications: %v", cxsdk.NewAPIError(httpResp, err))
	}
	i := slices.IndexFunc(apps.GetAiApplications(), func(a aiapplications.AiApplication) bool { return a.GetApplication() != "" })
	if i < 0 {
		t.Fatal("the account has no AI application")
	}
	app := apps.GetAiApplications()[i]
	s := slot{application: app.GetApplication(), subsystem: app.GetSubsystem()}
	if s.subsystem == "" {
		t.Fatalf("AI application %q has no subsystem. The schema requires one (F8).", s.application)
	}
	used := map[string]bool{}
	for _, e := range listEvaluations(t, cs, s) {
		if e.GetTarget() == "" {
			t.Fatalf("AI application %q has an evaluation with no target", s.application)
		}
		used[string(e.GetTarget())] = true
	}
	for _, target := range targets {
		if !used[target] {
			s.target = target
			return s
		}
	}
	t.Fatalf("AI application %q uses all targets %v", s.application, targets)
	return s
}

func listEvaluations(t *testing.T, cs *cxsdk.ClientSet, s slot) []aievaluations.AiEvaluation {
	t.Helper()
	out, httpResp, err := cs.AIEvaluations().AiEvaluationsServiceListAiEvaluations(context.Background()).
		Application(s.application).Subsystem(s.subsystem).PageSize(200).PageOffset(0).Execute()
	if err != nil {
		t.Fatalf("list AI evaluations: %v", cxsdk.NewAPIError(httpResp, err))
	}
	return out.GetAiEvaluations()
}

// deleteLeftovers deletes every evaluation on the slot that was created
// after start. Terraform normally deletes it already. This covers a failure
// before the state has the id.
func deleteLeftovers(t *testing.T, cs *cxsdk.ClientSet, s slot, start time.Time) {
	for _, e := range listEvaluations(t, cs, s) {
		if string(e.GetTarget()) != s.target || e.GetCreatedAt().Before(start.Add(-time.Minute)) {
			continue
		}
		_, httpResp, err := cs.AIEvaluations().AiEvaluationsServiceDeleteAiEvaluation(context.Background(), e.GetId()).Execute()
		if err != nil {
			t.Errorf("delete leftover AI evaluation %s: %v", e.GetId(), cxsdk.NewAPIError(httpResp, err))
			continue
		}
		t.Logf("deleted leftover AI evaluation %s", e.GetId())
	}
}
