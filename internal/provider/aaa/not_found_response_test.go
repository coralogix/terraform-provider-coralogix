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

package aaa

import (
	"net/http"
	"testing"
)

func TestIsNotFoundResponse(t *testing.T) {
	tests := []struct {
		name string
		resp *http.Response
		want bool
	}{
		{
			name: "nil_response_is_not_404",
			resp: nil,
			want: false,
		},
		{
			name: "confirmed_404",
			resp: &http.Response{StatusCode: http.StatusNotFound},
			want: true,
		},
		{
			name: "500_is_not_404",
			resp: &http.Response{StatusCode: http.StatusInternalServerError},
			want: false,
		},
		{
			name: "401_is_not_404",
			resp: &http.Response{StatusCode: http.StatusUnauthorized},
			want: false,
		},
		{
			name: "200_is_not_404",
			resp: &http.Response{StatusCode: http.StatusOK},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNotFoundResponse(tc.resp); got != tc.want {
				t.Fatalf("isNotFoundResponse() = %v, want %v", got, tc.want)
			}
		})
	}
}
