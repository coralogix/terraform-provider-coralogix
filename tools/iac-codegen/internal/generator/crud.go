package generator

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/coralogix-management-sdk/iac/sdkcheck"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// updateOnlyAttr is an attribute that Create cannot send, with the Go expression of its
// declared server default.
type updateOnlyAttr struct{ Attr, Default string }

type keptExtra struct {
	TFName string // Terraform attribute in the plan or prior state
	Model  string // Go field in the resource model
	Type   string // Framework type of the value, for example types.Dynamic
	// AttrType is the attr.Type of an object. A prior or plan value is kept only when it has this
	// type and no unknown value inside: a prior schema can hold an object of another shape.
	AttrType string
}

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
	// IDWrap names the wrap helpers of an id that the API holds in a one-property object, for
	// example {"thingId": {"id": 1}}: wrap<IDWrap> and unwrap<IDWrap>. "" for an id that is the
	// SDK field itself.
	IDWrap string
	// IDAsString: the integer id has the released Terraform type String (terraformType: String).
	// The state holds its decimal text.
	IDAsString bool
	// Singleton: no id in the path (D18). The id attribute is the fixed value
	// SingletonID, and the API calls take no id.
	Singleton bool
	// SingletonID: the id attribute is the id field of the API resource, not the fixed value.
	SingletonID bool
	// FixedID is the Go expression of the fixed id of a singleton without singletonID.fromAPI:
	// singletonID.fixed of the behavior-overrides file, or TypeName.
	FixedID string
	// Upsert: Create and Update are one Set call, and Delete makes no call. The API cannot remove
	// the resource.
	Upsert bool
	// Overwrite: the resource is the list that one write replaces (api.overwrite). Create and
	// Update send the write and read the resource with Get. Delete sends the write with every
	// list empty.
	Overwrite bool
	// GetHooks: the Hooks have Get and AfterGet (getHooks of the behavior-overrides file).
	// GetResponse is the SDK type that the Get call returns.
	GetHooks    bool
	GetResponse string
	// ReadAfterWrite: Create and Update read the resource with Get after a write whose response
	// holds the resource (readAfterWrite of the behavior-overrides file). It needs GetHooks.
	ReadAfterWrite bool
	// GetRequest is the SDK type of the Get request builder, which the Get hook receives a factory of.
	GetRequest string
	// GetQuery are the query values that Get always sends: the request builder method and the Go
	// expression of the value.
	GetQuery []crudQuery
	// GetNextPage is the request builder method that sets the page token of the next page, and
	// GetNextPageField is the response field that holds it. Only a paged GetFromList has them.
	GetNextPage      string
	GetNextPageField string
	// GetNextPageValue is true when the response field holds the token by value, not pointer.
	GetNextPageValue bool
	// GetNextPageInt is true when the token is an integer: the loop compares it with 0, and
	// the builder method takes the number, not a string.
	GetNextPageInt bool
	// Arms are the Go fields of the inlined oneOf arms of the resource. Read treats a response
	// without one as a resource that does not exist.
	Arms []string
	// RootUnsupported are the Go fields of the oneOf arms of the resource that it does not
	// configure. A response that selects one exists: flatten reports it.
	RootUnsupported []string
	// Replace: Update is a full replace (PUT, E11) with no update mask.
	Replace bool
	// UpdateIDInBody: the Update call takes no id. The id is a field of the body.
	UpdateIDInBody bool
	// UpdateBodyID is the SDK path of the body id, for example "Thing.Id", when only the server
	// sets the id, so no Update field sends it. Update copies the state id there. "" otherwise.
	UpdateBodyID string
	// UpdateBodyIDValue: that body field is a value, not a pointer.
	UpdateBodyIDValue bool
	// Existing: the resource has users. The package exports Flatten for a handwritten data source.
	Existing bool
	// KeptExtras are the dynamic extra attributes and the configurable string extra attributes of
	// the resource. The API holds no value for them, so setState starts with the plan or prior
	// state before AfterFlatten can change it.
	KeptExtras []keptExtra
	// KeptObjects: a kept extra attribute is an object, so resource.go names attr.Type.
	KeptObjects bool
	// UpdateOnly are the attributes that Create cannot send. Create follows with an Update
	// when the configuration sets another value than the one Create stored, or omits the
	// attribute and Create stored another value than the declared server default.
	UpdateOnly []updateOnlyAttr
	// Upgrades are the state upgraders: one per older schema version.
	Upgrades []upgradeData
	// MaskInBody: updateMask is an optional JSON body property. Update sets it
	// to the same changed-field mask a query parameter would send.
	MaskInBody bool
	// OmitMask: Update sends no body mask (updateMask: omit). The server updates every field
	// present in the body.
	OmitMask bool
	// UpdateMask is the SDK request-builder method for the PATCH updateMask
	// query parameter. It is empty for a full replace, for MaskInBody, and for an Update by POST,
	// which has no mask.
	UpdateMask string
	SDKName    string // package name of the resource SDK package
	Client     string // SDK client type
	Resource   string // SDK type of the resource
	// The provider clientset supplies provider data and the checked service
	// accessor. The SDK cxsdk package supplies API error helpers.
	ProviderPkg, ProviderName string
	CXPkg, CXName             string
	ClientSet, Accessor       string
	NewAPIError, APINotFound  string
	Create, Get               crudOp
	Update, Delete            crudOp
}

