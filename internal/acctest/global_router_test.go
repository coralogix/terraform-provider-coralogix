package acctest

import (
	"slices"
	"testing"
)

func TestGlobalRouterLabelsReadsEveryRouterOfTheConfigs(t *testing.T) {
	full := `resource "coralogix_connector" "http" {
  id = "acc-1-http"
}

resource "coralogix_global_router" "test" {
  name = "acc-1-name"
  rules = [{ targets = [{ connector_id = coralogix_connector.http.id }] }]
  routing_labels = {
    environment = "tf-acc-env"
    service     = "tf-acc-service"
    team        = "tf-acc-team"
  }
}`
	minimal := `resource "coralogix_global_router" "test" {
  name           = "acc-1-name"
  routing_labels = { environment = "tf-acc-minimal" }
}

resource "coralogix_global_router" "unlabeled" {
  name = "acc-1-unlabeled"
}`
	got, err := globalRouterLabels([]string{full, minimal, full})
	if err != nil {
		t.Fatal(err)
	}
	want := []routingLabels{
		{Environment: "tf-acc-env", Service: "tf-acc-service", Team: "tf-acc-team"},
		{Environment: "tf-acc-minimal"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("labels = %+v, want %+v", got, want)
	}
}

// A label that is not a literal cannot be compared with the routers of the API, so the sweep stops
// instead of deleting the wrong router or none.
func TestGlobalRouterLabelsRejectsAnExpression(t *testing.T) {
	config := `resource "coralogix_global_router" "test" {
  routing_labels = { environment = var.environment }
}`
	if _, err := globalRouterLabels([]string{config}); err == nil {
		t.Fatal("want an error for a routing label that is not a literal")
	}
}
