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

package provider

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
)

// userFidelityUnknownID is a userId that no user has. A recorded PUT for it shows the not-found answer.
const userFidelityUnknownID = "00000000-0000-4000-8000-000000000000"

// TestUserFakeMatchesRecordedBackend checks the fake against a live API recording.
func TestUserFakeMatchesRecordedBackend(t *testing.T) {
	cases, recorded := loadFidelityAt(t, userGoldenDir)
	for ci, c := range cases.Cases {
		t.Run(c.Name, func(t *testing.T) {
			rc := recorded.Cases[ci]
			if rc.Name != c.Name || len(rc.Steps) != len(c.Steps) {
				t.Fatalf("recorded.json does not match cases.json for %q: record again", c.Name)
			}
			fake := newUserFidelityFake()
			vars := userFidelityVars("fidelity")
			redactor := newUserFidelityRedactor(t, "fidelity")
			for si, step := range c.Steps {
				label := fmt.Sprintf("step %d: %s %s", si+1, step.Method, step.Path)
				status, got := runUserFidelityStep(t, fake, step, vars, redactor)
				compareFidelityStep(t, label, status, got, rc.Steps[si].Status, rc.Steps[si].Response)
			}
		})
	}
}

// runUserFidelityStep sends one step to the fake. It returns the status and the response after the
// same redaction that the recorder applies to the live answer.
func runUserFidelityStep(t *testing.T, fake *userFake, step fidelityStep, vars map[string]string, redactor *userFidelityRedactor) (int, any) {
	t.Helper()
	body := step.Body
	if body != nil {
		body = substJSON(t, body, vars)
	}
	path, rawQuery, _ := strings.Cut(substString(step.Path, vars), "?")
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	status, response := fake.handle(step.Method, userBasePath+path, query, body)
	fake.mu.Unlock()
	raw := roundTrip(t, response)
	if step.Capture != "" && status == http.StatusOK {
		vars[step.Capture] = userFidelityCapture(step.Capture, raw)
	}
	return status, userFidelityAnswer(t, redactor, raw, vars)
}

// userFidelityCapture returns the value that a step captures: the nextPageToken of the answer for a
// name that ends in _TOKEN, and the userId of the first user for any other name. The token keeps the
// text the API sent, so a later step sends it back unchanged.
func userFidelityCapture(name string, response any) string {
	if !strings.HasSuffix(name, "_TOKEN") {
		return firstUserID(response)
	}
	switch token := asObject(response)["nextPageToken"].(type) {
	case string:
		return token
	case float64:
		return strconv.FormatFloat(token, 'f', -1, 64)
	}
	return ""
}

// userFidelityAnswer redacts an answer and replaces the run's emails with their tokens.
func userFidelityAnswer(t *testing.T, redactor *userFidelityRedactor, raw any, vars map[string]string) any {
	t.Helper()
	return unsubstLongestFirst(t, redactor.redact(raw), vars)
}

