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

// TestResourceCoralogixEnrichmentUpdateDeletesOldIdsWhenFieldsEmptied verifies
// that when a fields set is emptied, the update flow still deletes the OLD
// backend enrichments. Deletion IDs are derived from the pre-change set via
// extractOldIdsFromEnrichment (d.GetChange), so an emptied set (proposed len 0)
// no longer skips the Delete and orphans the old backend enrichments. This is
// the regression Codex review comment 4121293986 flagged.
func TestResourceCoralogixEnrichmentUpdateDeletesOldIdsWhenFieldsEmptied(t *testing.T) {
	// Save and restore the package-level seams the update flow calls through.
	origAdd, origDelete := addEnrichmentsFn, deleteEnrichmentsFn
	defer func() {
		addEnrichmentsFn = origAdd
		deleteEnrichmentsFn = origDelete
	}()

	// Delete records its request and succeeds; Add succeeds so no rollback runs.
	var deleteReqs []*cxsdk.DeleteEnrichmentsRequest
	deleteEnrichmentsFn = func(_ context.Context, _ interface{}, req *cxsdk.DeleteEnrichmentsRequest) error {
		deleteReqs = append(deleteReqs, req)
		return nil
	}
	var addReqs []*cxsdk.AddEnrichmentsRequest
	addEnrichmentsFn = func(_ context.Context, _ interface{}, req *cxsdk.AddEnrichmentsRequest) (any, error) {
		addReqs = append(addReqs, req)
		return nil, nil
	}

	// Build a *schema.ResourceData whose OLD (pre-change) state has geo_ip.0.fields
	// with id=42, mirroring the rollback test's flatmap pattern. GetChange(old)
	// then sees field 42.
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

	// Empty the geo_ip fields set so GetChange(new) is empty while GetChange(old)
	// still carries field 42. d.Get would see the empty set and skip the Delete;
	// extractOldIdsFromEnrichment must still surface id 42.
	if err := d.Set("geo_ip", []interface{}{
		map[string]interface{}{
			"fields": schema.NewSet(hashFields(), []interface{}{}),
		},
	}); err != nil {
		t.Fatalf("failed to set emptied geo_ip: %s", err)
	}

	// On the success path the update flow ends by calling
	// resourceCoralogixEnrichmentRead, which needs a live *clientset.ClientSet
	// (not one of the stubbable seams). With a nil meta that Read panics, so we
	// recover from it here: the Delete/Add requests we assert on are already
	// recorded by the time Read runs, and Read behaviour is out of scope for
	// this test.
	func() {
		defer func() { _ = recover() }()
		resourceCoralogixEnrichmentUpdate(context.Background(), d, nil)
	}()

	// Delete must be called exactly once, targeting the OLD enrichment id 42.
	if len(deleteReqs) != 1 {
		t.Fatalf("expected Delete to be called once, got %d", len(deleteReqs))
	}
	// EnrichmentIds is []*wrapperspb.UInt32Value.
	gotIds := deleteReqs[0].GetEnrichmentIds()
	if len(gotIds) != 1 {
		t.Fatalf("expected Delete to target 1 enrichment id, got %d", len(gotIds))
	}
	if gotIds[0].GetValue() != 42 {
		t.Fatalf("expected Delete to target old id 42, got %d", gotIds[0].GetValue())
	}
	// Add must be called exactly once (no rollback; the real Add succeeded).
	if len(addReqs) != 1 {
		t.Fatalf("expected Add to be called once, got %d", len(addReqs))
	}
}
