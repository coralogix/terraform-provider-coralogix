package generator

import (
	"fmt"
	"slices"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// crudData is the template data for the resource: CRUD, import, and the
// provider data. Every SDK name comes from the checked sdkRefs.
type crudData struct {
	TypeName string // resource type name without the provider prefix
	Model    string // Terraform model struct of the resource
	IDAttr   string // Terraform attribute of the id
	IDField  string // SDK field of the id in the resource
	IDValue  bool   // the SDK id field is a value, not a pointer (F18)
	IDGoType string // string, int32, or int64
	IDTFType string // String, Int32, or Int64
	IDBits   int    // 0 for string; 32 or 64 for integer import parsing
	// Singleton: no id in the path (D18). The id attribute is the fixed value
	// TypeName, and the API calls take no id.
	Singleton bool
	// Replace: Update is a full replace (PUT, E11) with no update mask.
	Replace bool
	// UpdateIDInBody: the Update call takes no id. The id is a field of the body.
	UpdateIDInBody bool
	// Existing: the resource has users. The package exports Flatten for a handwritten data source.
	Existing bool
	// Upgrades are the state upgraders: one per older schema version.
	Upgrades []upgradeData
	// MaskInBody: updateMask is an optional JSON body property. Update sets it
	// to the same changed-field mask a query parameter would send.
	MaskInBody bool
	// UpdateMask is the SDK request-builder method for the PATCH updateMask
	// query parameter. It is empty for a full replace and for MaskInBody.
	UpdateMask string
	SDKName    string // package name of the resource SDK package
	Client     string // SDK client type
	Resource   string // SDK type of the resource
	// The provider clientset supplies provider data and the checked service
	// accessor. The SDK cxsdk package supplies API error helpers.
	ProviderPkg, ProviderName string
	CXPkg, CXName             string
	ClientSet, Accessor       string
	NewAPIError, APICode      string
	Create, Get               crudOp
	Update, Delete            crudOp
}

// upgradeData is a state upgrader. It reads the resource from the API, so the prior schema is
// only used to decode the old state.
type upgradeData struct {
	Version    int64
	ImportPath string // package of the function that returns the frozen prior schema
	Alias      string // import alias of that package
	Func       string
}

func sortedVersions(file *overrides.File) []int64 {
	versions := make([]int64, 0, len(file.Schema.Upgrade))
	for v := range file.Schema.Upgrade {
		versions = append(versions, v)
	}
	slices.Sort(versions)
	return versions
}

// crudOp is the SDK call of one operation:
//
//	client.<Method>(ctx[, id]).<Body>(body).Execute()
type crudOp struct {
	Method   string
	Body     string // request builder method that sets the body; "" when there is no body
	Response string // response field that holds the resource; "" when there is none, or the response is the resource
}

// buildCRUD maps the operations and their SDK names to the template data.
// Create, Get, and Update must return the resource. Delete must not have a
// body.
// buildCRUDWith is buildCRUD for a resource with a behavior-overrides file (nil for a new resource).
func buildCRUDWith(r *model.Resource, refs []sdkRef, file *overrides.File) (*crudData, error) {
	ix, err := indexRefs(refs)
	if err != nil {
		return nil, err
	}
	client, err := ix.typeRef("resource")
	if err != nil {
		return nil, err
	}
	resource, err := ix.typeRef("fields")
	if err != nil {
		return nil, err
	}
	out := &crudData{
		TypeName:   tfName(r.Name),
		Model:      modelTypeName(r.Name),
		IDAttr:     "id",
		IDGoType:   "string",
		IDTFType:   "String",
		Singleton:  r.Singleton,
		Replace:    r.Replace,
		MaskInBody: r.MaskInBody,

		UpdateIDInBody: r.Policy.UpdateIDInBody,
		Existing:       r.Policy.Existing,
		SDKName:        ix.pkg.Name,
		Client:         client.Name,
		Resource:       resource.Name,
	}
	if err := resourceIDData(r, ix, out); err != nil {
		return nil, err
	}
	if file != nil {
		for _, version := range sortedVersions(file) {
			importPath, function, _ := file.Schema.Upgrade[version].PriorSchemaFunc()
			out.Upgrades = append(out.Upgrades, upgradeData{Version: version, ImportPath: importPath, Alias: fmt.Sprintf("priorSchema%d", version), Func: function})
		}
	}
	if err := updateExtras(ix, r, out); err != nil {
		return nil, err
	}
	if err := providerNames(ix, client.Name, out); err != nil {
		return nil, err
	}
	if err := cxsdkNames(ix, out); err != nil {
		return nil, err
	}

	ops := []crudSpec{
		{"create", r.Create, &out.Create, true, true},
		{"get", r.Get, &out.Get, false, true},
		{"update", r.Update, &out.Update, true, true},
		{"delete", r.Delete, &out.Delete, false, false},
	}
	for _, o := range ops {
		if *o.out, err = buildCRUDOp(ix, o, client.Name, resource.Name); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func resourceIDData(r *model.Resource, ix *refIndex, out *crudData) error {
	if r.Singleton {
		return nil
	}
	idGoType, err := valueType(r.IDType)
	if err != nil {
		return fmt.Errorf("resource id: %w", err)
	}
	switch idGoType {
	case "string":
		out.IDTFType = "String"
	case "int32":
		out.IDTFType, out.IDBits = "Int32", 32
	case "int64":
		out.IDTFType, out.IDBits = "Int64", 64
	default:
		return fmt.Errorf("resource id: Go type %s is not supported", idGoType)
	}
	out.IDGoType = idGoType
	id, err := ix.fieldRef("fields." + r.IDParam)
	if err != nil {
		return err
	}
	if id.Want != "*"+idGoType && id.Want != idGoType {
		return fmt.Errorf("SDK field %s has type %s, the id needs *%s or %s", id.sdkName(), id.Want, idGoType, idGoType)
	}
	out.IDAttr, out.IDField, out.IDValue = tfName(r.IDParam), id.Name, id.Want == idGoType
	return nil
}

func updateExtras(ix *refIndex, r *model.Resource, out *crudData) error {
	if r.Replace || r.MaskInBody {
		return nil
	}
	builder, err := ix.typeRef("update")
	if err != nil {
		return err
	}
	mask, err := ix.methodRef("update.mask", builder.Name)
	if err != nil {
		return err
	}
	out.UpdateMask = mask.Name
	return nil
}

// crudSpec is one operation of the resource, and what buildCRUD requires of it.
type crudSpec struct {
	name     string
	op       model.Operation
	out      *crudOp
	body     bool // the operation must have a request body
	resource bool // the response must hold the resource
}

// buildCRUDOp returns the SDK names of one operation. client is the SDK client
// type, and resource is the SDK type of the resource.
func buildCRUDOp(ix *refIndex, o crudSpec, client, resource string) (crudOp, error) {
	var out crudOp
	if (o.op.Body != "") != o.body {
		return out, fmt.Errorf("%s: request body %q is not supported", o.name, o.op.Body)
	}
	if (o.op.Response.Field != "" || o.op.Response.Direct) != o.resource {
		return out, fmt.Errorf("%s: response field %q is not supported", o.name, o.op.Response.Field)
	}
	method, err := ix.methodRef(o.name, client)
	if err != nil {
		return out, err
	}
	out.Method = method.Name
	if o.body {
		builder, err := ix.typeRef(o.name)
		if err != nil {
			return out, err
		}
		body, err := ix.methodRef(o.name+".body", builder.Name)
		if err != nil {
			return out, err
		}
		out.Body = body.Name
	}
	if o.resource && !o.op.Response.Direct {
		field, err := ix.fieldRef(o.name + ".response." + o.op.Response.Field)
		if err != nil {
			return out, err
		}
		if field.Want != "*"+resource {
			return out, fmt.Errorf("SDK field %s has type %s, want *%s", field.sdkName(), field.Want, resource)
		}
		out.Response = field.Name
	}
	return out, nil
}

// providerNames sets the provider clientset and accessor names (F17).
func providerNames(ix *refIndex, client string, out *crudData) error {
	pkg, err := ix.pkgRef("provider.clientset")
	if err != nil {
		return err
	}
	clientSet, err := ix.typeRef("provider.clientset")
	if err != nil {
		return err
	}
	accessor, err := ix.methodRef("resource", clientSet.Name)
	if err != nil {
		return err
	}
	if want := "func() *" + ix.pkg.Name + "." + client; accessor.Want != want {
		return fmt.Errorf("SDK method %s has type %s, want %s", accessor.sdkName(), accessor.Want, want)
	}
	out.ProviderPkg, out.ProviderName = pkg.Pkg, pkg.Name
	out.ClientSet, out.Accessor = clientSet.Name, accessor.Name
	return nil
}

// cxsdkNames sets the names of the SDK API error helpers.
func cxsdkNames(ix *refIndex, out *crudData) error {
	pkg, err := ix.pkgRef("cxsdk")
	if err != nil {
		return err
	}
	newAPIError, err := ix.funcRef("cxsdk.errors", "NewAPIError")
	if err != nil {
		return err
	}
	code, err := ix.funcRef("cxsdk.errors", "Code")
	if err != nil {
		return err
	}
	out.CXPkg, out.CXName = pkg.Pkg, pkg.Name
	out.NewAPIError, out.APICode = newAPIError.Name, code.Name
	return nil
}
