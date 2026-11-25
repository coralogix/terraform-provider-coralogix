// Package failreport turns a failure of a generated acceptance test into one JSON line that a person
// who owns the API can act on.
//
// The generated test installs a recording transport (Install), numbers its steps (Steps), and sets
// ErrorCheck on the resource.TestCase. ErrorCheck classifies each error of a step with a Code and
// appends the line to the error text:
//
//	TFGEN_FAILURE {"code":"READ_DROPS_VALUE","tfPath":"runbook","apiField":"AlertDef.runbook",...}
//
// The classification reads the error text of Terraform and of the API. It never hides the error: the
// test still fails with the original text first.
package failreport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/zclconf/go-cty/cty"
)

// Marker starts the line of a failure in the error text.
const Marker = "TFGEN_FAILURE "

// Code classifies a failure. Its owner decides who fixes it.
type Code string

const (
	ReadDropsValue  Code = "READ_DROPS_VALUE"  // was "A", but now null
	ServerDefault   Code = "SERVER_DEFAULT"    // was null, but now "X"
	DerivedDefault  Code = "DERIVED_DEFAULT"   // a value changed that the config of the step does not set
	ServerNormalize Code = "SERVER_NORMALIZES" // was "A", but now "B"
	ValueRejected   Code = "VALUE_REJECTED"    // 400, invalid value
	NotDeployed     Code = "NOT_DEPLOYED"      // 400, unknown field
	PlanNotEmpty    Code = "PLAN_NOT_EMPTY"    // the plan after apply is not empty
	ImportDiff      Code = "IMPORT_DIFF"       // ImportStateVerify found a difference
	DataSourceDiff  Code = "DATASOURCE_DIFF"   // the resource and the data source differ
	Unclassified    Code = "UNCLASSIFIED"      // anything else
)

// Owner says who acts on the code: "API" (the owner of the API change), "Infra" (rerun later), or "Provider".
func (c Code) Owner() string {
	switch c {
	case NotDeployed:
		return "Infra"
	case Unclassified:
		return "Provider"
	}
	return "API"
}

// Line is the JSON line of a failure.
type Line struct {
	Code     Code   `json:"code"`
	Owner    string `json:"owner"`
	TFPath   string `json:"tfPath,omitempty"`
	APIField string `json:"apiField,omitempty"`
	Step     string `json:"step,omitempty"`
	Request  string `json:"request,omitempty"`
	Response string `json:"response,omitempty"`
}

// Classification is the result of Classify.
type Classification struct {
	Code   Code
	TFPath string
}

var (
	// "path: was X, but now Y". Terraform writes X and Y as a plain value or as a cty value.
	// A Terraform path: ".search[0].text", `.items["a b"].name`, ".tags[cty.StringVal(\"a\")]", ".map.%".
	tfPath       = `((?:\.[\w%#*-]+|\[[^\]]*\])+)`
	inconsistent = regexp.MustCompile(tfPath + `: was (.+?), but now (.+?)(?:\.\s|\.$|$|\n)`)
	vanished     = regexp.MustCompile(tfPath + `: element "[^"]*" has vanished`)
	blockCount   = regexp.MustCompile(tfPath + `: block count changed from \d+ to 0`)
	absent       = regexp.MustCompile(tfPath + `: resource absent, data source`)
	unknownField = regexp.MustCompile(`(?s)\b400\b.*unknown field`)
	rejected     = regexp.MustCompile(`(?is)\b400\b|invalid value for enum|invalid argument|InvalidArgument`)
)

func isNull(v string) bool {
	v = strings.TrimSpace(v)
	return v == "null" || strings.HasPrefix(v, "cty.NullVal(")
}

func cleanPath(p string) string { return strings.Trim(strings.TrimPrefix(p, "."), ":") }

