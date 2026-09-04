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

package fleet

import "testing"

func TestYAMLStringsEqualTreatsInlineAndMultilineListsAsTheSame(t *testing.T) {
	inline := "receivers: [otlp]\n"
	multiline := "receivers:\n  - otlp\n"
	if !yamlStringsEqual(inline, multiline) {
		t.Fatal("inline and multiline YAML lists should compare equal")
	}
}

func TestYAMLStringsEqualRejectsDifferentDocuments(t *testing.T) {
	if yamlStringsEqual("receivers: [otlp]\n", "receivers: [http]\n") {
		t.Fatal("different YAML documents should not compare equal")
	}
}
