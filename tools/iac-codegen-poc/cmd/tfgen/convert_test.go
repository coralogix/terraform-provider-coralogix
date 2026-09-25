package main

import (
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// TestBuildConvRejects changes the model or the SDK names so that expand or
// flatten cannot be generated. The generated code itself is tested in
// generated/aievaluation.
func TestBuildConvRejects(t *testing.T) {
	cases := []struct {
		name       string
		change     func(r *model.Resource)
		changeRefs func(refs []sdkRef)
		want       string
	}{
		{
			name:   "date-time in a request",
			change: func(r *model.Resource) { topField(t, r, "createdAt").Create = &model.Attrs{} },
			want:   "create.body: created_at: date-time in a request is not supported",
		},
		{
			name: "set of objects",
			change: func(r *model.Resource) {
				f := nestedField(t, r, "config", "customEvaluation", "examples")
				f.Type = &model.Type{Kind: model.Set, Elem: f.Type.Elem}
			},
			want: "customEvaluation: examples: set of object is not supported",
		},
		{
			name: "signed 64-bit integer",
			change: func(r *model.Resource) {
				nestedField(t, r, "config", "sqlLoad", "cteLimit").Type = &model.Type{Kind: model.Integer, Format: "int64"}
			},
			want: `sqlLoad: cteLimit: integer format "int64" is not supported`,
		},
		{
			name: "SDK type does not fit the conversion",
			changeRefs: func(refs []sdkRef) {
				for i := range refs {
					if refs[i].Path == "fields.config.sqlLoad.joinLimit" {
						refs[i].Want = "*int64"
					}
				}
			},
			want: "SDK field SqlLoadConfig.JoinLimit has type *int64, the uint64 conversion needs *string",
		},
		{
			name: "update mask is not a string",
			changeRefs: func(refs []sdkRef) {
				for i := range refs {
					if refs[i].Path == "update.body.updateMask" {
						refs[i].Want = "[]string"
					}
				}
			},
			want: "the update mask needs *string",
		},
		{
			name:   "Update field name is not a mask entry",
			change: func(r *model.Resource) { topField(t, r, "threshold").Name = "max-threshold" },
			want:   "update.body.max-threshold: the name is not a valid update mask entry",
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
			_, err = buildConv(r, refs)
			if err == nil {
				t.Fatal("no error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error:\n%v\nwant it to contain:\n%s", err, c.want)
			}
		})
	}
}