// Classify names the failure by the error text. set holds the paths that the config of the failing step
// sets (see ConfigPaths); a nil set means that it is not known.
func Classify(err error, set map[string]bool) Classification {
	text := err.Error()
	switch {
	case strings.Contains(text, "the plan was not empty"):
		return Classification{PlanNotEmpty, ""}
	case strings.Contains(text, "ImportStateVerify attributes not equivalent"):
		return Classification{ImportDiff, ""}
	}
	if m := absent.FindStringSubmatch(text); m != nil {
		return Classification{DataSourceDiff, cleanPath(m[1])}
	}
	if strings.Contains(text, "data.coralogix_") && strings.Contains(text, "expected") {
		return Classification{DataSourceDiff, ""}
	}
	if unknownField.MatchString(text) {
		return Classification{NotDeployed, ""}
	}
	if m := vanished.FindStringSubmatch(text); m != nil {
		return Classification{ReadDropsValue, cleanPath(m[1])}
	}
	if m := blockCount.FindStringSubmatch(text); m != nil {
		return Classification{ReadDropsValue, cleanPath(m[1])}
	}
	if m := inconsistent.FindStringSubmatch(text); m != nil {
		path, was, now := cleanPath(m[1]), m[2], m[3]
		switch {
		case isNull(now) && !isNull(was):
			return Classification{ReadDropsValue, path}
		case isNull(was) && !isNull(now):
			return Classification{ServerDefault, path}
		case set != nil && !isSet(set, normalize(path)):
			return Classification{DerivedDefault, path}
		}
		return Classification{ServerNormalize, path}
	}
	if rejected.MatchString(text) {
		return Classification{ValueRejected, ""}
	}
	return Classification{Unclassified, ""}
}

var index = regexp.MustCompile(`\.(\d+|\*|%|#)(\.|$)`)

// dropBrackets removes every [..] step of a path. A step can hold quotes and brackets of its own
// (`["a]b"]`, `[cty.StringVal("x")]`), so it is read with a scanner, not a pattern.
func dropBrackets(path string) string {
	var out strings.Builder
	depth, quoted, escaped := 0, false, false
	for _, r := range path {
		switch {
		case depth == 0 && r == '[':
			depth = 1
		case depth == 0:
			out.WriteRune(r)
		case escaped:
			escaped = false
		case quoted && r == '\\':
			escaped = true
		case r == '"':
			quoted = !quoted
		case !quoted && r == '[':
			depth++
		case !quoted && r == ']':
			depth--
		}
	}
	return out.String()
}

// normalize removes the positions of lists, sets and maps from a Terraform path: "a.0.b", "a[0].b",
// `a["k"].b` and `a[cty.StringVal("k")].b` are "a.b".
func normalize(path string) string {
	path = dropBrackets(path)
	for {
		next := index.ReplaceAllString(path, "$2")
		if next == path {
			return strings.TrimSuffix(path, ".")
		}
		path = next
	}
}

// isSet says that the config of the step sets the attribute at the path. The paths of the config hold
// the keys of a map (`colors.cpu.color`), which the path of an error does not, so the segments of the
// path must be a subsequence of the segments of a config path, with the same first and last segment.
func isSet(set map[string]bool, path string) bool {
	if set[path] {
		return true
	}
	want := strings.Split(path, ".")
	for p := range set {
		have := strings.Split(p, ".")
		if len(have) <= len(want) || have[0] != want[0] || have[len(have)-1] != want[len(want)-1] {
			continue
		}
		i := 0
		for _, seg := range have {
			if i < len(want) && seg == want[i] {
				i++
			}
		}
		if i == len(want) {
			return true
		}
	}
	return false
}

// Exchange is one HTTP request and its response.
type Exchange struct {
	Method, URL, RequestBody string
	Status                   int
	ResponseBody             string
}

func (e Exchange) request() string {
	return strings.TrimSpace(e.Method + " " + e.URL + " " + e.RequestBody)
}
func (e Exchange) response() string {
	return strings.TrimSpace(fmt.Sprintf("%d %s", e.Status, e.ResponseBody))
}

const (
	maxBody      = 2000
	maxExchanges = 50
)

var (
	mu        sync.Mutex
	exchanges []Exchange
	step      string
	stepPaths map[string]bool
	installed sync.Once
	// sensitive are the names of API fields that the schema marks sensitive or write-only, in lower case.
	sensitive = map[string]bool{}
)

type recorder struct{ next http.RoundTripper }

