package failreport_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/acctest/failreport"
)

// Each code has one error text of Terraform or of the API. ErrorCheck turns it into one line with that code.
func TestErrorCheckGivesOneLinePerCode(t *testing.T) {
	failreport.Install()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_, _ = w.Write([]byte(`{"name":"x","clientSecret":"s3cr3t"}`))
	}))
	defer server.Close()
	apiFields := map[string]string{"runbook": "AlertDef.runbook", "search.text": "Search.text"}
	cases := []struct {
		code failreport.Code
		err  string
		path string
		api  string
	}{
		{failreport.ReadDropsValue, `Provider produced inconsistent result after apply: .runbook: was "a", but now null.`, "runbook", "AlertDef.runbook"},
		{failreport.ReadDropsValue, `.color_overrides: element "key" has vanished.`, "color_overrides", ""},
		{failreport.ServerDefault, `.search[0].text: was cty.NullVal(cty.String), but now cty.StringVal("x").`, "search[0].text", ""},
		{failreport.ServerNormalize, `.runbook: was "A", but now "a".`, "runbook", "AlertDef.runbook"},
		{failreport.ValueRejected, `rpc error: 400 invalid value for enum field priority: "acc-1"`, "", ""},
		{failreport.NotDeployed, `API error: 400 unknown field "runbook"`, "", ""},
		{failreport.PlanNotEmpty, "After applying this test step, the plan was not empty.", "", ""},
		{failreport.ImportDiff, "ImportStateVerify attributes not equivalent. Difference is shown below.", "", ""},
		{failreport.DataSourceDiff, `.color_overrides.%: resource absent, data source "0"`, "color_overrides.%", ""},
		{failreport.Unclassified, "something else broke", "", ""},
	}
	seen := map[failreport.Code]bool{}
	for _, c := range cases {
		failreport.Reset()
		req, _ := http.NewRequest(http.MethodPost, server.URL+"/v3/things?token=abc", strings.NewReader(`{"runbook":"x","password":"p","nested":{"secret_wo":"w"}}`))
		req.Header.Set("Authorization", "Bearer key")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()

		got := failreport.ErrorCheck(apiFields)(errors.New(c.err))
		_, line, ok := strings.Cut(got.Error(), failreport.Marker)
		if !ok || strings.Count(got.Error(), failreport.Marker) != 1 {
			t.Fatalf("%s: want one %q line in %q", c.code, failreport.Marker, got)
		}
		if !strings.HasPrefix(got.Error(), c.err) {
			t.Errorf("%s: the error text must stay first: %q", c.code, got)
		}
		var l failreport.Line
		if err := json.Unmarshal([]byte(line), &l); err != nil {
			t.Fatalf("%s: %v", c.code, err)
		}
		if l.Code != c.code {
			t.Errorf("code = %s, want %s for %q", l.Code, c.code, c.err)
		}
		if l.Owner == "" {
			t.Errorf("%s: no owner", c.code)
		}
		if strings.TrimPrefix(l.TFPath, ".") != c.path || l.APIField != c.api {
			t.Errorf("%s: tfPath %q apiField %q, want %q %q", c.code, l.TFPath, l.APIField, c.path, c.api)
		}
		for _, secret := range []string{"Bearer", `"p"`, `"w"`, "abc", "s3cr3t"} {
			if strings.Contains(line, secret) {
				t.Errorf("%s: the line holds %q:\n%s", c.code, secret, line)
			}
		}
		if !strings.HasPrefix(l.Request, "POST /v3/things") || !strings.Contains(l.Request, `"runbook":"x"`) || !strings.Contains(l.Response, "200") {
			t.Errorf("%s: request %q response %q", c.code, l.Request, l.Response)
		}
		seen[l.Code] = true
	}
	if len(seen) != len(cases)-1 { // ReadDropsValue appears twice
		t.Errorf("codes seen: %d", len(seen))
	}
}

// A path that the config of the step sets is a normalization; one that it does not set is derived.
func TestConfigPathsDecideDerivedDefault(t *testing.T) {
	paths := failreport.ConfigPaths(`
resource "coralogix_x" "test" {
  name = "a"
  search = { text = "t", tags = ["a"] }
  items = [{ label = "l" }]
}`)
	for _, want := range []string{"name", "search", "search.text", "search.tags", "items", "items.label"} {
		if !paths[want] {
			t.Errorf("paths = %v, want %s", paths, want)
		}
	}
	if got := failreport.Classify(errors.New(`.search.0.text: was "a", but now "b".`), paths).Code; got != failreport.ServerNormalize {
		t.Errorf("set path: code = %s", got)
	}
	if got := failreport.Classify(errors.New(`.limit: was "a", but now "b".`), paths).Code; got != failreport.DerivedDefault {
		t.Errorf("unset path: code = %s", got)
	}
}

func TestRedact(t *testing.T) {
	got := failreport.Redact([]byte(`{"a":1,"apiToken":"t","list":[{"clientSecret":"s","ok":true}],"x_wo":"w"}`))
	for _, leak := range []string{`"t"`, `"s"`, `"w"`} {
		if strings.Contains(got, leak) {
			t.Errorf("redacted body holds %s: %s", leak, got)
		}
	}
	if !strings.Contains(got, `"ok":true`) {
		t.Errorf("redacted body lost a plain field: %s", got)
	}
}
