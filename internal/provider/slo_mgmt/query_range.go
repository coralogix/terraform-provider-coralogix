// Copyright 2024 Coralogix Ltd.
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

package slo_mgmt

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// requestBasedQueryPaths are the queries of a request-based SLI. The backend evaluates them once
// a minute and rewrites a query that has no range selector: it injects `[1m]` into every rollup
// and prints the query again, so the stored text differs from the configured one and the apply
// ends in an inconsistent result. A window-based SLI query is stored as written, so it is not
// checked.
var requestBasedQueryPaths = []path.Path{
	path.Root("sli").AtName("request_based_metric_sli").AtName("good_events").AtName("query"),
	path.Root("sli").AtName("request_based_metric_sli").AtName("total_events").AtName("query"),
}

// validateRequestBasedQueryRanges rejects a request-based SLI query that has no range selector.
// It checks presence only: the backend accepts `[1m]` and rejects other ranges, and which
// spellings it accepts is its decision. A null or unknown query is skipped, because a value that
// comes from another resource is unknown while Terraform validates the configuration.
func validateRequestBasedQueryRanges(ctx context.Context, config tfsdk.Config) diag.Diagnostics {
	var diags diag.Diagnostics
	for _, queryPath := range requestBasedQueryPaths {
		var query types.String
		if d := config.GetAttribute(ctx, queryPath, &query); d.HasError() {
			// A missing parent object is a null value, not an error. Other errors belong to
			// the schema validation that already reports them.
			continue
		}
		if query.IsNull() || query.IsUnknown() {
			continue
		}
		if !hasRangeSelector(query.ValueString()) {
			diags.AddAttributeError(queryPath, "Query has no range selector",
				"The backend evaluates request-based SLI queries every minute, so it requires a `[1m]` range "+
					"selector in the query, for example `sum(rate(requests_total{status=\"ok\"}[1m]))`. Without "+
					"one it injects `[1m]` into the query and stores the rewritten text, which then differs from "+
					"the configuration. Add the `[1m]` range to the query.")
		}
	}
	return diags
}

// hasRangeSelector reports whether the PromQL text has a range selector or a subquery: a `[`
// outside a string literal or a comment. Label values such as {x=~"[a-z]+"} are string literals,
// so their brackets do not count. It does not parse the query, so a malformed range passes and
// is left to the backend.
func hasRangeSelector(query string) bool {
	var quote rune // the open string delimiter, or 0
	escaped := false
	inComment := false
	for _, r := range query {
		switch {
		case inComment:
			inComment = r != '\n'
		case quote != 0:
			switch {
			case escaped:
				escaped = false
			case r == '\\' && quote != '`':
				escaped = true
			case r == quote:
				quote = 0
			}
		case r == '"' || r == '\'' || r == '`':
			quote = r
		case r == '#':
			inComment = true
		case r == '[':
			return true
		}
	}
	return false
}