func (r recorder) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewReader(body))
	}
	resp, err := r.next.RoundTrip(req)
	ex := Exchange{Method: req.Method, URL: redactURL(req.URL), RequestBody: Redact(body)}
	if err == nil {
		payload, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		resp.Body = io.NopCloser(bytes.NewReader(payload))
		ex.Status, ex.ResponseBody = resp.StatusCode, Redact(payload)
	} else {
		// A transport error holds the full URL, with its query. Keep only the cause.
		var urlError *url.Error
		if errors.As(err, &urlError) {
			err = urlError.Err
		}
		ex.ResponseBody = err.Error()
	}
	mu.Lock()
	if exchanges = append(exchanges, ex); len(exchanges) > maxExchanges {
		exchanges = exchanges[1:]
	}
	mu.Unlock()
	return resp, err
}

// Install records the HTTP exchanges of the provider, which the test serves in the same process. The
// SDK clients use http.DefaultClient, so the DefaultTransport sees all of them. The Authorization header
// is never read. Call it at the start of a test. sensitiveFields are the API names of the fields that the
// schema of the resource marks sensitive or write-only (an API key value, for example): they are redacted
// like the names that end in token or secret, whatever their name is.
func Install(sensitiveFields ...string) {
	mu.Lock()
	for _, name := range sensitiveFields {
		sensitive[strings.ToLower(name)] = true
	}
	mu.Unlock()
	installed.Do(func() { http.DefaultTransport = recorder{next: http.DefaultTransport} })
}

var secretKey = regexp.MustCompile(`(?i)(_wo|token|secret|password|passwd|passphrase|apikey|api_key|privatekey|private_key|secretkey|accesskey|access_key|servicekey|service_key|routingkey|routing_key|integrationkey|integration_key|signingkey|signing_key|encryptionkey|encryption_key|authkey|auth_key|sharedkey|shared_key|masterkey|master_key|credentials?|authorization|cookie)$`)

func isSecret(name string) bool {
	if secretKey.MatchString(name) {
		return true
	}
	mu.Lock()
	defer mu.Unlock()
	return sensitive[strings.ToLower(name)]
}

func redactURL(u *url.URL) string {
	c := *u
	c.User = nil
	q := c.Query()
	for k := range q {
		if isSecret(k) {
			q.Set(k, "[REDACTED]")
		}
	}
	c.RawQuery = q.Encode()
	return c.Path + func() string {
		if c.RawQuery == "" {
			return ""
		}
		return "?" + c.RawQuery
	}()
}

// Redact removes the values of the secret fields of a JSON body (names that end in _wo, token, secret,
// password) and cuts the text to a length that a comment can show. A body that is not JSON is cut only.
func Redact(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return cut(redactText(string(body)))
	}
	out, err := json.Marshal(redactValue(v))
	if err != nil {
		return ""
	}
	return cut(string(out))
}

// textSecret matches name=value and name: value pairs of a body that is not JSON (a form, a log line).
var textSecret = regexp.MustCompile(`(?i)("?[\w.-]*(?:_wo|token|secret|password|passwd|passphrase|apikey|api_key|privatekey|private_key|secretkey|accesskey|access_key|servicekey|service_key|routingkey|routing_key|integrationkey|integration_key|signingkey|signing_key|encryptionkey|encryption_key|authkey|auth_key|sharedkey|shared_key|masterkey|master_key|credentials?|authorization|cookie)"?\s*[:=]\s*)("[^"]*"|[^\s&,;]+)`)

func redactText(s string) string {
	s = textSecret.ReplaceAllString(s, `${1}"[REDACTED]"`)
	mu.Lock()
	names := make([]string, 0, len(sensitive))
	for n := range sensitive {
		names = append(names, regexp.QuoteMeta(n))
	}
	mu.Unlock()
	if len(names) == 0 {
		return s
	}
	named := regexp.MustCompile(`(?i)("?(?:` + strings.Join(names, "|") + `)"?\s*[:=]\s*)("[^"]*"|[^\s&,;]+)`)
	return named.ReplaceAllString(s, `${1}"[REDACTED]"`)
}

func cut(s string) string {
	if len(s) > maxBody {
		return s[:maxBody] + "…"
	}
	return s
}

func redactValue(v any) any {
	switch v := v.(type) {
	case map[string]any:
		for k, e := range v {
			if isSecret(k) {
				v[k] = "[REDACTED]"
				continue
			}
			v[k] = redactValue(e)
		}
	case []any:
		for i, e := range v {
			v[i] = redactValue(e)
		}
	}
	return v
}

