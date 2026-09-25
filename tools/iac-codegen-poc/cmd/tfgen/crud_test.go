package main

import (
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// TestBuildCRUDRejects changes the model or the SDK names so that the CRUD
// code cannot be generated. The generated code itself is tested in
// generated/aievaluation.
func TestBuildCRUDRejects(t *testing.T) {
	setWant := func(path, want string) func(refs []sdkRef) {
		return func(refs []sdkRef) {
			for i := range refs {
				if refs[i].Path == path {
					refs[i].Want = want
				}
			}
		}
	}
	cases := []struct {
		name       string
		change     func(r *model.Resource)
		changeRefs func(refs []sdkRef)
		want       string
	}{
		{
			name:       "id is not a string",
			changeRefs: setWant("fields.id", "*int64"),
			want:       "SDK field AiEvaluation.Id has type *int64, the id needs *string",
		},
		{
			name:       "response does not hold the resource",
			changeRefs: setWant("get.response.aiEvaluation", "*string"),
			want:       "SDK field GetAiEvaluationResponse.AiEvaluation has type *string, want *AiEvaluation",
		},
		{
			name:   "Get does not return the resource",
			change: func(r *model.Resource) { r.Get.Response.Field = "" },
			want:   `get: response field "" is not supported`,
		},
		{
			name:   "Delete has a body",
			change: func(r *model.Resource) { r.Delete.Body = "inline" },
			want:   `delete: request body "inline" is not supported`,
		},
	}
	doc, _ := testSDK(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := model.Build(doc, "AiEvaluation")
			if err != nil {
				t.Fatal(err)
			}
			if c.change != nil {
				c.change(r)
			}
			refs, err := resolveSDKNames(r, "AI Evaluations Service")
			if err != nil {
				t.Fatal(err)
			}
			if c.changeRefs != nil {
				c.changeRefs(refs)
			}
			_, err = buildCRUD(r, refs)
			if err == nil {
				t.Fatal("no error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error:\n%v\nwant it to contain:\n%s", err, c.want)
			}
		})
	}
}
