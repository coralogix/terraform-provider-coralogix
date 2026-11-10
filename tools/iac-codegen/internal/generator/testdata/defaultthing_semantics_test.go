package defaultthing

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

	"example.com/iac-test-sdk/go/openapi/gen/default_things_service"
)

type defaultProvider struct{}

func (defaultProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "test"
}

func (defaultProvider) Schema(context.Context, provider.SchemaRequest, *provider.SchemaResponse) {}

func (defaultProvider) Configure(context.Context, provider.ConfigureRequest, *provider.ConfigureResponse) {
}

func (defaultProvider) Resources(context.Context) []func() frameworkresource.Resource {
	return []func() frameworkresource.Resource{func() frameworkresource.Resource {
		return NewResourceWithClient(&default_things_service.DefaultThingsServiceAPIService{}, Hooks{})
	}}
}

func (defaultProvider) DataSources(context.Context) []func() datasource.DataSource { return nil }

func factories() map[string]func() (tfprotov6.ProviderServer, error) {
	return map[string]func() (tfprotov6.ProviderServer, error){"test": providerserver.NewProtocol6WithError(defaultProvider{})}
}

const thing = "test_default_thing.t"

func config(body string) string {
	return "resource \"test_default_thing\" \"t\" {\n" + body + "}\n"
}

// The plan of a thing that sets nothing holds the defaults: an object of nulls, an object with
// values and a nested object, a null list, and an empty list. After the apply, the service returns
// the objects with empty lists and the stored values, and the plan that the test framework runs
// next must be empty.
func TestOmittedFieldsPlanTheirDefaults(t *testing.T) {
	default_things_service.Created = nil
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories(),
		Steps: []resource.TestStep{
			{
				Config: config(""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectKnownValue(thing, tfjsonpath.New("routing"), knownvalue.ObjectExact(map[string]knownvalue.Check{
						"targets": knownvalue.Null(), "regions": knownvalue.Null(), "severities": knownvalue.Null(), "note": knownvalue.Null(),
					})),
					plancheck.ExpectKnownValue(thing, tfjsonpath.New("limits"), knownvalue.ObjectExact(map[string]knownvalue.Check{
						"unit": knownvalue.StringExact("never"), "enabled": knownvalue.Bool(false), "count": knownvalue.Null(),
						"window": knownvalue.ObjectExact(map[string]knownvalue.Check{"days": knownvalue.Int64Exact(7)}),
					})),
					plancheck.ExpectKnownValue(thing, tfjsonpath.New("overrides"), knownvalue.Null()),
					plancheck.ExpectKnownValue(thing, tfjsonpath.New("rules"), knownvalue.ListSizeExact(0)),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr(thing, "routing.targets.#"),
					resource.TestCheckNoResourceAttr(thing, "routing.regions.#"),
					resource.TestCheckResourceAttr(thing, "limits.unit", "never"),
					resource.TestCheckResourceAttr(thing, "limits.enabled", "false"),
					resource.TestCheckResourceAttr(thing, "limits.window.days", "7"),
					resource.TestCheckNoResourceAttr(thing, "overrides.#"),
					resource.TestCheckResourceAttr(thing, "rules.#", "0"),
				),
			},
			// A refresh and a plan change nothing.
			{Config: config(""), PlanOnly: true},
		},
	})
	if len(default_things_service.Created) != 1 {
		t.Fatalf("create bodies = %d, want 1", len(default_things_service.Created))
	}
	body := default_things_service.Created[0]
	// The object of nulls is sent as an empty object, and the object with values as those values.
	if body.Routing == nil || body.Routing.Targets != nil || body.Routing.Regions != nil {
		t.Fatalf("sent routing = %+v, want an empty object", body.Routing)
	}
	if body.Limits == nil || body.Limits.Unit == nil || *body.Limits.Unit != default_things_service.DEFAULTUNIT_UNIT_NEVER_OR_UNSPECIFIED ||
		body.Limits.Enabled == nil || *body.Limits.Enabled || body.Limits.Window == nil || *body.Limits.Window.Days != 7 {
		t.Fatalf("sent limits = %+v, want the defaults", body.Limits)
	}
	if body.Overrides != nil {
		t.Fatalf("sent overrides = %+v, want none for the null list", body.Overrides)
	}
}

