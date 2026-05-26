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

package clientset

import "testing"

func TestGrpcTargetFromDomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		domain string
		want   string
	}{
		{
			domain: "api.private.eu2.coralogix.com",
			want:   "api.private.eu2.coralogix.com:443",
		},
		{
			domain: "https://api.private.us1.coralogix.com/",
			want:   "api.private.us1.coralogix.com:443",
		},
		{
			domain: "custom.example.com",
			want:   "ng-api-grpc.custom.example.com:443",
		},
		{
			domain: "api.eu2.coralogix.com",
			want:   "ng-api-grpc.api.eu2.coralogix.com:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			t.Parallel()
			if got := GrpcTargetFromDomain(tt.domain); got != tt.want {
				t.Fatalf("GrpcTargetFromDomain(%q) = %q, want %q", tt.domain, got, tt.want)
			}
		})
	}
}
