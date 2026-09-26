// Package specload_test checks that an OpenAPI library can read the parts of the
// spec that the Terraform generator needs. The same checks run for each library.
package specload_test

const (
	pathCollection = "/ai/evaluations/v3"
	pathItem       = "/ai/evaluations/v3/{id}"
	refEvalConfig  = "#/components/schemas/EvaluationConfig"
	refSQLLoad     = "#/components/schemas/SqlLoadConfig"
)

var (
	wantCreateProps = []string{"application", "config", "isEnabled", "subsystem", "target", "threshold"}
	wantUpdateProps = []string{"application", "config", "isEnabled", "subsystem", "target", "threshold", "updateMask"}
	wantSQLLoad     = []string{"allowRecursiveCte", "cteLimit", "joinLimit"}
)

const wantArms = 19

// overlaySample has the keywords that the overlay (step 1) adds, and 3.1-only
// syntax. The source spec does not have them yet.
const overlaySample = `openapi: 3.1.0
info:
  title: overlay sample
  version: "1"
paths: {}
components:
  schemas:
    Sample:
      type: object
      properties:
        joinLimit:
          type: string
          format: uint64
          pattern: ^[0-9]+$
        isEnabled:
          type: boolean
          default: false
        threshold:
          type: number
          format: double
          x-coralogix-presence: true
        topics:
          type: array
          uniqueItems: true
          x-coralogix-collection: set
          items:
            type: string
        nickname:
          type: [string, "null"]
        label:
          type: string
          examples: [a, b]
`

// propertyOrder is the order of Sample properties in overlaySample.
var propertyOrder = []string{"joinLimit", "isEnabled", "threshold", "topics", "nickname", "label"}
