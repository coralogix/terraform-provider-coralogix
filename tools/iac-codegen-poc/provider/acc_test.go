package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	aiapplications "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_applications_service"
	aievaluations "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_evaluations_service"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const resourceName = "coralogix_ai_evaluation.test"

var factories = map[string]func() (tfprotov6.ProviderServer, error){
	TypeName: providerserver.NewProtocol6WithError(New()),
}

// targets are the targets the test can use, in order. The server may allow
// only one evaluation per application, subsystem, and target.
var targets = []string{"PROMPT", "RESPONSE"}

// slot is where the test creates its evaluation.
type slot struct {
	application, subsystem, target string
}

// TestAccAiEvaluation runs Create, Read, Import, Update, change of the config
// arm, clear, and Delete on EU2. See README.md, "Acceptance test".
func TestAccAiEvaluation(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}
	cs, err := newClientSet()
	if err != nil {
		t.Fatal(err)
	}
	s := freeSlot(t, cs)
	start := time.Now()
	t.Cleanup(func() { deleteLeftovers(t, cs, s, start) })

	var id string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: factories,
		CheckDestroy:             checkDestroy(cs),
		Steps: []resource.TestStep{
			{
				Config: s.config(`
  is_enabled = true
  threshold  = 0.8
  config = {
    pii = { categories = ["EMAIL_ADDRESS", "CREDIT_CARD"] }
  }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					saveID(&id),
					resource.TestCheckResourceAttr(resourceName, "application", s.application),
					resource.TestCheckResourceAttr(resourceName, "subsystem", s.subsystem),
					resource.TestCheckResourceAttr(resourceName, "target", s.target),
					resource.TestCheckResourceAttr(resourceName, "is_enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "threshold", "0.8"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.pii.categories.*", "EMAIL_ADDRESS"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.pii.categories.*", "CREDIT_CARD"),
					resource.TestCheckResourceAttrSet(resourceName, "company_id"),
					resource.TestCheckResourceAttrSet(resourceName, "created_at"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// Update: change config and threshold. is_enabled stays true: the server
				// rejects isEnabled in the update mask (F25).
				Config: s.config(`
  is_enabled = true
  threshold  = 0.9
  config = {
    pii = { categories = ["PHONE_NUMBER", "US_SSN"] }
  }`),
				ConfigPlanChecks: updateInPlace(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr(resourceName, "id", &id),
					resource.TestCheckResourceAttr(resourceName, "is_enabled", "true"),
					resource.TestCheckResourceAttr(resourceName, "threshold", "0.9"),
					resource.TestCheckResourceAttr(resourceName, "config.pii.categories.#", "2"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.pii.categories.*", "PHONE_NUMBER"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.pii.categories.*", "US_SSN"),
				),
			},
			{
				// Change of the config arm: the whole config is replaced.
				Config: s.config(`
  is_enabled = true
  threshold  = 0.9
  config = {
    allowed_topics = { topics = ["billing", "observability"] }
  }`),
				ConfigPlanChecks: updateInPlace(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr(resourceName, "id", &id),
					resource.TestCheckNoResourceAttr(resourceName, "config.pii.categories.#"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.allowed_topics.topics.*", "billing"),
					resource.TestCheckTypeSetElemAttr(resourceName, "config.allowed_topics.topics.*", "observability"),
				),
			},
			{
				// Clear: threshold is in the update mask and not in the body.
				Config: s.config(`
  is_enabled = true
  config = {
    allowed_topics = { topics = ["billing", "observability"] }
  }`),
				ConfigPlanChecks: updateInPlace(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPtr(resourceName, "id", &id),
					resource.TestCheckNoResourceAttr(resourceName, "threshold"),
				),
			},
		},
	})
}

// config returns the HCL of the evaluation with the attributes in body.
func (s slot) config(body string) string {
	return fmt.Sprintf(`resource "coralogix_ai_evaluation" "test" {
  application = %q
  subsystem   = %q
  target      = %q
%s
}
`, s.application, s.subsystem, s.target, body)
}

func updateInPlace() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
		plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
	}}
}

func saveID(id *string) resource.TestCheckFunc {
	return func(st *terraform.State) error {
		rs, ok := st.RootModule().Resources[resourceName]
		if !ok || rs.Primary.ID == "" {
			return fmt.Errorf("%s has no id", resourceName)
		}
		*id = rs.Primary.ID
		return nil
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

// checkDestroy fails when an evaluation in the state still exists.
func checkDestroy(cs *cxsdk.ClientSet) resource.TestCheckFunc {
	return func(st *terraform.State) error {
		for _, rs := range st.RootModule().Resources {
			if rs.Type != "coralogix_ai_evaluation" {
				continue
			}
			_, httpResp, err := cs.AIEvaluations().AiEvaluationsServiceGetAiEvaluation(context.Background(), rs.Primary.ID).Execute()
			if err == nil {
				return fmt.Errorf("AI evaluation %s still exists", rs.Primary.ID)
			}
			if err := cxsdk.NewAPIError(httpResp, err); cxsdk.Code(err) != http.StatusNotFound {
				return err
			}
		}
		return nil
	}
}
