package acctest

import (
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/acctest/leftovers"
)

// RemoveLeftoverAiEvaluations deletes the evaluations that a cancelled run left on the application
// of the generated ai_evaluation test. See leftovers.AiEvaluations. configs is unused: the
// application comes from the environment.
func RemoveLeftoverAiEvaluations(t *testing.T, _ ...string) {
	t.Helper()
	clients, err := newClientSet()
	if err != nil {
		t.Fatal(err)
	}
	leftovers.AiEvaluations(t, clients)
}
