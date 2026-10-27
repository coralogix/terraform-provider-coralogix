package utils

import (
	"testing"

	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func TestOverrideDatasourceDescriptions(t *testing.T) {
	s := datasourceschema.Schema{
		MarkdownDescription: "resource text",
		Attributes: map[string]datasourceschema.Attribute{
			"id": datasourceschema.StringAttribute{Computed: true, MarkdownDescription: "old id"},
			"rules": datasourceschema.SetNestedAttribute{
				Computed: true,
				NestedObject: datasourceschema.NestedAttributeObject{Attributes: map[string]datasourceschema.Attribute{
					"allocation": datasourceschema.Float32Attribute{Computed: true, MarkdownDescription: "old allocation"},
				}},
			},
		},
	}
	OverrideDatasourceDescriptions(&s, map[string]string{"": "new text", "id": "new id", "rules.allocation": "new allocation"})
	if s.MarkdownDescription != "new text" || s.Description != "new text" {
		t.Fatalf("schema description = %q, %q", s.Description, s.MarkdownDescription)
	}
	if got := s.Attributes["id"].GetMarkdownDescription(); got != "new id" {
		t.Fatalf("id description = %q", got)
	}
	nested := s.Attributes["rules"].(datasourceschema.SetNestedAttribute).NestedObject.Attributes["allocation"]
	if nested.GetMarkdownDescription() != "new allocation" || nested.GetDescription() != "new allocation" {
		t.Fatalf("allocation description = %q", nested.GetMarkdownDescription())
	}
	if !s.Attributes["id"].IsComputed() {
		t.Fatal("the override changed the flags of id")
	}
}

func TestOverrideDatasourceDescriptionsRejectsAMissingAttribute(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for an attribute that the data source does not have")
		}
	}()
	OverrideDatasourceDescriptions(&datasourceschema.Schema{}, map[string]string{"nope": "x"})
}
