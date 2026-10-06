// Package legacyprior is the frozen prior schema that the LegacyThing
// overrides name. It lets the generated legacy resource compile in tests.
package legacyprior

import "github.com/hashicorp/terraform-plugin-framework/resource/schema"

// V1 returns schema version 1 of LegacyThing.
func V1() schema.Schema {
	return schema.Schema{Version: 1}
}