// A configured value replaces the default, and the fields of the object that it leaves out take
// their own defaults. Removing the values again plans the defaults, and the plan is empty after
// each apply.
func TestConfiguredValuesReplaceTheDefaultAndRemovalRestoresIt(t *testing.T) {
	full := config(`  routing = { targets = ["a", "b"], severities = ["high"] }
  limits  = { unit = "hours", count = 3 }
  overrides = [{ field_name = "f", template = "t" }]
  rules   = [{ name = "r", expression = "e" }]
`)
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories(),
		Steps: []resource.TestStep{
			{
				Config: full,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectKnownValue(thing, tfjsonpath.New("limits"), knownvalue.ObjectExact(map[string]knownvalue.Check{
						"unit": knownvalue.StringExact("hours"), "enabled": knownvalue.Bool(false), "count": knownvalue.Int64Exact(3),
						"window": knownvalue.ObjectExact(map[string]knownvalue.Check{"days": knownvalue.Int64Exact(7)}),
					})),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(thing, "routing.targets.#", "2"),
					resource.TestCheckNoResourceAttr(thing, "routing.regions.#"),
					resource.TestCheckResourceAttr(thing, "limits.unit", "hours"),
					resource.TestCheckResourceAttr(thing, "limits.count", "3"),
					resource.TestCheckResourceAttr(thing, "overrides.#", "1"),
					resource.TestCheckResourceAttr(thing, "rules.#", "1"),
				),
			},
			{Config: full, PlanOnly: true},
			{
				Config: config(""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr(thing, "routing.targets.#"),
					resource.TestCheckResourceAttr(thing, "limits.unit", "never"),
					resource.TestCheckNoResourceAttr(thing, "limits.count"),
					resource.TestCheckNoResourceAttr(thing, "overrides.#"),
					resource.TestCheckResourceAttr(thing, "rules.#", "0"),
				),
			},
			{Config: config(""), PlanOnly: true},
		},
	})
}

// An empty object that the user writes plans the defaults of its fields, not the default of the
// object: an object that is set but leaves a field out gets the default of that field, and a nested
// object that the user writes empty has no value of its own.
func TestAnEmptyObjectTakesTheDefaultsOfItsFields(t *testing.T) {
	body := config("  routing = {}\n  limits = { window = {} }\n")
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories(),
		Steps: []resource.TestStep{
			{
				Config: body,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr(thing, "routing.targets.#"),
					resource.TestCheckResourceAttr(thing, "limits.unit", "never"),
					resource.TestCheckResourceAttr(thing, "limits.enabled", "false"),
					resource.TestCheckNoResourceAttr(thing, "limits.window.days"),
				),
			},
			{Config: body, PlanOnly: true},
		},
	})
}

// The same defaults hold inside the items of a list, and an import reads the object with empty lists
// as the default.
func TestDefaultsInsideListItemsAndImport(t *testing.T) {
	body := config("  remotes = [{ name = \"a\" }, { name = \"b\", filters = { include = [\"x\"] } }]\n")
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: factories(),
		Steps: []resource.TestStep{
			{
				Config: body,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectKnownValue(thing, tfjsonpath.New("remotes").AtSliceIndex(0), knownvalue.ObjectExact(map[string]knownvalue.Check{
						"name": knownvalue.StringExact("a"), "overrides": knownvalue.Null(),
						"filters": knownvalue.ObjectExact(map[string]knownvalue.Check{"include": knownvalue.Null(), "exclude": knownvalue.Null()}),
					})),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr(thing, "remotes.0.filters.include.#"),
					resource.TestCheckNoResourceAttr(thing, "remotes.0.overrides.#"),
					resource.TestCheckResourceAttr(thing, "remotes.1.filters.include.#", "1"),
					resource.TestCheckNoResourceAttr(thing, "remotes.1.filters.exclude.#"),
				),
			},
			{Config: body, PlanOnly: true},
			{Config: body, ResourceName: thing, ImportState: true, ImportStateVerify: true},
		},
	})
}

// The default of an object inside the items of a set: an item that leaves the object out plans the
// object of nulls, next to an item that sets it.
func TestDefaultsInsideSetItems(t *testing.T) {
	for _, names := range [][2]string{{"a", "b"}, {"b", "a"}} {
		body := config("  groups = [{ name = \"" + names[0] + "\" }, { name = \"" + names[1] + "\", filters = { include = [\"x\"] } }]\n")
		t.Run(names[0]+names[1], func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				ProtoV6ProviderFactories: factories(),
				Steps: []resource.TestStep{
					{Config: body, Check: resource.TestCheckResourceAttr(thing, "groups.#", "2")},
					{Config: body, PlanOnly: true},
					{Config: config("  groups = [{ name = \"" + names[0] + "\" }]\n")},
					{Config: config("  groups = [{ name = \"" + names[0] + "\" }]\n"), PlanOnly: true},
				},
			})
		})
	}
}