// crudQuery is one SDK request-builder setter: the method name, the Go expression of its
// value, and, for a pointer field that takes a typed variable, the Go type of that variable.
type crudQuery struct {
	Method, Value string
	Type          string
}

// crudResult is the batch Create answer (api.batchWrite.createResult): Create sends one item
// and reads the first result. The item is created only when Status holds one of the Created
// constants, and Resource then holds it. Otherwise Message, when the result has it, says why.
type crudResult struct {
	Items       string   // response field that holds the results
	Resource    string   // result field that holds the resource
	Status      string   // result field that holds the status enum
	CreatedVals []string // enum constants that mean created
	Message     string   // result field with the message; "" when the line names none
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
	IDs string // SDK query setter of a batch Get that returns a resource array
	// FromList: the Get is the List of the resource (api.getFromList). It takes no id, and the
	// generated call selects the item whose id is the id of the resource. Response is the field
	// that holds the items.
	FromList bool
	Method   string
	Body     string // request builder method that sets the body; "" when there is no body
	// DeleteBodyType is the SDK type of the Delete request body, when the remove sends one
	// (api.delete.ids). "" when Delete has no body.
	DeleteBodyType string
	// DeleteIDs is the SDK field of the id array in the Delete body (api.delete.ids).
	DeleteIDs string
	// DeleteValues are the fixed properties that Delete sends beside the ids: the body field and
	// the Go expression of each value.
	DeleteValues []crudQuery
	// Result are the names of the batch Create answer (api.batchWrite.createResult): the
	// response items field, the result fields, and the enum constants that mean created.
	Result   *crudResult
	Response string // response field that holds the resource; "" when there is none, or the response is the resource
	// ResponseValue: the response field is a value, not a pointer. openapi-generator uses a value
	// for a required field.
	ResponseValue bool
	// UpdateItems: the Update answers the array of resources it wrote (api.batchWrite). The
	// generated Update selects the item whose id is the id of the resource from the array.
	UpdateItems bool
	// ResponseID is the response field that holds only the id of the resource: a Create response
	// that does not return the resource. The generated call then reads the resource with Get.
	ResponseID string
	// ResponseIDValue: the id field is a value, not a pointer.
	ResponseIDValue bool
	// ResponseIDWrap: the id field is the wrapper of the id, as the id field of the resource is.
	ResponseIDWrap bool
	// ResponseIDZero is the Go zero value of the id field: the id that the API never gives to a
	// resource, so a response without an id is an error. "\"\"" for a string, "0" for an integer.
	ResponseIDZero string
	// Empty: the response has no fields, so the generated call reads the resource with Get.
	Empty bool
	// Copy are the Go fields that the write of an overwrite resource returns
	// (api.overwrite.writeReturnsResource). The generated call copies them into the resource
	// instead of reading it with Get.
	Copy []string
}

// Items reports whether the call answers with an array of resources, from which the resource is
// selected: a batch Get by ids, or the List of a getFromList resource. No item means that the
// resource does not exist.

func (o crudOp) Items() bool { return o.IDs != "" || o.FromList }