// ConfigPaths returns the attribute paths that the resource blocks of an HCL config set. A position of
// a list or a set is not in the path: "search.text" for an item of a list "search".
func ConfigPaths(config string) map[string]bool {
	file, diags := hclsyntax.ParseConfig([]byte(config), "config.tf", hcl.InitialPos)
	if diags.HasErrors() {
		return nil
	}
	out := map[string]bool{}
	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return nil
	}
	for _, block := range body.Blocks {
		if block.Type != "resource" {
			continue
		}
		addBody(out, "", block.Body)
	}
	return out
}

// addBody adds the attributes of a body, and of the blocks inside it, with their paths.
func addBody(out map[string]bool, prefix string, body *hclsyntax.Body) {
	for name, attr := range body.Attributes {
		path := joinPath(prefix, name)
		out[path] = true
		addPaths(out, path, attr.Expr)
	}
	for _, nested := range body.Blocks {
		path := joinPath(prefix, nested.Type)
		out[path] = true
		addBody(out, path, nested.Body)
	}
}

func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func addPaths(out map[string]bool, prefix string, expr hclsyntax.Expression) {
	switch e := expr.(type) {
	case *hclsyntax.ObjectConsExpr:
		for _, item := range e.Items {
			key, diags := item.KeyExpr.Value(nil)
			if diags.HasErrors() || !key.IsKnown() || key.IsNull() || key.Type() != cty.String {
				continue
			}
			name := prefix + "." + key.AsString()
			out[name] = true
			addPaths(out, name, item.ValueExpr)
		}
	case *hclsyntax.TupleConsExpr:
		for _, item := range e.Exprs {
			addPaths(out, prefix, item)
		}
	case *hclsyntax.ObjectConsKeyExpr:
		addPaths(out, prefix, e.Wrapped)
	}
}

// Steps numbers the steps of a test case: before each step applies, it records the name of the subtest
// and the number of the step, and the paths that the config of the step sets. An import step has no
// config and keeps the paths of the step before it.
func Steps(t testing.TB, steps []resource.TestStep) []resource.TestStep {
	name := t.Name()
	if _, after, ok := strings.Cut(name, "/"); ok {
		name = after
	}
	for i := range steps {
		n, config, prior := i+1, steps[i].Config, steps[i].PreConfig
		steps[i].PreConfig = func() {
			if prior != nil {
				prior()
			}
			mu.Lock()
			defer mu.Unlock()
			step = fmt.Sprintf("%s/%d", name, n)
			if config != "" {
				stepPaths = ConfigPaths(config)
			}
		}
	}
	return steps
}

// ErrorCheck returns the ErrorCheck of a resource.TestCase. apiFields maps the Terraform path of an
// attribute (without positions) to its API field ("Component.property").
func ErrorCheck(apiFields map[string]string) resource.ErrorCheckFunc {
	return func(err error) error {
		if err == nil {
			return nil
		}
		return fmt.Errorf("%w\n%s%s", err, Marker, Describe(err, apiFields))
	}
}

// Describe returns the JSON line of the error, with the last request and the last response that the
// recorder saw.
func Describe(err error, apiFields map[string]string) string {
	mu.Lock()
	defer mu.Unlock()
	c := Classify(err, stepPaths)
	line := Line{Code: c.Code, Owner: c.Code.Owner(), TFPath: c.TFPath, APIField: apiFields[normalize(c.TFPath)], Step: step}
	if last, ok := lastWrite(); ok {
		line.Request = last.request()
	}
	if n := len(exchanges); n > 0 {
		line.Response = exchanges[n-1].response()
	}
	out, _ := json.Marshal(line)
	return string(out)
}

// lastWrite returns the last request that changes the API: the one that the failure is most likely about.
func lastWrite() (Exchange, bool) {
	for i := len(exchanges) - 1; i >= 0; i-- {
		switch exchanges[i].Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			return exchanges[i], true
		}
	}
	return Exchange{}, false
}

// Reset forgets the recorded exchanges and the step. A test of this package uses it.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	exchanges, step, stepPaths = nil, "", nil
}
