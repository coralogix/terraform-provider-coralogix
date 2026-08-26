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

package dashboard_widgets

import (
	"context"
	"fmt"
	"math"
	"math/big"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// int32NumberValidator rejects a number attribute value the API's int32 field
// cannot hold. Without it the conversion drops the fraction or wraps the
// magnitude, the request carries a value the user did not write, and the apply
// fails with an inconsistent result once that value is read back.
//
// This checks only what the wire type can represent, not what the API accepts.
// A documented business range stays unvalidated, because a wrong bound that
// blocks a valid configuration is harder to undo than a missing one.
type int32NumberValidator struct{}

func (v int32NumberValidator) Description(_ context.Context) string {
	return "value must be a whole number a 32-bit signed integer can hold"
}

func (v int32NumberValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v int32NumberValidator) ValidateNumber(_ context.Context, req validator.NumberRequest, resp *validator.NumberResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	value := req.ConfigValue.ValueBigFloat()
	if !value.IsInt() {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Attribute Value",
			fmt.Sprintf("Attribute %s must be a whole number, got %s.", req.Path, value.Text('f', -1)),
		)
		return
	}

	converted, accuracy := value.Int64()
	if accuracy != big.Exact || converted < math.MinInt32 || converted > math.MaxInt32 {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Attribute Value",
			fmt.Sprintf("Attribute %s must be between %d and %d, got %s.", req.Path, math.MinInt32, math.MaxInt32, value.Text('f', -1)),
		)
	}
}
