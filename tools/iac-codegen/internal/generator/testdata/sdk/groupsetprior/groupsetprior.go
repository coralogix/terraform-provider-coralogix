// Package groupsetprior is the frozen prior schema that the GroupSetThing overrides name. Version 0
// has the document of the current version and groups of another shape.
package groupsetprior

import "github.com/hashicorp/terraform-plugin-framework/resource/schema"

// V0 returns schema version 0 of GroupSetThing.
func V0() schema.Schema {
	return schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id":       schema.StringAttribute{Computed: true},
			"document": schema.StringAttribute{Optional: true},
			"group":    schema.StringAttribute{Optional: true},
		},
	}
}