// buildCRUDWith is buildCRUD for a resource with a behavior-overrides file (nil for a new resource).
func buildCRUDWith(r *model.Resource, refs []sdkcheck.Ref, file *overrides.File) (*crudData, error) {
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
		TypeName:    tfName(r.Name),
		Model:       modelTypeName(r.Name),
		IDAttr:      "id",
		IDGoType:    "string",
		IDTFType:    "String",
		Singleton:   r.Singleton,
		Upsert:      file.Upserts(),
		Overwrite:   r.Overwrite,
		SingletonID: file.IDFromAPI(),
		Replace:     r.Replace,
		MaskInBody:  r.MaskInBody,
		OmitMask:    file != nil && file.UpdateMask == overrides.UpdateMaskOmit,

		UpdateIDInBody: r.Policy.UpdateIDInBody,
		Existing:       r.Policy.Existing,
		SDKName:        ix.pkg.Name,
		Client:         client.Name,
		Resource:       resource.Name,
	}
	if file != nil && file.TypeName != "" {
		out.TypeName = file.TypeName
	}
	if err := updateOnlyAttrs(r, out); err != nil {
		return nil, err
	}
	if err := resourceIDData(r, ix, file, out); err != nil {
		return nil, err
	}
	if err := updateIDFromState(r, ix, out); err != nil {
		return nil, err
	}
	out.FixedID = singletonID(file)
	if file != nil {
		for _, arm := range file.Types[r.Name].UnsupportedArms {
			ref, err := ix.fieldRef("fields." + arm)
			if err != nil {
				return nil, err
			}
			out.RootUnsupported = append(out.RootUnsupported, ref.Name)
		}
	}
	for _, parent := range inlinedParents(r, file) {
		if parent.Arm {
			ref, err := ix.fieldRef("fields." + parent.Field)
			if err != nil {
				return nil, err
			}
			out.Arms = append(out.Arms, ref.Name)
		}
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

	if err := getCall(ix, r, file, out); err != nil {
		return nil, err
	}
	ops := []crudSpec{
		{"create", r.Create, &out.Create, true, true},
		{"get", r.Get, &out.Get, false, true},
		{"update", r.Update, &out.Update, true, true},
		// The Delete of an overwrite resource is the write, with every list empty.
		// A remove through a body (api.delete.ids) sends the id array beside the
		// fixed values, so Delete has a body then.
		{"delete", r.Delete, &out.Delete, r.Overwrite || r.Policy.DeleteIDs != "", false},
	}
	for _, o := range ops {
		if o.name == "delete" && file.Upserts() {
			continue // the API cannot remove an upsert, so Delete makes no call
		}
		if *o.out, err = buildCRUDOp(ix, o, client.Name, resource.Name, r, r.IDType != nil && len(r.IDType.Wrappers) == 1); err != nil {
			return nil, err
		}
	}
	if err := out.checkReadAfterWrite(); err != nil {
		return nil, err
	}
	return out, nil
}

// singletonID is the Go expression of the fixed id of a singleton: singletonID.fixed of the
// behavior-overrides file, or TypeName. The check that the resource is a singleton is in tfschema.go.
func singletonID(file *overrides.File) string {
	if fixed := file.FixedID(); fixed != "" {
		return fmt.Sprintf("%q", fixed)
	}
	return "TypeName"
}

func resourceIDData(r *model.Resource, ix *refIndex, file *overrides.File, out *crudData) error {
	if r.Singleton {
		return nil
	}
	idGoType, err := sdkcheck.ValueType(r.IDType)
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
	id, err := ix.fieldRef("fields." + r.ID())
	if err != nil {
		return err
	}
	// The SDK field holds the id, or the wrapper of the id. The id conversion makes the wrap
	// helpers of that wrapper.
	want := idGoType
	switch len(r.IDType.Wrappers) {
	case 0:
	case 1:
		w := r.IDType.Wrappers[0]
		wrapper, err := ix.schemaRef(w.Schema)
		if err != nil {
			return err
		}
		want, out.IDWrap = wrapper.Name, wrapHelperName([]string{camelize(w.Schema)})
	default:
		return fmt.Errorf("resource id: %d wrappers, want at most 1", len(r.IDType.Wrappers))
	}
	if id.Want != "*"+want && id.Want != want {
		return fmt.Errorf("SDK field %s has type %s, the id needs *%s or %s", id.SDKName(), id.Want, want, want)
	}
	out.IDAttr, out.IDField, out.IDValue = terraformName(r.Policy, r.Name, r.ID()), id.Name, id.Want == want
	if err := idAsString(r, file, out); err != nil {
		return err
	}
	return nil
}

