package leftovers

import (
	"context"
	"os"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
)

// AiEvaluations deletes every AI evaluation of the application and subsystem that the generated
// ai_evaluation test uses (AI_EVALUATION_APPLICATION and AI_EVALUATION_SUBSYSTEM). An application,
// a subsystem, and a target hold one evaluation, so an evaluation that a cancelled run left behind
// makes every later create fail with 409. The handwritten tests never use this application (see
// testAccAIApplicationIsGeneratedTestSlot), so nothing else holds an evaluation there.
func AiEvaluations(t *testing.T, clients *clientset.ClientSet) {
	t.Helper()
	application, subsystem := os.Getenv("AI_EVALUATION_APPLICATION"), os.Getenv("AI_EVALUATION_SUBSYSTEM")
	if application == "" {
		return
	}
	evaluations := clients.AIEvaluations()
	ctx := context.Background()
	request := evaluations.AiEvaluationsServiceListAiEvaluations(ctx).Application(application).PageSize(200).PageOffset(0)
	if subsystem != "" {
		request = request.Subsystem(subsystem)
	}
	list, _, err := request.Execute()
	if err != nil {
		t.Fatalf("list AI evaluations of application %q: %v", application, err)
	}
	for _, evaluation := range list.GetAiEvaluations() {
		if evaluation.GetId() == "" {
			continue
		}
		t.Logf("deleting AI evaluation %s of application %q, left by an earlier run", evaluation.GetId(), application)
		if _, _, err := evaluations.AiEvaluationsServiceDeleteAiEvaluation(ctx, evaluation.GetId()).Execute(); err != nil {
			t.Fatalf("delete AI evaluation %s: %v", evaluation.GetId(), err)
		}
	}
}
