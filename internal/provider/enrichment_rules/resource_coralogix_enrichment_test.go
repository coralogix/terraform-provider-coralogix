package enrichment_rules

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestExpandAwsUsesPublicResourceAttribute(t *testing.T) {
	fields := schema.NewSet(hashAwsFields(), []interface{}{
		map[string]interface{}{
			"name":     "coralogix.metadata.aws_resource_id",
			"resource": "ec2",
		},
	})

	got := expandAws(map[string]interface{}{"fields": fields})
	if len(got) != 1 {
		t.Fatalf("expanded enrichments = %d, want 1", len(got))
	}
	if resourceType := got[0].GetEnrichmentType().GetAws().GetResourceType().GetValue(); resourceType != "ec2" {
		t.Fatalf("resource = %q, want ec2", resourceType)
	}
}

// TestResourceCoralogixEnrichmentUpdateRollsBackOnAddFailure verifies that when
// the update flow deletes the existing enrichments and the subsequent Add fails,
// it best-effort re-adds the OLD enrichment set and returns the ORIGINAL Add
// error. This guards the non-atomic Delete->Add update from leaving the resource
// with all of its enrichments removed. Mirrors the coralogix_data_enrichments
// rollback fix (#765, e2fe61a).
func TestResourceCoralogixEnrichmentUpdateRollsBackOnAddFailure(t *testing.T) {
	// Save and restore the package-level seams the update flow calls through.
	origAdd, origDelete := addEnrichmentsFn, deleteEnrichmentsFn
	defer func() {
		addEnrichmentsFn = origAdd
		deleteEnrichmentsFn = origDelete
	}()

	// Delete succeeds so the flow reaches Add.
	var deleteCalled bool
	deleteEnrichmentsFn = func(_ context.Context, _ interface{}, _ *cxsdk.DeleteEnrichmentsRequest) error {
		deleteCalled = true
		return nil
	}

	// Add fails on the first call (the real update) and succeeds on the second
	// (the rollback re-add), recording each request so we can assert the rollback
	// carried the OLD config.
	addErr := errors.New("add enrichments failed")
	var addReqs []*cxsdk.AddEnrichmentsRequest
	addEnrichmentsFn = func(_ context.Context, _ interface{}, req *cxsdk.AddEnrichmentsRequest) (any, error) {
		addReqs = append(addReqs, req)
		if len(addReqs) == 1 {
			return nil, addErr
		}
		return nil, nil
	}

	// Build a *schema.ResourceData from an explicit terraform.InstanceState so
	// the geo_ip block (and its computed id) actually persists into state. A
	// Set/State() round-trip on a fresh ResourceData does NOT persist the block,
	// so we construct the flatmap directly. The set-element hash is derived from
	// the production hashFields() over the same field map, so the keys stay
	// correct even if the field schema changes.
	r := ResourceCoralogixEnrichment()
	field := map[string]interface{}{
		"name": "coralogix.metadata.remote_ip",
		"id":   42,
	}
	fieldHash := hashFields()(field)
	prefix := fmt.Sprintf("geo_ip.0.fields.%d", fieldHash)
	instanceState := &terraform.InstanceState{
		ID: "geo_ip",
		Attributes: map[string]string{
			"geo_ip.#":          "1",
			"geo_ip.0.fields.#": "1",
			prefix + ".name":    "coralogix.metadata.remote_ip",
			prefix + ".id":      strconv.Itoa(42),
		},
	}
	d := r.Data(instanceState)

	diags := resourceCoralogixEnrichmentUpdate(context.Background(), d, nil)

	if !deleteCalled {
		t.Fatalf("expected delete to be called")
	}
	if len(addReqs) != 2 {
		t.Fatalf("expected Add to be called twice (update + rollback), got %d", len(addReqs))
	}
	// The rollback re-add must carry the OLD geo_ip config (one geo_ip field).
	rollback := addReqs[1]
	if got := len(rollback.GetRequestEnrichments()); got != 1 {
		t.Fatalf("rollback add carried %d enrichments, want 1 (the old geo_ip field)", got)
	}
	if rollback.GetRequestEnrichments()[0].GetEnrichmentType().GetGeoIp() == nil {
		t.Fatalf("rollback add did not carry the old geo_ip enrichment type")
	}
	// The update must return the ORIGINAL Add error, not the rollback outcome.
	if !diags.HasError() {
		t.Fatalf("expected update to return an error, got none")
	}
	if !strings.Contains(diags[0].Summary, addErr.Error()) {
		t.Fatalf("expected the original Add error %q to be returned, got %q", addErr.Error(), diags[0].Summary)
	}
}
