package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// surveySpec has a clean resource A, a resource B with the common shapes that
// the generator does not support yet, a singleton S, and a read-only R.
const surveySpec = `openapi: 3.1.0
info: {title: t, version: "1"}
paths:
  /a:
    post:
      operationId: Svc_CreateA
      requestBody: {content: {application/json: {schema: {type: object, properties: {name: {type: string}}}}}}
      responses: {"200": {description: "", content: {application/json: {schema: {type: object, properties: {a: {$ref: '#/components/schemas/A'}}}}}}}
  /a/{id}:
    parameters: [{in: path, name: id, required: true, schema: {type: string}}]
    get:
      operationId: Svc_GetA
      responses: {"200": {description: "", content: {application/json: {schema: {type: object, properties: {a: {$ref: '#/components/schemas/A'}}}}}}}
    patch:
      operationId: Svc_UpdateA
      requestBody: {content: {application/json: {schema: {type: object, properties: {name: {type: string}}}}}}
      responses: {"200": {description: "", content: {application/json: {schema: {type: object, properties: {a: {$ref: '#/components/schemas/A'}}}}}}}
    delete:
      operationId: Svc_DeleteA
      responses: {"200": {description: ""}}
  /b:
    post:
      operationId: Svc_CreateB
      requestBody: {content: {application/json: {schema: {type: object, properties: {b: {$ref: '#/components/schemas/B'}}}}}}
      responses: {"200": {description: "", content: {application/json: {schema: {type: object, properties: {bId: {type: string}}}}}}}
  /b/{b_id}:
    parameters: [{in: path, name: b_id, required: true, schema: {type: string}}]
    get:
      operationId: Svc_GetB
      responses: {"200": {description: "", content: {application/json: {schema: {type: object, properties: {b: {$ref: '#/components/schemas/B'}, createdAt: {type: string}}}}}}}
    delete:
      operationId: Svc_DeleteB
      responses: {"200": {description: ""}}
  /s:
    get:
      operationId: Svc_GetS
      responses: {"200": {description: "", content: {application/json: {schema: {$ref: '#/components/schemas/S'}}}}}
    patch:
      operationId: Svc_UpdateS
      requestBody: {content: {application/json: {schema: {type: object, properties: {on: {type: boolean}}}}}}
      responses: {"200": {description: ""}}
  /r/{id}:
    get:
      operationId: Svc_GetR
      parameters: [{in: path, name: id, required: true, schema: {type: string}}]
      responses: {"200": {description: ""}}
components:
  schemas:
    A: {type: object, properties: {id: {type: string}, name: {type: string}}}
    B: {type: object, properties: {id: {type: string}}}
    S: {type: object, properties: {on: {type: boolean}}}
`

func TestResourceSurvey(t *testing.T) {
	p := filepath.Join(t.TempDir(), "spec.yaml")
	if err := os.WriteFile(p, []byte(surveySpec), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := runResourceSurvey(p, &out); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"1 read-only names",
		"3 writable resource names",
		"operations: Create, Get, Update, Delete; Get response: wraps the resource\n",
		"id: the id path parameter is not a resource field ({b_id})",
		"Create body: wraps the resource in a field (b)",
		"Get response: the resource and other fields beside it",
		"Create response: no resource",
		"operations: Create, Get, Delete (no Update: a change replaces the resource)",
		"operations: Get, Update only (a singleton or settings); id: Get has no path parameter; Get response: is the resource itself",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output has no %q:\n%s", want, got)
		}
	}
}
