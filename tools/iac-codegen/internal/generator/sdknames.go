package generator

import (
	"fmt"
	"strings"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/coralogix-management-sdk/iac/sdkcheck"
)

// ruleProviderClientSet is the rule of the provider clientset names. The clientset is
// handwritten provider code, not SDK code.
const ruleProviderClientSet sdkcheck.Rule = "special: provider clientset"

// resolveSDKNames lists the SDK names that the generated code uses for the resource, then the
// names of the provider clientset that it calls. It only applies the naming rules. sdkIssues
// confirms that the names exist. An enum whose overrides state its values uses its zero value,
// so the SDK must have that constant too.
func resolveSDKNames(r *model.Resource, tag, module, providerModule string) ([]sdkcheck.Ref, error) {
	refs, err := sdkcheck.Names(r, tag, module, r.Policy.EnumOverride)
	if err != nil {
		return nil, err
	}
	return append(refs, providerClientSetRefs(tag, providerModule)...), nil
}

// dataSourceQueryIDRef names the optional ID setter on the Get request builder.
// The API contract check has established that this is an optional string query
// parameter named id; sdkcheck verifies that the pinned SDK exposes its setter.
func dataSourceQueryIDRef(refs []sdkcheck.Ref) (sdkcheck.Ref, error) {
	for _, ref := range refs {
		if ref.Path == "get" && ref.Kind == sdkcheck.KindType {
			return sdkcheck.Ref{
				Path:  "dataSource.get.query.id",
				Kind:  sdkcheck.KindMethod,
				Pkg:   ref.Pkg,
				Owner: ref.Name,
				Name:  goFieldName("id"),
				Want:  "func(id string) " + ref.Name,
				Rule:  sdkcheck.RuleParameter,
			}, nil
		}
	}
	return sdkcheck.Ref{}, fmt.Errorf("Get request builder type is missing")
}

// providerClientSetRefs are the names of the provider clientset. Its accessor name usually
// drops " Service" from the tag (F17); when it does not, the check finds the accessor by its
// type.
func providerClientSetRefs(tag, providerModule string) []sdkcheck.Ref {
	refs := []sdkcheck.Ref{
		{Path: "provider.clientset", Kind: sdkcheck.KindPackage, Name: "clientset"},
		{Path: "provider.clientset", Kind: sdkcheck.KindType, Name: "ClientSet"},
		{Path: "resource", Kind: sdkcheck.KindMethod, Owner: "ClientSet", Name: camelize(strings.TrimSuffix(tag, " Service")),
			Want: "func() *" + sdkcheck.PackageName(tag) + "." + sdkcheck.ClientType(tag), ByType: true},
	}
	for i := range refs {
		refs[i].Pkg, refs[i].Rule = providerModule+"/internal/clientset", ruleProviderClientSet
	}
	return refs
}

// goFieldName is the SDK struct field for a property name: "joinLimit" → "JoinLimit".
func goFieldName(property string) string { return camelize(property) }

// camelize splits s at each character that is not a letter or a digit, and
// joins the parts with the first letter in upper case. It keeps the case of
// the other letters: "AI Evaluations Service" → "AIEvaluationsService".
func camelize(s string) string {
	return model.GoName(s)
}

func lowerFirst(s string) string { return strings.ToLower(s[:1]) + s[1:] }
