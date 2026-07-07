// Copyright 2026 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alertschema

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

func TestEvaluationDelaySchemaModelsOptionalOverride(t *testing.T) {
	attr, ok := evaluationDelaySchema().(schema.Int32Attribute)
	if !ok {
		t.Fatalf("evaluationDelaySchema() returned %T, want schema.Int32Attribute", evaluationDelaySchema())
	}

	if !attr.Optional {
		t.Fatal("custom_evaluation_delay must remain optional")
	}
	if attr.Computed {
		t.Fatal("custom_evaluation_delay must not be computed; omitted config should remain unset")
	}
	if attr.Default != nil {
		t.Fatal("custom_evaluation_delay must not default to 0; omitted config should be sent as nil")
	}
	if len(attr.PlanModifiers) != 0 {
		t.Fatalf("custom_evaluation_delay must not preserve prior state; got %d plan modifier(s)", len(attr.PlanModifiers))
	}
}
