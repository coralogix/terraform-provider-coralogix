package main

import (
	"fmt"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// crudData is the template data for the resource: CRUD, import, and the
// provider data. Every SDK name comes from the checked sdkRefs.
type crudData struct {
	TypeName string // resource type name without the provider prefix
	Model    string // Terraform model struct of the resource
	IDAttr   string // Terraform attribute of the id
	IDField  string // SDK field of the id in the resource
	IDValue  bool   // the id field is a string, not a *string (F18)
	// Singleton: no id in the path (D18). The id attribute is the fixed value
	// TypeName, and the API calls take no id.
	Singleton bool
	SDKName   string // package name of the resource SDK package
	Client    string // SDK client type
	Resource  string // SDK type of the resource
	// The cxsdk package: the provider data type, the accessor of the client,
	// and the error helpers.
	CXPkg, CXName        string
	ClientSet, Accessor  string
	NewAPIError, APICode string
	Create, Get          crudOp
	Update, Delete       crudOp
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
func buildCRUD(r *model.Resource, refs []sdkRef) (*crudData, error) {
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
		TypeName:  tfName(r.Name),
		Model:     modelTypeName(r.Name),
		IDAttr:    "id",
		Singleton: r.Singleton,
		SDKName:   ix.pkg.Name,
		Client:    client.Name,
		Resource:  resource.Name,
	}
	if !r.Singleton {
		id, err := ix.fieldRef("fields." + r.IDParam)
		if err != nil {
			return nil, err
		}
		if id.Want != "*string" && id.Want != "string" {
			return nil, fmt.Errorf("SDK field %s has type %s, the id needs *string or string", id.sdkName(), id.Want)
		}
		out.IDAttr, out.IDField, out.IDValue = tfName(r.IDParam), id.Name, id.Want == "string"
	}
	if err := cxsdkNames(ix, client.Name, out); err != nil {
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

// cxsdkNames sets the names from the handwritten cxsdk package (F17).
func cxsdkNames(ix *refIndex, client string, out *crudData) error {
	pkg, err := ix.pkgRef("cxsdk")
	if err != nil {
		return err
	}
	clientSet, err := ix.typeRef("cxsdk")
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
	newAPIError, err := ix.funcRef("cxsdk.errors", "NewAPIError")
	if err != nil {
		return err
	}
	code, err := ix.funcRef("cxsdk.errors", "Code")
	if err != nil {
		return err
	}
	out.CXPkg, out.CXName = pkg.Pkg, pkg.Name
	out.ClientSet, out.Accessor = clientSet.Name, accessor.Name
	out.NewAPIError, out.APICode = newAPIError.Name, code.Name
	return nil
}
