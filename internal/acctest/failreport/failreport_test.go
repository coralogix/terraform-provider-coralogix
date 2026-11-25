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
		{failreport.ServerDefault, `.search[0].text: was cty.NullVal(cty.String), but now cty.StringVal("x").`, "search[0].text", "Search.text"},
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

// A bracket path that Terraform writes (".search[0].text") finds its API field and its set path.
func TestBracketPathsAreNormalized(t *testing.T) {
	paths := failreport.ConfigPaths(`
resource "coralogix_x" "test" {
  search = [{ text = "t" }]
}`)
	got := failreport.Classify(errors.New(`.search[0].text: was "a", but now "b".`), paths)
	if got.Code != failreport.ServerNormalize {
		t.Errorf("code = %s, want %s: the path is set by the config", got.Code, failreport.ServerNormalize)
	}
	got = failreport.Classify(errors.New(`.items["k"].name: was "a", but now "b".`), map[string]bool{"items.name": true})
	if got.Code != failreport.ServerNormalize {
		t.Errorf("map key path: code = %s", got.Code)
	}
}

// A field that the schema marks sensitive is redacted, whatever its name is (the value of an API key).
func TestSensitiveFieldsAreRedacted(t *testing.T) {
	failreport.Install("value")
	got := failreport.Redact([]byte(`{"name":"k","value":"plain-secret","nested":{"Value":"x"}}`))
	if strings.Contains(got, "plain-secret") || strings.Contains(got, `"x"`) {
		t.Errorf("a sensitive field is in the body: %s", got)
	}
	if !strings.Contains(got, `"name":"k"`) {
		t.Errorf("a plain field was lost: %s", got)
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

// Every form of path that Terraform prints finds its API field and its set path: a bracket index, a
// quoted map key with a space, a set element written as a cty value, and the key of a map that the
// config states in HCL.
func TestPathFormsAreNormalized(t *testing.T) {
	failreport.Install()
	apiFields := map[string]string{"search.text": "Search.text", "tags": "Thing.tags", "colors.color": "Color.color"}
	set := map[string]bool{"search.text": true, "tags": true, "colors": true, "colors.cpu": true, "colors.cpu.color": true}
	cases := []struct {
		err  string
		code failreport.Code
		api  string
	}{
		{`.search[0].text: was "a", but now "b".`, failreport.ServerNormalize, "Search.text"},
		{`.search.0.text: was "a", but now "b".`, failreport.ServerNormalize, "Search.text"},
		{`.tags[cty.StringVal("a")]: was cty.StringVal("a"), but now null.`, failreport.ReadDropsValue, "Thing.tags"},
		{`.colors["a b"].color: was "a", but now "b".`, failreport.ServerNormalize, "Color.color"},
		{`.colors.cpu.color: was "a", but now "b".`, failreport.ServerNormalize, ""},
		{`.search[0].limit: was "a", but now "b".`, failreport.DerivedDefault, ""},
	}
	for _, c := range cases {
		got := failreport.Classify(errors.New(c.err), set)
		if got.Code != c.code {
			t.Errorf("%s: code = %s, want %s", c.err, got.Code, c.code)
		}
		var l failreport.Line
		if err := json.Unmarshal([]byte(strings.TrimPrefix(failreport.Describe(errors.New(c.err), apiFields), failreport.Marker)), &l); err != nil {
			t.Fatal(err)
		}
		if l.APIField != c.api {
			t.Errorf("%s: apiField = %q, want %q", c.err, l.APIField, c.api)
		}
	}
}

// The config of a step also gives the paths of nested blocks.
func TestConfigPathsHoldBlocks(t *testing.T) {
	paths := failreport.ConfigPaths(`
resource "coralogix_x" "test" {
  rule {
    name = "n"
    target { id = "i" }
  }
}`)
	for _, want := range []string{"rule", "rule.name", "rule.target", "rule.target.id"} {
		if !paths[want] {
			t.Errorf("paths = %v, want %s", paths, want)
		}
	}
}

// A secret never reaches the line: not in a body that is not JSON, not in a transport error, and not
// under the names that credentials usually have.
func TestMoreSecretsAreRedacted(t *testing.T) {
	failreport.Reset()
	failreport.Install("signingValue")
	for _, body := range []string{
		"token=abc123&name=x",
		"password: hunter2\nname: x",
		`{"privateKey":"pk-1","Authorization":"Bearer zz","credentials":{"user":"u"},"signingValue":"sv-1"}`,
		`not json, signingValue = "sv-2"`,
	} {
		got := failreport.Redact([]byte(body))
		for _, leak := range []string{"abc123", "hunter2", "pk-1", "Bearer zz", `"user":"u"`, "sv-1", "sv-2"} {
			if strings.Contains(got, leak) {
				t.Errorf("body %q leaks %q: %s", body, leak, got)
			}
		}
	}

	// A refused connection to a URL with a secret in the query: the error names the URL, the line must not.
	dead := httptest.NewServer(http.NotFoundHandler())
	url := dead.URL + "/v3/x?apikey=urlsecret"
	dead.Close()
	if resp, err := http.Post(url, "application/json", strings.NewReader(`{"a":1}`)); err == nil {
		_ = resp.Body.Close()
		t.Fatal("the server is closed: want an error")
	}
	line := failreport.Describe(errors.New("something else broke"), nil)
	if strings.Contains(line, "urlsecret") {
		t.Errorf("the line holds the secret of the URL: %s", line)
	}
	if !strings.Contains(line, `"request":"POST /v3/x`) {
		t.Errorf("the line lost the request: %s", line)
	}
}

// A service key (the routing key of a pager service) is a credential under a name that has no
// token or secret in it.
func TestServiceKeysAreRedacted(t *testing.T) {
	got := failreport.Redact([]byte(`{"serviceKey":"sk-1","routing_key":"rk-1","projectKey":"PROJ"}`))
	for _, leak := range []string{"sk-1", "rk-1"} {
		if strings.Contains(got, leak) {
			t.Errorf("the body leaks %s: %s", leak, got)
		}
	}
	if !strings.Contains(got, "PROJ") {
		t.Errorf("a plain key field was redacted: %s", got)
	}
}