// unsubstLongestFirst replaces the values of the variables with their tokens, the longest value
// first, because one value can be part of another. The ids are placeholders by now, so they stay. A page
// token stays too: it can be a short number that appears anywhere in the answer.
func unsubstLongestFirst(t *testing.T, v any, vars map[string]string) any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(vars))
	for name, value := range vars {
		if value != "" && !strings.HasPrefix(name, "ID_") && !strings.HasSuffix(name, "_TOKEN") {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool {
		if len(vars[names[i]]) != len(vars[names[j]]) {
			return len(vars[names[i]]) > len(vars[names[j]])
		}
		return names[i] < names[j]
	})
	text := string(raw)
	for _, name := range names {
		text = strings.ReplaceAll(text, vars[name], "{{"+name+"}}")
	}
	var out any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// firstUserID returns the userId of the first user in a create, search, or update answer.
func firstUserID(response any) string {
	root := asObject(response)
	for _, key := range []string{"results", "users"} {
		items, _ := root[key].([]any)
		if len(items) == 0 {
			continue
		}
		item := asObject(items[0])
		if id, ok := item["userId"].(string); ok {
			return id
		}
		if id, ok := asObject(item["user"])["userId"].(string); ok {
			return id
		}
	}
	return ""
}

// userFidelityVars are the values that cases.json names. Every email has the synthetic
// tf-acc-user-<run>-<letter>@coralogix.com shape, and the run is random in a live recording.
func userFidelityVars(run string) map[string]string {
	userA := fmt.Sprintf("tf-acc-user-%s-a@coralogix.com", run)
	return map[string]string{
		"RUN":          run,
		"USER_A":       userA,
		"USER_B":       fmt.Sprintf("tf-acc-user-%s-b@coralogix.com", run),
		"USER_A_UPPER": strings.ToUpper(userA),
		// USER_A_RENAMED is the username that a PUT asks user A to take.
		"USER_A_RENAMED": fmt.Sprintf("tf-acc-user-%s-a-renamed@coralogix.com", run),
		// PARTIAL is part of both usernames, so a search for it shows the partial match.
		"PARTIAL": fmt.Sprintf("tf-acc-user-%s-", run),
	}
}

func newUserFidelityRunID(t *testing.T) string {
	t.Helper()
	unique := make([]byte, 6)
	if _, err := rand.Read(unique); err != nil {
		t.Fatalf("generate unique fidelity run ID: %v", err)
	}
	return hex.EncodeToString(unique)
}

// userFidelityRedactor removes what must never be committed from an answer of the live API. It denies
// by default: a key that userFidelityResponseKeys does not name stops the recording, so a field that
// a future API version adds is never copied unseen. The recorder applies it to every answer before it
// is written, and the fake's answers go through it too, so the two compare on the same terms:
//   - ids become placeholders in the shape of the id: userId a UUID, userAccountId an int64
//   - groupIds, totalCount, and team and company ids become placeholders
//   - a member that the run did not create gets a synthetic email and name
//   - every email other than this run's users is replaced, also inside messages
//   - in a message, a run of five or more digits becomes <n> and a domain other than coralogix.com
//     becomes <domain>, because a message can quote any value of the request
//   - request and trace ids are dropped
//   - a page token that is text and not a small offset becomes <page-token>
//
// The recorder only searches for usernames that this run's users match, so other members are not
// expected in an answer. This is the second line of defense.
type userFidelityRedactor struct {
	run      string
	userIDs  map[string]string
	accounts map[int64]int64
	others   map[string]string
	// fail stops the recording. It is t.Fatal in the recorder; a test replaces it to see the reason.
	fail func(reason string)
}

// userFidelityResponseKeys are the keys that an answer of the Users API may have, at any depth: the
// users, the create results, the update answer, and the error body. Every other key stops the recording.
// The keys that the redactor drops or replaces are in the list too, because the live API sends them.
var userFidelityResponseKeys = map[string]bool{
	"users": true, "user": true, "results": true, "totalCount": true, "nextPageToken": true,
	"userId": true, "userAccountId": true, "userAccountIds": true, "username": true,
	"firstName": true, "lastName": true, "status": true, "groupIds": true,
	"allowedLoginMode": true, "accessType": true, "permanentAccess": true, "temporaryAccess": true, "expireAt": true,
	"code": true, "message": true,
	"teamId": true, "companyId": true, "accountId": true,
	"requestId": true, "request_id": true, "traceId": true, "trace_id": true,
}

var (
	userFidelityUUIDPattern  = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	userFidelityEmailPattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+[A-Za-z0-9]`)
	// userFidelityTokenPattern finds what a message holds that has its own placeholder. The email comes
	// first, so an email that has a UUID in front of the @ is one email.
	userFidelityTokenPattern  = regexp.MustCompile(userFidelityEmailPattern.String() + "|" + userFidelityUUIDPattern.String())
	userFidelityDomainPattern = regexp.MustCompile(`\b(?:[A-Za-z0-9](?:[A-Za-z0-9-]*[A-Za-z0-9])?\.)+[A-Za-z]{2,}\b`)
	userFidelityDigitRun      = regexp.MustCompile(`[0-9]{5,}`)
	// userFidelityOffsetToken is a page token that is a small offset. Any other text token is opaque:
	// it can encode ids of the team, so the recording keeps only that it was text.
	userFidelityOffsetToken = regexp.MustCompile(`^[0-9]{1,4}$`)
)

func newUserFidelityRedactor(t *testing.T, run string) *userFidelityRedactor {
	return &userFidelityRedactor{
		run:      strings.ToLower(run),
		userIDs:  map[string]string{userFidelityUnknownID: userFidelityUnknownID},
		accounts: map[int64]int64{},
		others:   map[string]string{},
		fail: func(reason string) {
			t.Helper()
			t.Fatal(reason)
		},
	}
}

func (r *userFidelityRedactor) ours(email string) bool {
	return strings.Contains(strings.ToLower(email), r.run)
}

func (r *userFidelityRedactor) userID(id string) string {
	if placeholder, ok := r.userIDs[id]; ok {
		return placeholder
	}
	placeholder := fmt.Sprintf(userIDFormat, len(r.userIDs))
	r.userIDs[id] = placeholder
	return placeholder
}

func (r *userFidelityRedactor) account(id int64) int64 {
	if placeholder, ok := r.accounts[id]; ok {
		return placeholder
	}
	placeholder := int64(userAccountIDBase + len(r.accounts) + 1)
	r.accounts[id] = placeholder
	return placeholder
}

func (r *userFidelityRedactor) other(email string) string {
	key := strings.ToLower(email)
	if placeholder, ok := r.others[key]; ok {
		return placeholder
	}
	placeholder := fmt.Sprintf("tf-acc-user-other-%d@coralogix.com", len(r.others)+1)
	r.others[key] = placeholder
	return placeholder
}

func (r *userFidelityRedactor) redact(value any) any {
	switch value := value.(type) {
	case map[string]any:
		return r.redactObject(value)
	case []any:
		out := make([]any, len(value))
		for i, item := range value {
			out[i] = r.redact(item)
		}
		return out
	case string:
		return r.redactText(value)
	}
	return value
}

func (r *userFidelityRedactor) redactObject(object map[string]any) map[string]any {
	// A member that this run did not create has its names replaced too.
	username, hasUsername := object["username"].(string)
	foreign := hasUsername && !r.ours(username)

	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	out := make(map[string]any, len(object))
	for _, key := range keys {
		item := object[key]
		if !userFidelityResponseKeys[key] {
			r.fail(fmt.Sprintf("refusing to keep an answer with the key %q: check that it holds no member data, then add it to userFidelityResponseKeys", key))
			continue
		}
		switch key {
		case "requestId", "request_id", "traceId", "trace_id":
			continue
		case "userId":
			if id, ok := item.(string); ok {
				out[key] = r.userID(id)
				continue
			}
		case "userAccountId":
			if id, ok := item.(float64); ok {
				out[key] = float64(r.account(int64(id)))
				continue
			}
		case "userAccountIds":
			if ids, ok := item.([]any); ok {
				placeholders := make([]any, len(ids))
				for i, id := range ids {
					number, _ := id.(float64)
					placeholders[i] = float64(r.account(int64(number)))
				}
				out[key] = placeholders
				continue
			}
		case "groupIds":
			if ids, ok := item.([]any); ok {
				placeholders := make([]any, len(ids))
				for i := range ids {
					placeholders[i] = float64(i + 1)
				}
				out[key] = placeholders
				continue
			}
		case "totalCount", "teamId", "companyId", "accountId":
			if _, ok := item.(float64); ok {
				out[key] = float64(1)
				continue
			}
		case "nextPageToken":
			out[key] = redactPageToken(item)
			continue
		case "message":
			if text, ok := item.(string); ok {
				out[key] = r.redactMessage(text)
				continue
			}
		case "firstName", "lastName":
			if text, ok := item.(string); ok && foreign && text != "" {
				out[key] = map[string]string{"firstName": "Other", "lastName": "Member"}[key]
				continue
			}
		}
		out[key] = r.redact(item)
	}
	return out
}

// redactPageToken keeps a page token that is a number or a small offset, and replaces any other text.
func redactPageToken(token any) any {
	if text, ok := token.(string); ok && !userFidelityOffsetToken.MatchString(text) {
		return "<page-token>"
	}
	return token
}

// redactText replaces ids and emails inside a string: a username, or a message that quotes one.
func (r *userFidelityRedactor) redactText(text string) string {
	text = userFidelityUUIDPattern.ReplaceAllStringFunc(text, func(id string) string {
		return r.userID(strings.ToLower(id))
	})
	return userFidelityEmailPattern.ReplaceAllStringFunc(text, func(email string) string {
		if r.ours(email) {
			return email
		}
		return r.other(email)
	})
}

// redactMessage redacts a message of the API. A message can quote any value of the request, so beyond
// the ids and emails that redactText replaces, a run of five or more digits becomes <n> and a domain
// other than coralogix.com becomes <domain>. The ids and emails are replaced first and kept apart, because
// their placeholders hold digits.
func (r *userFidelityRedactor) redactMessage(text string) string {
	var out strings.Builder
	last := 0
	for _, loc := range userFidelityTokenPattern.FindAllStringIndex(text, -1) {
		out.WriteString(redactMessageText(text[last:loc[0]]))
		out.WriteString(r.redactText(text[loc[0]:loc[1]]))
		last = loc[1]
	}
	out.WriteString(redactMessageText(text[last:]))
	return out.String()
}

func redactMessageText(text string) string {
	text = userFidelityDomainPattern.ReplaceAllStringFunc(text, func(domain string) string {
		domain = strings.ToLower(domain)
		if domain == "coralogix.com" || strings.HasSuffix(domain, ".coralogix.com") {
			return domain
		}
		return "<domain>"
	})
	return userFidelityDigitRun.ReplaceAllString(text, "<n>")
}

func TestUserFidelityRedactorRemovesMemberData(t *testing.T) {
	redactor := newUserFidelityRedactor(t, "run1")
	live := map[string]any{
		"totalCount":    float64(8123),
		"nextPageToken": float64(1),
		"users": []any{
			map[string]any{
				"userId": "3f1c1e0a-52a1-4b8e-9b0e-0c6f4a1d2e77", "userAccountId": float64(93817265),
				"username": "tf-acc-user-run1-a@coralogix.com", "firstName": "Test", "lastName": "User",
				"groupIds": []any{float64(481516), float64(2342)}, "status": "USER_STATUS_ACTIVE",
			},
			map[string]any{
				"userId": "9a8b7c6d-1111-4222-8333-444455556666", "userAccountId": float64(55501234),
				"username": "Jane.Doe@example.com", "firstName": "Jane", "lastName": "Doe",
				"groupIds": []any{}, "status": "USER_STATUS_ACTIVE", "teamId": float64(4455667),
			},
		},
		"userAccountIds": []any{float64(93817265), float64(55501234)},
		"message": "Jane.Doe@example.com is already a member; request 9a8b7c6d-1111-4222-8333-444455556666; " +
			"account 77889900 of the team at corp.example.org also at login.coralogix.com",
		"requestId": "abc123",
	}
	// The placeholders <n> and <domain> stay readable: the recorder writes them without HTML escaping.
	raw := marshalUserFidelity(t, redactor.redact(live))
	if problems := userGoldenProblems(string(raw)); len(problems) != 0 {
		t.Fatalf("the redacted answer still has member data: %v\n%s", problems, raw)
	}
	for _, leaked := range []string{"Jane", "Doe", "8123", "481516", "2342", "requestId", "93817265", "4455667", "55501234", "77889900", "example", "corp"} {
		if strings.Contains(string(raw), leaked) {
			t.Errorf("the redacted answer still has %q: %s", leaked, raw)
		}
	}
	if !strings.Contains(string(raw), "tf-acc-user-run1-a@coralogix.com") {
		t.Errorf("the run's own user was redacted: %s", raw)
	}
	for _, kept := range []string{"account <n> of the team at <domain> also at login.coralogix.com", "00000000-0000-4000-8000-"} {
		if !strings.Contains(string(raw), kept) {
			t.Errorf("the redacted answer lost %q: %s", kept, raw)
		}
	}
	again := newUserFidelityRedactor(t, "run1").redact(live)
	againRaw := marshalUserFidelity(t, again)
	if !bytes.Equal(raw, againRaw) {
		t.Errorf("redaction is not deterministic:\n%s\n%s", raw, againRaw)
	}
}

// TestUserFidelityPageToken checks that a _TOKEN capture sends back the token text the API sent, and that
// the recording keeps an opaque token only as its JSON type.
func TestUserFidelityPageToken(t *testing.T) {
	for name, test := range map[string]struct {
		token    any
		capture  string
		recorded any
	}{
		"offset number": {float64(1), "1", float64(1)},
		"offset text":   {"1", "1", "1"},
		"opaque text":   {"eyJ0ZWFtIjo0NDU1NjY3fQ==", "eyJ0ZWFtIjo0NDU1NjY3fQ==", "<page-token>"},
		"no next page":  {nil, "", nil},
	} {
		t.Run(name, func(t *testing.T) {
			answer := map[string]any{"users": []any{}}
			if test.token != nil {
				answer["nextPageToken"] = test.token
			}
			if got := userFidelityCapture("NEXT_TOKEN", answer); got != test.capture {
				t.Errorf("captured %q, want %q", got, test.capture)
			}
			redacted := asObject(newUserFidelityRedactor(t, "run1").redact(answer))
			if got := redacted["nextPageToken"]; got != test.recorded {
				t.Errorf("recorded %v, want %v", got, test.recorded)
			}
		})
	}
}

func marshalUserFidelity(t *testing.T, value any) []byte {
	t.Helper()
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// TestUserFidelityRedactorDeniesUnknownKeys checks that the redactor stops on what it does not know,
// at any depth, and on a body that is not JSON.
func TestUserFidelityRedactorDeniesUnknownKeys(t *testing.T) {
	for name, answer := range map[string]any{
		"top level":     map[string]any{"users": []any{}, "memberDirectory": "x"},
		"in a user":     map[string]any{"users": []any{map[string]any{"userId": userFidelityUnknownID, "phoneNumber": "555"}}},
		"in a result":   map[string]any{"results": []any{map[string]any{"status": "CREATE_USER_STATUS_CREATED", "invitedBy": "x"}}},
		"snake case":    map[string]any{"users": []any{map[string]any{"user_id": "x"}}},
		"a raw body":    map[string]any{"raw": "<html>not json</html>"},
		"in a nested":   map[string]any{"users": []any{map[string]any{"accessType": map[string]any{"owner": "x"}}}},
		"error details": map[string]any{"code": float64(400), "message": "bad", "details": []any{map[string]any{"email": "x"}}},
	} {
		t.Run(name, func(t *testing.T) {
			redactor := newUserFidelityRedactor(t, "run1")
			var reasons []string
			redactor.fail = func(reason string) { reasons = append(reasons, reason) }
			redactor.redact(answer)
			if len(reasons) == 0 {
				t.Fatalf("the redactor kept an answer with a key it does not know: %v", answer)
			}
		})
	}

	known := map[string]any{"code": float64(404), "message": "Not Found: user not found", "users": []any{map[string]any{
		"userId": userFidelityUnknownID, "accessType": map[string]any{"permanentAccess": map[string]any{}},
	}}}
	redactor := newUserFidelityRedactor(t, "run1")
	redactor.fail = func(reason string) { t.Errorf("the redactor refused a known answer: %s", reason) }
	redactor.redact(known)

	t.Run("a body that is not JSON stops the recorder", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte("Jane.Doe@example.com 8123456"))
		}))
		defer srv.Close()
		_, _, err := sendUserFidelityRequestNoTest(srv.URL, "dry-run-key", http.MethodGet, "?username=x", nil)
		if err == nil {
			t.Fatal("a body that is not JSON was kept")
		}
		if strings.Contains(err.Error(), "Jane") || strings.Contains(err.Error(), "8123456") {
			t.Errorf("the error shows the body: %v", err)
		}
	})
}

// TestRecordUserFidelity writes real API answers for cases.json. The Users API has no delete, so the
// recorder leaves every user it creates inactive on the team. Run it only against a disposable team that has
// room for a few more members. Every create uses ONBOARDING_MODE_NO_INVITE, so no invitation is sent.
// Every search passes a username that only this run's users match, so the answers hold no other member.
//
// The region comes from exactly one of CORALOGIX_ENV (or CX_REGION) and CORALOGIX_DOMAIN, as for the
// provider. USER_FIDELITY_BASE_URL is for local use only: it points the recorder at a local fake for a
// dry run, and the recorder sends the API key to it. Never point it at a host you do not control.
func TestRecordUserFidelity(t *testing.T) {
	if os.Getenv("RECORD_USER_FIDELITY") != "1" {
		t.Skip("set RECORD_USER_FIDELITY=1 to record against a disposable tenant")
	}
	if os.Getenv("USER_FIDELITY_ALLOW_MUTATION") != "I_UNDERSTAND_THIS_LEAVES_INACTIVE_USERS" {
		t.Fatal("recording creates users that cannot be deleted; set USER_FIDELITY_ALLOW_MUTATION=I_UNDERSTAND_THIS_LEAVES_INACTIVE_USERS only for a disposable team")
	}
	apiKey := os.Getenv("CORALOGIX_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("CX_API_KEY")
	}
	if apiKey == "" {
		t.Fatal("CORALOGIX_API_KEY is required to record")
	}
	base, err := userFidelityBaseURL(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	memberCap, err := strconv.Atoi(os.Getenv("USER_FIDELITY_MEMBER_CAP"))
	if err != nil || memberCap <= 0 {
		t.Fatal("USER_FIDELITY_MEMBER_CAP is required to record: set it to the member cap of the team, so the recorder can check that its users fit")
	}
	recordUserFidelity(t, base, apiKey, filepath.Join(userGoldenDir, "fidelity", "recorded.json"), memberCap)
}

// userFidelityBaseURL returns the API URL to record against. USER_FIDELITY_BASE_URL wins, for a
// dry run against a local fake. Otherwise exactly one of the region and CORALOGIX_DOMAIN picks it.
func userFidelityBaseURL(getenv func(string) string) (string, error) {
	if base := getenv("USER_FIDELITY_BASE_URL"); base != "" {
		return base, nil
	}
	region := getenv("CORALOGIX_ENV")
	if region == "" {
		region = getenv("CX_REGION")
	}
	domain := getenv("CORALOGIX_DOMAIN")
	switch {
	case region != "" && domain != "":
		return "", errors.New("set only one of CORALOGIX_ENV (or CX_REGION) and CORALOGIX_DOMAIN")
	case domain != "":
		return cxsdkOpenapi.URLFromDomain(domain), nil
	case region == "":
		return "", errors.New("CORALOGIX_ENV, CX_REGION, or CORALOGIX_DOMAIN is required to record")
	}
	base, ok := cxsdkOpenapi.URLFromRegion(strings.ToLower(region))
	if !ok {
		return "", fmt.Errorf("unknown region %q", region)
	}
	return base, nil
}

func TestUserFidelityBaseURL(t *testing.T) {
	for name, test := range map[string]struct {
		env     map[string]string
		want    string
		wantErr bool
	}{
		"region":            {map[string]string{"CORALOGIX_ENV": "EU2"}, "https://api.eu2.coralogix.com/mgmt/openapi/5", false},
		"region alias":      {map[string]string{"CX_REGION": "us1"}, "https://api.us1.coralogix.com/mgmt/openapi/5", false},
		"domain":            {map[string]string{"CORALOGIX_DOMAIN": "eu2.coralogix.com"}, "https://api.eu2.coralogix.com/mgmt/openapi/5", false},
		"domain with api":   {map[string]string{"CORALOGIX_DOMAIN": "api.eu2.coralogix.com"}, "https://api.eu2.coralogix.com/mgmt/openapi/5", false},
		"local base url":    {map[string]string{"USER_FIDELITY_BASE_URL": "http://127.0.0.1:1", "CORALOGIX_ENV": "EU2"}, "http://127.0.0.1:1", false},
		"region and domain": {map[string]string{"CORALOGIX_ENV": "EU2", "CORALOGIX_DOMAIN": "eu2.coralogix.com"}, "", true},
		"neither":           {map[string]string{}, "", true},
		"unknown region":    {map[string]string{"CORALOGIX_ENV": "nowhere"}, "", true},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := userFidelityBaseURL(func(key string) string { return test.env[key] })
			if (err != nil) != test.wantErr || got != test.want {
				t.Errorf("got %q, %v; want %q, error %t", got, err, test.want, test.wantErr)
			}
		})
	}
}

func recordUserFidelity(t *testing.T, base, apiKey, outPath string, memberCap int) {
	t.Helper()
	cases := loadUserFidelityCases(t)
	if err := userFidelityCapacity(t, base, apiKey, userFidelityUsersPerRun(cases), memberCap); err != nil {
		t.Fatal(err)
	}
	run := newUserFidelityRunID(t)
	mutated := false
	t.Cleanup(func() {
		if !mutated {
			return
		}
		if err := cleanupUserFidelity(base, apiKey, run); err != nil {
			t.Errorf("could not deactivate the users of this recording: %v", err)
		}
	})
	ensureUserFidelityRunIsNew(t, base, apiKey, run)

	owned := map[string]struct{}{userFidelityUnknownID: {}}
	out := fidelityRecorded{}
	for ci, c := range cases.Cases {
		// Every case has its own users, because a user stays on the team after the case.
		caseRun := fmt.Sprintf("%s-%d", run, ci+1)
		vars := userFidelityVars(caseRun)
		redactor := newUserFidelityRedactor(t, caseRun)
		recordedCase := struct {
			Name  string `json:"name"`
			Steps []struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			} `json:"steps"`
		}{Name: c.Name}
		for _, step := range c.Steps {
			request := step
			request.Path = substString(step.Path, vars)
			if step.Body != nil {
				request.Body = substJSON(t, step.Body, vars)
			}
			if problem := userFidelityRequestProblem(request, run, owned); problem != "" {
				t.Fatal(problem)
			}
			if request.Method != http.MethodGet {
				// Arm the cleanup before a write: the server may commit it even when the
				// client fails to read the answer.
				mutated = true
			}
			status, body := sendUserFidelityRequest(t, base, apiKey, request)
			if status == http.StatusUnauthorized || status == http.StatusForbidden {
				t.Fatalf("the API answered %d; recording needs a management key with team-members permissions", status)
			}
			if request.Method == http.MethodPost && status == http.StatusOK {
				captureUserFidelityIDs(owned, body, run)
			}
			if step.Capture != "" && status == http.StatusOK {
				vars[step.Capture] = userFidelityCapture(step.Capture, body)
			}
			recordedCase.Steps = append(recordedCase.Steps, struct {
				Status   int `json:"status"`
				Response any `json:"response"`
			}{Status: status, Response: userFidelityAnswer(t, redactor, body, vars)})
		}
		out.Cases = append(out.Cases, recordedCase)
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if problems := userGoldenProblems(string(raw)); len(problems) != 0 {
		t.Fatalf("refusing to write a recording that holds member data: %v", problems)
	}
	if err := os.WriteFile(outPath, append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// ensureUserFidelityRunIsNew refuses to record when a user of this run id already exists.
func ensureUserFidelityRunIsNew(t *testing.T, base, apiKey, run string) {
	t.Helper()
	vars := userFidelityVars(run)
	status, body := sendUserFidelityRequest(t, base, apiKey, fidelityStep{Method: http.MethodGet, Path: "?page_size=1&username=" + url.QueryEscape(vars["PARTIAL"])})
	if status != http.StatusOK {
		t.Fatalf("preflight search answered %d; refusing to record against this team: %s", status, compact(body))
	}
	if users, _ := asObject(body)["users"].([]any); len(users) != 0 {
		t.Fatal("preflight found users of this run id; refusing to record")
	}
}

// userFidelityHeadroom is the number of member seats that a recording must leave free on the team.
const userFidelityHeadroom = 2

func loadUserFidelityCases(t *testing.T) fidelityCases {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(userGoldenDir, "fidelity", "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases fidelityCases
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

// userFidelityUsersPerRun counts the users that one recording creates: the distinct usernames that
// each case sends in a create. A create of a username that the case already made adds no user.
func userFidelityUsersPerRun(cases fidelityCases) int {
	count := 0
	for _, c := range cases.Cases {
		vars := userFidelityVars("count")
		usernames := map[string]struct{}{}
		for _, step := range c.Steps {
			if step.Method != http.MethodPost {
				continue
			}
			items, _ := step.Body.([]any)
			for _, item := range items {
				username, _ := asObject(asObject(item)["userTemplate"])["username"].(string)
				usernames[strings.ToLower(substString(username, vars))] = struct{}{}
			}
		}
		count += len(usernames)
	}
	return count
}

// userFidelityCapacity refuses a recording that would fill the team. The Users API has no delete, so
// every user that a recording creates keeps its seat. It reads only totalCount of two searches: the
// users that earlier recordings left, and all the members of the team. Neither answer is kept or
// recorded, which is why these searches do not go through userFidelityRequestProblem.
func userFidelityCapacity(t *testing.T, base, apiKey string, need, memberCap int) error {
	t.Helper()
	leftovers, err := userFidelityTotalCount(base, apiKey, "?page_size=1&username="+url.QueryEscape("tf-acc-user-"))
	if err != nil {
		return fmt.Errorf("count the users of earlier recordings: %w", err)
	}
	members, err := userFidelityTotalCount(base, apiKey, "?page_size=1")
	if err != nil {
		return fmt.Errorf("count the members of the team: %w", err)
	}
	t.Logf("the team has %d members, %d of them left by earlier recordings; this recording creates %d", members, leftovers, need)
	if members+need+userFidelityHeadroom > memberCap {
		return fmt.Errorf("refusing to record: %d members + %d new users + %d headroom exceed the member cap of %d; ask the tenant owner for room", members, need, userFidelityHeadroom, memberCap)
	}
	return nil
}

// userFidelityTotalCount sends a search and returns only its totalCount. The API leaves a zero count out.
func userFidelityTotalCount(base, apiKey, path string) (int, error) {
	status, body, err := sendUserFidelityRequestNoTest(base, apiKey, http.MethodGet, path, nil)
	if err != nil {
		return 0, err
	}
	if status != http.StatusOK {
		return 0, fmt.Errorf("the search answered %d", status)
	}
	root := asObject(body)
	switch count := root["totalCount"].(type) {
	case float64:
		return int(count), nil
	case string:
		return strconv.Atoi(count)
	case nil:
		if users, _ := root["users"].([]any); len(users) == 0 {
			return 0, nil
		}
	}
	return 0, errors.New("the search answered without a totalCount that the recorder can read")
}

// userFidelityRequestProblem is the safety net of the recorder. It names the problem with a request
// that could reach a user this run did not create, invite anyone, or read the member list of the
// team. It returns "" for a safe request.
func userFidelityRequestProblem(step fidelityStep, run string, owned map[string]struct{}) string {
	run = strings.ToLower(run)
	switch step.Method {
	case http.MethodGet:
		_, rawQuery, _ := strings.Cut(step.Path, "?")
		query, err := url.ParseQuery(rawQuery)
		if err != nil || !strings.Contains(strings.ToLower(query.Get("username")), run) {
			return "refusing a search that does not filter to this run's users: " + step.Path
		}
	case http.MethodPost:
		items, problem := userFidelityBatch(step)
		if problem != "" {
			return problem
		}
		for _, item := range items {
			username, _ := asObject(item["userTemplate"])["username"].(string)
			if item["onboardingMode"] != userNoInvite {
				return fmt.Sprintf("refusing a create without onboardingMode NO_INVITE for %q", username)
			}
			if !strings.Contains(strings.ToLower(username), run) {
				return fmt.Sprintf("refusing to create %q, which is not a user of this run", username)
			}
		}
	case http.MethodPut:
		items, problem := userFidelityBatch(step)
		if problem != "" {
			return problem
		}
		for _, item := range items {
			id, _ := item["userId"].(string)
			if _, ok := owned[id]; !ok {
				return fmt.Sprintf("refusing to update user %q, which this run did not create", id)
			}
		}
	default:
		return fmt.Sprintf("refusing a %s request: the recorder only searches, creates, and updates", step.Method)
	}
	return ""
}

func userFidelityBatch(step fidelityStep) ([]map[string]any, string) {
	items, ok := step.Body.([]any)
	if !ok {
		return nil, fmt.Sprintf("a %s request needs an array body", step.Method)
	}
	out := make([]map[string]any, len(items))
	for i, item := range items {
		out[i] = asObject(item)
	}
	return out, ""
}

// captureUserFidelityIDs remembers the users a create answered with, when they are this run's.
func captureUserFidelityIDs(owned map[string]struct{}, body any, run string) {
	results, _ := asObject(body)["results"].([]any)
	for _, raw := range results {
		result := asObject(raw)
		username, _ := result["username"].(string)
		id, _ := result["userId"].(string)
		if id != "" && strings.Contains(strings.ToLower(username), strings.ToLower(run)) {
			owned[id] = struct{}{}
		}
	}
}

func sendUserFidelityRequest(t *testing.T, base, apiKey string, step fidelityStep) (int, any) {
	t.Helper()
	status, body, err := sendUserFidelityRequestNoTest(base, apiKey, step.Method, step.Path, step.Body)
	if err != nil {
		t.Fatal(err)
	}
	return status, body
}

func sendUserFidelityRequestNoTest(base, apiKey, method, path string, body any) (int, any, error) {
	target := strings.TrimRight(base, "/") + userBasePath + path
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, target, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	var parsed any
	if len(bytes.TrimSpace(raw)) == 0 {
		parsed = map[string]any{}
	} else if err := json.Unmarshal(raw, &parsed); err != nil {
		// The text could hold anything, so it is neither kept nor shown.
		return resp.StatusCode, nil, fmt.Errorf("the API answered %d with a body that is not JSON; refusing to keep it", resp.StatusCode)
	}
	return resp.StatusCode, parsed, nil
}

// cleanupUserFidelity deactivates every user of this run. The run id is random and part of every
// username the recorder creates, so a search for it lists only this run's users.
func cleanupUserFidelity(base, apiKey, run string) error {
	prefix := userFidelityVars(run)["PARTIAL"]
	status, body, err := sendUserFidelityRequestNoTest(base, apiKey, http.MethodGet, "?page_size=100&username="+url.QueryEscape(prefix), nil)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("cleanup search answered %d: %s", status, compact(body))
	}
	users, _ := asObject(body)["users"].([]any)
	for _, raw := range users {
		user := asObject(raw)
		username, _ := user["username"].(string)
		id, _ := user["userId"].(string)
		if id == "" || !strings.HasPrefix(strings.ToLower(username), strings.ToLower(prefix)) {
			continue
		}
		if user["status"] == "USER_STATUS_INACTIVE" {
			continue
		}
		template := map[string]any{
			"firstName": user["firstName"], "lastName": user["lastName"], "status": "USER_STATUS_INACTIVE",
		}
		for _, key := range []string{"allowedLoginMode", "accessType"} {
			if value, ok := user[key]; ok {
				template[key] = value
			}
		}
		status, body, err := sendUserFidelityRequestNoTest(base, apiKey, http.MethodPut, "", []any{
			map[string]any{"userId": id, "userTemplate": template},
		})
		if err != nil {
			return err
		}
		if status != http.StatusOK {
			return fmt.Errorf("cleanup could not deactivate %s: %d %s", username, status, compact(body))
		}
	}
	return nil
}

func TestUserRecorderCleanupDeactivatesOnlyThisRun(t *testing.T) {
	fake := newUserFidelityFake()
	own := fake.seed(map[string]any{"username": "tf-acc-user-cleanup-a@coralogix.com", "firstName": "Test"})
	foreign := fake.seed(map[string]any{"username": "tf-acc-user-other-run@coralogix.com"})
	srv := httptest.NewServer(fake)
	defer srv.Close()

	if err := cleanupUserFidelity(srv.URL, "dry-run-key", "cleanup"); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
	if got := fake.find(own)["status"]; got != "USER_STATUS_INACTIVE" {
		t.Errorf("this run's user is %v, want inactive", got)
	}
	if got := fake.find(own)["firstName"]; got != "Test" {
		t.Errorf("cleanup changed the name of this run's user to %v", got)
	}
	if got := fake.find(foreign)["status"]; got != "USER_STATUS_ACTIVE" {
		t.Errorf("cleanup touched a user of another run: %v", got)
	}
}

func TestUserRecorderRefusesUnsafeRequests(t *testing.T) {
	owned := map[string]struct{}{userFidelityUnknownID: {}, "00000000-0000-4000-8000-000000000001": {}}
	safeCreate := []any{map[string]any{
		"onboardingMode": userNoInvite,
		"userTemplate":   map[string]any{"username": "tf-acc-user-run1-a@coralogix.com"},
	}}
	for name, test := range map[string]struct {
		step fidelityStep
		safe bool
	}{
		"search for this run":            {fidelityStep{Method: http.MethodGet, Path: "?username=tf-acc-user-run1-"}, true},
		"search of the whole team":       {fidelityStep{Method: http.MethodGet, Path: "?page_size=100"}, false},
		"member count of the team":       {fidelityStep{Method: http.MethodGet, Path: "?page_size=1"}, false},
		"count of earlier recordings":    {fidelityStep{Method: http.MethodGet, Path: "?page_size=1&username=tf-acc-user-"}, false},
		"missing route for this run":     {fidelityStep{Method: http.MethodGet, Path: "/no-such-route?page_size=1&username=run1"}, true},
		"search for another run":         {fidelityStep{Method: http.MethodGet, Path: "?username=tf-acc-user-run2-"}, false},
		"create of this run":             {fidelityStep{Method: http.MethodPost, Body: safeCreate}, true},
		"create with an invitation":      {fidelityStep{Method: http.MethodPost, Body: []any{map[string]any{"onboardingMode": "ONBOARDING_MODE_INVITE_LINK", "userTemplate": map[string]any{"username": "tf-acc-user-run1-a@coralogix.com"}}}}, false},
		"create without onboarding mode": {fidelityStep{Method: http.MethodPost, Body: []any{map[string]any{"userTemplate": map[string]any{"username": "tf-acc-user-run1-a@coralogix.com"}}}}, false},
		"create of another user":         {fidelityStep{Method: http.MethodPost, Body: []any{map[string]any{"onboardingMode": userNoInvite, "userTemplate": map[string]any{"username": "tf-acc-user-run2-a@coralogix.com"}}}}, false},
		"update of an owned user":        {fidelityStep{Method: http.MethodPut, Body: []any{map[string]any{"userId": "00000000-0000-4000-8000-000000000001"}}}, true},
		"update of the unknown id":       {fidelityStep{Method: http.MethodPut, Body: []any{map[string]any{"userId": userFidelityUnknownID}}}, true},
		"update of a user not owned":     {fidelityStep{Method: http.MethodPut, Body: []any{map[string]any{"userId": "00000000-0000-4000-8000-000000000009"}}}, false},
		"delete":                         {fidelityStep{Method: http.MethodDelete}, false},
	} {
		t.Run(name, func(t *testing.T) {
			if problem := userFidelityRequestProblem(test.step, "run1", owned); (problem == "") != test.safe {
				t.Errorf("request accepted = %t, want %t (%s)", problem == "", test.safe, problem)
			}
		})
	}
}

// TestUserRecorderDryRun exercises the live recorder pipeline against the fake without credentials.
func TestUserRecorderDryRun(t *testing.T) {
	fake := newUserFidelityFake()
	outPath := filepath.Join(t.TempDir(), "recorded.json")
	// The local server stands in for the region API URL.
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)
	cases := loadUserFidelityCases(t)
	need := userFidelityUsersPerRun(cases)
	recordUserFidelity(t, srv.URL, "dry-run-key", outPath, need+userFidelityHeadroom)
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("recorder did not write output: %v", err)
	}
	var recorded fidelityRecorded
	if err := json.Unmarshal(raw, &recorded); err != nil {
		t.Fatal(err)
	}
	for ci, c := range cases.Cases {
		replay := newUserFidelityFake()
		vars := userFidelityVars("dry-run")
		redactor := newUserFidelityRedactor(t, "dry-run")
		for si, step := range c.Steps {
			status, body := runUserFidelityStep(t, replay, step, vars, redactor)
			want := recorded.Cases[ci].Steps[si]
			compareFidelityStep(t, fmt.Sprintf("%s step %d", c.Name, si+1), status, body, want.Status, want.Response)
		}
	}
	// The recording is redacted: it holds only placeholder ids and the synthetic emails.
	if problems := userGoldenProblems(string(raw)); len(problems) != 0 {
		t.Errorf("the dry-run recording holds member data: %v", problems)
	}
	// Nothing is left active: the recorder's users are inactive after the run.
	for _, user := range fake.users {
		if user["status"] != "USER_STATUS_INACTIVE" {
			t.Errorf("user %v is still %v after the recording", user["username"], user["status"])
		}
	}
	// The member cap check counts the users that the run creates.
	if len(fake.users) != need {
		t.Errorf("the dry run created %d users, but the member cap check counted %d", len(fake.users), need)
	}
}

// TestUserRecorderRefusesAFullTeam checks that the recorder stops before its first write when its
// users and the headroom do not fit under the member cap.
func TestUserRecorderRefusesAFullTeam(t *testing.T) {
	need := userFidelityUsersPerRun(loadUserFidelityCases(t))
	for name, test := range map[string]struct {
		members int
		fits    bool
	}{
		"empty team":        {0, true},
		"room for the run":  {3, true},
		"one seat short":    {4, false},
		"at the member cap": {need + userFidelityHeadroom + 3, false},
	} {
		t.Run(name, func(t *testing.T) {
			fake := newUserFidelityFake()
			for i := range test.members {
				fake.seed(map[string]any{"username": fmt.Sprintf("member-%d@coralogix.com", i)})
			}
			srv := httptest.NewServer(fake)
			defer srv.Close()

			err := userFidelityCapacity(t, srv.URL, "dry-run-key", need, need+userFidelityHeadroom+3)
			if (err == nil) != test.fits {
				t.Fatalf("recording allowed = %t, want %t (%v)", err == nil, test.fits, err)
			}
			for _, request := range fake.takeRequests() {
				if request.Method != http.MethodGet {
					t.Errorf("the member cap check sent %s %s", request.Method, request.Path)
				}
			}
		})
	}
}