// idAsString keeps the released decimal string representation of an int64 resource id.
func idAsString(r *model.Resource, file *overrides.File, out *crudData) error {
	if file == nil || file.Types[r.Name].Fields[r.ID()].TerraformType != "String" {
		return nil
	}
	if out.IDGoType != "int64" {
		return fmt.Errorf("%s.%s: terraformType String needs an int64 id, not %s", r.Name, r.ID(), out.IDGoType)
	}
	out.IDTFType, out.IDAsString = "String", true
	return nil
}

// updateIDFromState decides how an Update with the id in its body sends the id. A client-set
// id is an Update field. A server-set id is not, so Update copies the state id into the body:
// into the resource that the body wraps, whose id field resourceIDData checked, or into the
// id field of the body itself.
func updateIDFromState(r *model.Resource, ix *refIndex, out *crudData) error {
	if sdkcheck.ServerSetBodyID(r) == nil {
		return nil
	}
	if wrapper := r.Policy.RequestWrapper; wrapper != "" && !r.UpdateIDOutsideWrapper {
		out.UpdateBodyID, out.UpdateBodyIDValue = goFieldName(wrapper)+"."+out.IDField, out.IDValue
		return nil
	}
	id, err := ix.fieldRef("update.body." + r.ID())
	if err != nil {
		return err
	}
	out.UpdateBodyID, out.UpdateBodyIDValue = id.Name, id.Want == out.IDGoType
	return nil
}

func updateExtras(ix *refIndex, r *model.Resource, out *crudData) error {
	if r.Replace || r.MaskInBody || r.UpdateMask == "" {
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
// type, and resource is the SDK type of the resource. idWrapped is true when the resource id is
// inside a wrapper: the model then requires a Create response id to hold the same wrapper.
func buildCRUDOp(ix *refIndex, o crudSpec, client, resource string, r *model.Resource, idWrapped bool) (crudOp, error) {
	var out crudOp
	overwrite := r.Overwrite
	if (o.op.Body != "") != o.body {
		return out, fmt.Errorf("%s: request body %q is not supported", o.name, o.op.Body)
	}
	// The write of an overwrite resource returns what it created in its own shape, so Create and
	// Update read the resource with Get. A batch Create (api.batchWrite.createResult) reads its
	// first result instead of the resource.
	writes := overwrite && (o.name == "create" || o.name == "update")
	readsBack := o.op.Response.ID != "" || o.op.Response.Result != nil || (o.name == "update" || o.name == "create") && o.op.Response.Empty || writes
	if (o.op.Response.Field != "" || o.op.Response.Direct) != o.resource && !readsBack {
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
		if o.name == "delete" && r.Policy.DeleteIDs != "" {
			// A remove through a body sends the id array beside the fixed values
			// (api.delete.ids, api.delete.values). The sdkcheck refs register them on the
			// Delete body type.
			bodyType, err := ix.typeRef("delete.body")
			if err != nil {
				return out, err
			}
			out.DeleteBodyType = bodyType.Name
			ids, err := ix.fieldRef("delete.body." + r.Policy.DeleteIDs)
			if err != nil {
				return out, err
			}
			// The generated Delete writes the id into a string array; a body field of any
			// other shape fails at provider build instead of with a generator diagnostic.
			if ids.Want != "[]string" {
				return out, fmt.Errorf("delete.body.%s: the SDK field is %s, want []string", r.Policy.DeleteIDs, ids.Want)
			}
			out.DeleteIDs = ids.Name
			for _, v := range r.DeleteValues {
				f, err := ix.fieldRef("delete.body." + v.Name)
				if err != nil {
					return out, err
				}
				value := fmt.Sprintf("%q", v.Value)
				if v.Type != nil {
					switch v.Type.Kind {
					case model.Enum:
						c, ok := ix.consts["delete.body."+v.Name+"."+v.Value]
						if !ok {
							return out, fmt.Errorf("delete.body.%s: no SDK constant for %s", v.Name, v.Value)
						}
						value = ix.pkg.Name + "." + c.Name
					case model.Bool:
						value = v.Value
					case model.Integer, model.Number:
						value = v.Value
					}
				}
				// A pointer field takes the address of a variable of the field's type, so the
				// template declares the value with the type of the SDK field. A named SDK type
				// renders with its package; a builtin renders bare.
				setter := crudQuery{Method: f.Name, Value: value}
				if strings.HasPrefix(f.Want, "*") {
					setter.Type = f.Want[1:]
					if !slices.Contains([]string{"string", "bool", "int", "int8", "int16", "int32", "int64", "uint", "uint32", "uint64", "float32", "float64", "byte", "rune"}, setter.Type) {
						setter.Type = ix.pkg.Name + "." + setter.Type
					}
				}
				out.DeleteValues = append(out.DeleteValues, setter)
			}
		}
	}
	if o.op.Response.ID != "" && idWrapped {
		// The response holds the wrapper of the id, as the resource does. The wrap helpers of the
		// resource id read it.
		field, err := ix.fieldRef(o.name + ".response." + o.op.Response.ID)
		if err != nil {
			return out, err
		}
		out.ResponseID, out.ResponseIDValue, out.ResponseIDWrap = field.Name, !strings.HasPrefix(field.Want, "*"), true
		return out, nil
	}
	if o.op.Response.ID != "" {
		field, err := ix.fieldRef(o.name + ".response." + o.op.Response.ID)
		if err != nil {
			return out, err
		}
		// The id of the response is the id of the resource: a string, or the same integer.
		goType, zero := "string", `""`
		if r.IDType.Kind == model.Integer {
			goType, zero = r.IDType.Format, "0"
		}
		if field.Want != "*"+goType && field.Want != goType {
			return out, fmt.Errorf("SDK field %s has type %s, want *%s", field.SDKName(), field.Want, goType)
		}
		out.ResponseID, out.ResponseIDValue, out.ResponseIDZero = field.Name, field.Want == goType, zero
		return out, nil
	}
	if (o.name == "update" || o.name == "create") && o.op.Response.Empty || writes {
		out.Empty = true
		if writes && r.WriteReturnsResource {
			for _, f := range r.Fields {
				if !f.InGet {
					continue
				}
				field, err := ix.fieldRef(o.name + ".response." + f.Name)
				if err != nil {
					return out, err
				}
				out.Copy = append(out.Copy, field.Name)
			}
		}
		return out, nil
	}
	if o.op.Response.Result != nil {
		// A batch Create (api.batchWrite.createResult) answers with per-item results, not the
		// resource. This must run before the resource-response branch: a result response has an
		// empty Field, which that branch would reject.
		result, err := buildCRUDResult(ix, o, o.op.Response.Result)
		if err != nil {
			return out, err
		}
		out.Result = result
		return out, nil
	}
	if o.resource && !o.op.Response.Direct {
		field, err := ix.fieldRef(o.name + ".response." + o.op.Response.Field)
		if err != nil {
			return out, err
		}
		if o.op.Response.Items {
			if field.Want != "[]"+resource {
				return out, fmt.Errorf("batch response needs []%s", resource)
			}
			if o.name == "get" {
				if r.GetFromList {
					// The Get is the List (api.getFromList). A List that pages its result is
					// supported: the generated Get follows the next page token.
					out.Response, out.FromList = field.Name, true
					return out, nil
				}
				builder, err := ix.typeRef("get")
				if err != nil {
					return out, err
				}
				setter, err := ix.methodRef("get.ids", builder.Name)
				if err != nil {
					return out, err
				}
				out.Response, out.IDs = field.Name, setter.Name
				return out, nil
			}
			if o.name == "update" {
				// The Update answers the array of users it wrote (api.batchWrite). The request
				// carries the id (updateIDInBody), so the generated Update selects the item
				// whose id matches the state. No createResult: there is no created/failed
				// distinction on an update of existing users.
				out.Response, out.ResponseValue, out.UpdateItems = field.Name, true, true
				return out, nil
			}
			// Create answers with the array of resources it wrote. Without
			// api.batchWrite.createResult the generated call could not tell the created item
			// from the failed ones, so the shape is refused.
			return out, fmt.Errorf("the %s response is a batch write array; name what Create returns with api.batchWrite.createResult", o.name)
		}
		if field.Want != "*"+resource && field.Want != resource {
			return out, fmt.Errorf("SDK field %s has type %s, want *%s", field.SDKName(), field.Want, resource)
		}
		out.Response, out.ResponseValue = field.Name, field.Want == resource
	}
	return out, nil
}

// buildCRUDResult resolves the names of the batch Create answer: the response items field, the
// result fields, and the enum constants that mean created (sdknames.go writeResult).
func buildCRUDResult(ix *refIndex, o crudSpec, result *model.WriteResult) (*crudResult, error) {
	items, err := ix.fieldRef(o.name + ".response." + result.Items)
	if err != nil {
		return nil, err
	}
	resourceField, err := ix.fieldRef(o.name + ".result." + result.Resource)
	if err != nil {
		return nil, err
	}
	status, err := ix.fieldRef(o.name + ".result." + result.Status)
	if err != nil {
		return nil, err
	}
	created := make([]string, 0, len(result.Created))
	for _, v := range result.Created {
		c, ok := ix.consts[o.name+".result."+result.Status+"."+v]
		if !ok {
			return nil, fmt.Errorf("%s.result.%s: no SDK constant for %s", o.name, result.Status, v)
		}
		created = append(created, ix.pkg.Name+"."+c.Name)
	}
	out := &crudResult{
		Items:       items.Name,
		Resource:    resourceField.Name,
		Status:      status.Name,
		CreatedVals: created,
	}
	// The template derefs the status and message fields, so they must be pointers in the SDK.
	if !strings.HasPrefix(status.Want, "*") {
		return nil, fmt.Errorf("%s.result.%s: the SDK field is a value (%s), want a pointer the template can check", o.name, result.Status, status.Want)
	}
	if result.Message != "" {
		message, err := ix.fieldRef(o.name + ".result." + result.Message)
		if err != nil {
			return nil, err
		}
		if !strings.HasPrefix(message.Want, "*") {
			return nil, fmt.Errorf("%s.result.%s: the SDK field is a value (%s), want a pointer the template can nil-check", o.name, result.Message, message.Want)
		}
		out.Message = message.Name
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
		return fmt.Errorf("SDK method %s has type %s, want %s", accessor.SDKName(), accessor.Want, want)
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
	// IsNotFound tells a missing resource from a 404 of a route that does not
	// exist, for example in a region without the service. Only the first one
	// removes the resource from state.
	notFound, err := ix.funcRef("cxsdk.errors", "IsNotFound")
	if err != nil {
		return err
	}
	out.CXPkg, out.CXName = pkg.Pkg, pkg.Name
	out.NewAPIError, out.APINotFound = newAPIError.Name, notFound.Name
	return nil
}

// updateOnlyAttrs lists the fields that only Update takes, with the server default that create stores.
func updateOnlyAttrs(r *model.Resource, out *crudData) error {
	for _, f := range r.Fields {
		if !updateOnly(f) {
			continue
		}
		value, err := serverDefaultValue(f.Type, *declaredDefault(f))
		if err != nil {
			return fmt.Errorf("%s: server default: %w", f.Name, err)
		}
		out.UpdateOnly = append(out.UpdateOnly, updateOnlyAttr{Attr: terraformName(r.Policy, r.Name, f.Name), Default: value})
	}
	return nil
}

// getCall sets what the Get call needs besides the id: the query values that it always sends, and
// the SDK type that it returns, for the Get and AfterGet hooks. A paged GetFromList also gets the
// page setters and the next page token.
func getCall(ix *refIndex, r *model.Resource, file *overrides.File, out *crudData) error {
	if err := getQuery(ix, r, out); err != nil {
		return err
	}
	if err := getPage(ix, r, out); err != nil {
		return err
	}
	return getHooks(ix, r, file, out)
}

// normalizePageName is the SDK's name comparison for paging names: lower case without "_" and
// "-", so page_token and pageToken are both "pagetoken".
func normalizePageName(name string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(name))
}

// getPage resolves the paging of a GetFromList Get: the request builder method of the page-token
// parameter and the response field that holds the next token.
func getPage(ix *refIndex, r *model.Resource, out *crudData) error {
	if !r.GetFromList {
		return nil
	}
	if r.NextPage == nil {
		// A List with paging parameters but no next-page property that the model reads would
		// silently drop resources beyond the first page.
		if len(r.PageParameters) > 0 {
			return fmt.Errorf("get.page: the List pages with %d parameters but the model cannot read where the pages end", len(r.PageParameters))
		}
		return nil
	}
	builder, err := ix.typeRef("get")
	if err != nil {
		return err
	}
	// The token is the paging query parameter whose normal name matches the next page response
	// property (next_page_token), or a plain page_token parameter. Size/offset parameters page
	// the List but do not carry the next-page token.
	token := ""
	for _, p := range r.PageParameters {
		switch normalizePageName(p.Name) {
		case "nextpagetoken", "pagetoken":
			token = p.Name
		}
	}
	if token == "" {
		return fmt.Errorf("get.page: the List has a next page property %q but no page-token query parameter", r.NextPage.Name)
	}
	next, err := ix.methodRef("get.page."+token, builder.Name)
	if err != nil {
		return err
	}
	field, err := ix.fieldRef("get.response." + r.NextPage.Name)
	if err != nil {
		return err
	}
	// The generated loop compares the token with its zero value and sends it to the builder
	// method. A string token compares with ""; an integer token (the users API pages with an
	// int) compares with 0. Any other type fails with a generator diagnostic.
	want := strings.TrimPrefix(field.Want, "*")
	switch want {
	case "string", "int", "int32", "int64", "uint32", "uint64":
	default:
		return fmt.Errorf("get.response.%s: the SDK field is %s, want a string, an integer, or their pointer as the next page token", r.NextPage.Name, field.Want)
	}
	out.GetNextPage, out.GetNextPageField = next.Name, field.Name
	out.GetNextPageValue = !strings.HasPrefix(field.Want, "*")
	out.GetNextPageInt = want != "string"
	return nil
}

// getHooks names the SDK type that the Get call returns, for the Get and AfterGet hooks.
func getHooks(ix *refIndex, r *model.Resource, file *overrides.File, out *crudData) error {
	if file != nil && file.ReadAfterWrite && !file.GetHooks {
		return errors.New("readAfterWrite needs getHooks: the values that it reads back reach the state through the AfterGet hook")
	}
	if file == nil || !file.GetHooks {
		return nil
	}
	if r.Singleton {
		return errors.New("getHooks is for a resource whose Get call takes an id: a singleton has none")
	}
	response, err := ix.typeRef("get.response")
	if err != nil {
		return err
	}
	request, err := ix.typeRef("get")
	if err != nil {
		return err
	}
	out.GetHooks, out.GetResponse, out.GetRequest = true, response.Name, request.Name
	out.ReadAfterWrite = file.ReadAfterWrite
	return nil
}

// createReadsBack reports whether the generated Create already reads the resource with Get: its
// response holds only the id, has no fields, or the resource is an overwrite list.
func (c *crudData) createReadsBack() bool {
	return c.Overwrite || c.Create.Empty || c.Create.ResponseID != "" || len(c.Create.Copy) > 0
}

// updateReadsBack reports whether the generated Update already reads the resource with Get.
func (c *crudData) updateReadsBack() bool {
	return c.Overwrite || c.Update.Empty || len(c.Update.Copy) > 0
}

// checkReadAfterWrite stops a readAfterWrite line that changes nothing: both writes already read the
// resource back. A line that matches nothing hides that the behavior comes from somewhere else.
func (c *crudData) checkReadAfterWrite() error {
	if c.ReadAfterWrite && c.createReadsBack() && c.updateReadsBack() {
		return errors.New("readAfterWrite changes nothing: Create and Update already read the resource with Get. Delete the line")
	}
	return nil
}

// getQuery sets the query values that Get always sends: the request builder method of each, and an
// enum constant or a Go literal for its value.
func getQuery(ix *refIndex, r *model.Resource, out *crudData) error {
	if len(r.GetQuery) == 0 {
		return nil
	}
	builder, err := ix.typeRef("get")
	if err != nil {
		return err
	}
	for _, q := range r.GetQuery {
		path := "get.query." + q.Name
		method, err := ix.methodRef(path, builder.Name)
		if err != nil {
			return err
		}
		value := fmt.Sprintf("%q", q.Value)
		switch q.Type.Kind {
		case model.Enum:
			c, ok := ix.consts[path+"."+q.Value]
			if !ok {
				return fmt.Errorf("%s: no SDK constant for %s", path, q.Value)
			}
			value = ix.pkg.Name + "." + c.Name
		case model.Bool, model.Integer, model.Number:
			value = q.Value
		}
		out.GetQuery = append(out.GetQuery, crudQuery{Method: method.Name, Value: value})
	}
	return nil
}
