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
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/aaa"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const (
	userTypeName  = "coralogix_user"
	userGoldenDir = "testdata/user_baseline"
	userBasePath  = "/aaa/users/v2"
)

// These tests record the handwritten coralogix_user resource and paired data source. The data
// source is generated in stack 3, so its read behavior is baselined here too: the data source schema
// golden comes from dataSourceBaselines, as for the other resources.
// After migration, the golden diff should show only message wording or explicitly accepted bugs.
//
// The scenarios record what the resource does today, including behavior that a later change
// fixes: Update removes the resource from state when the user is gone. The golden records it as
// it is.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestUserBaseline
func newUserHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, userTypeName, newUserFake(t))
}

func TestUserBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	aaa.NewUserResource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, userGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func TestUserBaselineScenarios(t *testing.T) {
	for name, run := range userScenarios {
		t.Run(name, func(t *testing.T) {
			h := newUserHarness(t)
			run(h)
			canonicalizeUserStepValues(h.steps)
			checkGoldenAt(t, userGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

// TestUserBaselineGoldensAreSynthetic fails when a committed golden or fidelity file holds what
// must never be committed: an email outside the synthetic shape, or an id that is not a
// placeholder. The users API lists real team members, so a recording is checked before it is committed.
func TestUserBaselineGoldensAreSynthetic(t *testing.T) {
	files := 0
	err := filepath.WalkDir(userGoldenDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files++
		for _, problem := range userGoldenProblems(string(raw)) {
			t.Errorf("%s: %s", path, problem)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 {
		t.Fatalf("no files in %s: run with UPDATE_GOLDEN=1", userGoldenDir)
	}
}

var (
	userAnyEmailPattern       = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+`)
	userSyntheticEmailPattern = regexp.MustCompile(`^(?i:tf-acc-user-[a-z0-9-]*@coralogix\.com)$`)
	userAnyUUIDPattern        = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
	userPlaceholderUUID       = regexp.MustCompile(`^00000000-0000-4000-8000-[0-9]{12}$`)
	// userNumberFieldPattern finds the numeric fields that hold tenant ids and counts, in the camel case
	// of the API and the snake case of a Terraform-side or SDK-side copy.
	userNumberFieldPattern = regexp.MustCompile(`"(?:userAccountId|user_account_id|teamId|team_id|companyId|company_id|accountId|account_id|totalCount|total_count)"\s*:\s*([0-9]+)`)
	// userNumberListPattern finds the arrays of tenant ids: the account ids of an update and the group ids.
	userNumberListPattern     = regexp.MustCompile(`"(?:userAccountIds|user_account_ids|groupIds|group_ids)"\s*:\s*\[([^\]]*)\]`)
	userDigitsPattern         = regexp.MustCompile(`[0-9]+`)
	userJSONStringPattern     = regexp.MustCompile(`"(?:[^"\\]|\\.)*"`)
	userPlaceholderIDInString = regexp.MustCompile(`00000000-0000-4000-8000-[0-9]{12}`)
	userNumberPlaceholders    = regexp.MustCompile(`^[0-9]{1,5}$`)
	userLongDigitRun          = regexp.MustCompile(`[0-9]{5,}`)
)

func userGoldenProblems(text string) []string {
	var problems []string
	for _, email := range userAnyEmailPattern.FindAllString(text, -1) {
		if !userSyntheticEmailPattern.MatchString(email) {
			problems = append(problems, "email outside the synthetic tf-acc-user-<id>@coralogix.com shape: "+email)
		}
	}
	for _, id := range userAnyUUIDPattern.FindAllString(text, -1) {
		if !userPlaceholderUUID.MatchString(id) {
			problems = append(problems, "UUID that is not a placeholder: "+id)
		}
	}
	for _, match := range userNumberFieldPattern.FindAllStringSubmatch(text, -1) {
		if !userNumberPlaceholders.MatchString(match[1]) {
			problems = append(problems, "numeric id that is not a placeholder: "+match[0])
		}
	}
	for _, match := range userNumberListPattern.FindAllStringSubmatch(text, -1) {
		for _, number := range userDigitsPattern.FindAllString(match[1], -1) {
			if !userNumberPlaceholders.MatchString(number) {
				problems = append(problems, "numeric id in a list that is not a placeholder: "+match[0])
				break
			}
		}
	}
	for _, quoted := range userJSONStringPattern.FindAllString(text, -1) {
		// What is left after the placeholder ids and the synthetic emails, which can hold long digit runs.
		rest := userPlaceholderIDInString.ReplaceAllString(quoted, "")
		rest = userAnyEmailPattern.ReplaceAllStringFunc(rest, func(email string) string {
			if userSyntheticEmailPattern.MatchString(email) {
				return ""
			}
			return email
		})
		if run := userLongDigitRun.FindString(rest); run != "" {
			problems = append(problems, "string with a run of five or more digits: "+quoted)
		}
	}
	return problems
}

func TestUserGoldenScanFlagsRealLookingData(t *testing.T) {
	clean := `{"username": "tf-acc-user-1@coralogix.com", "userId": "00000000-0000-4000-8000-000000000001", "userAccountId": 1001,
		"userAccountIds": [1001, 1002], "groupIds": [1, 2], "totalCount": 2, "message": "account <n> of <domain>"}`
	if problems := userGoldenProblems(clean); len(problems) != 0 {
		t.Fatalf("a synthetic record was flagged: %v", problems)
	}
	dirty := `{"username": "jane.doe@example.com", "userId": "3f1c1e0a-52a1-4b8e-9b0e-0c6f4a1d2e77", "userAccountId": 93817265, "teamId": 4455667}`
	if got := len(userGoldenProblems(dirty)); got != 4 {
		t.Fatalf("flagged %d problems, want 4", got)
	}
	for name, snippet := range map[string]string{
		"account id list":      `{"userAccountIds": [1001, 93817265]}`,
		"group id list":        `{"groupIds": [481516]}`,
		"snake case account":   `{"user_account_id": 93817265}`,
		"snake case team":      `{"team_id": 4455667}`,
		"snake case id lists":  `{"user_account_ids": [93817265], "group_ids": [481516]}`,
		"total count":          `{"totalCount": 8123456}`,
		"digit run in message": `{"message": "account 77889900 is full"}`,
		"digit run in string":  `{"username": "tf-acc-user-1@coralogix.com", "note": "ticket 123456"}`,
		"unknown domain":       `{"message": "user a@corp.example.org is not allowed"}`,
	} {
		if len(userGoldenProblems(snippet)) == 0 {
			t.Errorf("%s was not flagged: %s", name, snippet)
		}
	}
}

const (
	userA = "tf-acc-user-1@coralogix.com"
	userB = "tf-acc-user-2@coralogix.com"
	// userPartialMatch contains userA, so a search for userA lists both users and the code has to
	// filter to the exact one.
	userPartialMatch = "tf-acc-user-x-tf-acc-user-1@coralogix.com"
)

var (
	userTestName    = j{"given_name": "Test", "family_name": "User"}
	userRenamedName = j{"given_name": "Renamed", "family_name": "Person"}
	userLoginModes  = l{"ALLOWED_LOGIN_MODE_SSO", "ALLOWED_LOGIN_MODE_LOCAL"}
	userAccessType  = j{"accessType": "permanent", "permanentAccess": j{}}
	userOutside     = j{"allowedLoginMode": userLoginModes, "accessType": userAccessType}
)

// userConfig is the config of a user resource: the user name and the optional fields that a scenario sets.
func userConfig(userName string, fields j) j {
	config := j{"user_name": userName}
	for key, value := range fields {
		config[key] = value
	}
	return config
}

// userCreateAnswers makes the API answer every create with one per-user status.
func userCreateAnswers(status, message string) func(h *grHarness) {
	return func(h *grHarness) {
		h.users().createResult = status
		h.users().createMessage = message
		h.Apply("create: API answers "+status, h.null(), userConfig(userA, j{"name": userTestName}))
	}
}

// userCreateShape makes a create that succeeds answer in a shape that the resource cannot use.
func userCreateShape(mode string) func(h *grHarness) {
	return func(h *grHarness) {
		h.users().createMode = mode
		h.Apply("create: response shape "+mode, h.null(), userConfig(userA, j{"name": userTestName}))
	}
}

var userScenarios = map[string]func(h *grHarness){
	"full-lifecycle": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		s = h.RefreshPlan("refresh and replan after create", s)
		s = h.Apply("update name", s, userConfig(userA, j{"name": userRenamedName}))
		s = h.RefreshPlan("refresh and replan after the name update", s)
		s = h.Apply("update active to false", s, userConfig(userA, j{"name": userRenamedName, "active": false}))
		s = h.RefreshPlan("refresh and replan after deactivation", s)
		s = h.Apply("update active to true", s, userConfig(userA, j{"name": userRenamedName, "active": true}))
		s = h.RefreshPlan("refresh and replan after reactivation", s)
		h.Destroy("destroy", s)
	},
	"create-inactive": func(h *grHarness) {
		s := h.Apply("create with active = false", h.null(), userConfig(userA, j{"name": userTestName, "active": false}))
		h.RefreshPlan("refresh and replan", s)
	},
	"create-without-name": func(h *grHarness) {
		s := h.Apply("create with name omitted", h.null(), userConfig(userA, j{}))
		h.RefreshPlan("refresh and replan", s)
	},
	"create-given-name-only": func(h *grHarness) {
		s := h.Apply("create with only given_name", h.null(), userConfig(userA, j{"name": j{"given_name": "Test"}}))
		h.RefreshPlan("refresh and replan", s)
	},
	"create-empty-given-name": func(h *grHarness) {
		s := h.Apply("create with given_name = \"\"", h.null(), userConfig(userA, j{"name": j{"given_name": ""}}))
		h.RefreshPlan("refresh and replan", s)
	},
	"name-removed-on-update": func(h *grHarness) {
		s := h.Apply("create with name", h.null(), userConfig(userA, j{"name": userTestName}))
		s = h.Apply("remove name: Optional+Computed keeps the prior value", s, userConfig(userA, j{}))
		s = h.Apply("remove name and deactivate: the update sends the prior name", s, userConfig(userA, j{"active": false}))
		h.RefreshPlan("refresh and replan", s)
	},
	"active-removed-on-update": func(h *grHarness) {
		s := h.Apply("create with active = false", h.null(), userConfig(userA, j{"name": userTestName, "active": false}))
		s = h.Apply("remove active: the default reactivates the user", s, userConfig(userA, j{"name": userTestName}))
		h.RefreshPlan("refresh and replan", s)
	},
	"out-of-band-deactivation": func(h *grHarness) {
		config := userConfig(userA, j{"name": userTestName})
		s := h.Apply("create", h.null(), config)
		h.users().change(idOf(s), j{"status": "USER_STATUS_INACTIVE"})
		s = h.RefreshPlan("refresh after the user was deactivated outside Terraform", s)
		s = h.Apply("apply the config again: the user is reactivated", s, config)
		h.RefreshPlan("refresh and replan", s)
	},
	"destroy-inactive-user": func(h *grHarness) {
		s := h.Apply("create with active = false", h.null(), userConfig(userA, j{"name": userTestName, "active": false}))
		h.Destroy("destroy: the user is already inactive", s)
	},
	"destroy-after-outside-deactivation": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		h.users().change(idOf(s), j{"status": "USER_STATUS_INACTIVE"})
		h.Destroy("destroy without a refresh: the user was deactivated outside Terraform", s)
	},
	"login-modes-survive": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		h.users().change(idOf(s), userOutside)
		s = h.Refresh("refresh stores the login modes and the access type", s)
		s = h.Apply("rename: the PUT sends the login modes back", s, userConfig(userA, j{"name": userRenamedName}))
		s = h.Apply("deactivate: the PUT sends the login modes back", s, userConfig(userA, j{"name": userRenamedName, "active": false}))
		h.Destroy("destroy: the PUT sends the login modes back", s)
	},
	"echo-fallback-on-update": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		h.users().change(idOf(s), userOutside)
		h.ForgetPrivateState()
		s = h.Apply("update with no private state: a search finds the login modes", s, userConfig(userA, j{"name": userRenamedName}))
		h.RefreshPlan("refresh and replan", s)
	},
	"echo-fallback-on-destroy": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		h.users().change(idOf(s), userOutside)
		h.ForgetPrivateState()
		h.Destroy("destroy with no private state: a search finds the login modes", s)
	},
	"user-name-case-folded-by-backend": func(h *grHarness) {
		h.users().foldCase = true
		config := userConfig("TF-ACC-USER-1@Coralogix.com", j{"name": userTestName})
		s := h.Apply("create: the state keeps the case of the config", h.null(), config)
		s = h.RefreshPlan("refresh writes the case of the backend, and the replan stays empty", s)
		h.Apply("apply the config again", s, config)
	},
	"user-name-case-changed-outside": func(h *grHarness) {
		config := userConfig(userA, j{"name": userTestName})
		s := h.Apply("create", h.null(), config)
		h.users().change(idOf(s), j{"username": "TF-ACC-USER-1@CORALOGIX.COM"})
		s = h.RefreshPlan("refresh rewrites user_name to the case of the backend, and the replan stays empty", s)
		h.Apply("apply the config again", s, config)
	},
	"user-name-case-only-change": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		s = h.Apply("change only the case of user_name: nothing is planned", s, userConfig("TF-ACC-USER-1@Coralogix.com", j{"name": userTestName}))
		h.RefreshPlan("refresh and replan", s)
	},
	"user-name-rename-rejected": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		s = h.Apply("rename user_name: Update rejects it", s, userConfig(userB, j{"name": userTestName}))
		h.RefreshPlan("refresh keeps the user", s)
	},
	"user-name-changed-outside": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		h.users().change(idOf(s), j{"username": userB})
		// The search for the stored username finds nothing, so a scan of the whole team finds the id.
		h.RefreshPlan("refresh after the username changed outside Terraform", s)
	},
	"user-name-changed-outside-multi-page": func(h *grHarness) {
		h.users().pageLimit = 2
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		for _, name := range []string{"tf-acc-user-3@coralogix.com", "tf-acc-user-4@coralogix.com", "tf-acc-user-5@coralogix.com", "tf-acc-user-6@coralogix.com"} {
			h.users().seed(j{"username": name})
		}
		h.users().change(idOf(s), j{"username": userB})
		// The search for the stored username misses, so the scan of the whole team reads every page.
		h.RefreshPlan("refresh: the username search misses and the scan reads three pages", s)
	},
	"read-user-gone": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		h.users().forget(idOf(s))
		h.RefreshPlan("refresh: the user is gone", s)
	},
	"update-user-gone": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		h.users().forget(idOf(s))
		// Update removes the resource from state, which Terraform core rejects. The golden records it.
		h.Apply("update: the PUT answers 404", s, userConfig(userA, j{"name": userRenamedName}))
	},
	"update-user-gone-without-echo": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		h.users().forget(idOf(s))
		h.ForgetPrivateState()
		h.Apply("update with no private state: the search finds no user", s, userConfig(userA, j{"name": userRenamedName}))
	},
	"delete-user-gone": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		h.users().forget(idOf(s))
		h.Destroy("destroy: the PUT answers 404", s)
	},
	"delete-user-gone-without-echo": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		h.users().forget(idOf(s))
		h.ForgetPrivateState()
		h.Destroy("destroy with no private state: the search finds no user", s)
	},
	"read-route-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		// A 404 on the route itself says nothing about the user, so it is an error.
		h.users().searchStatus = 404
		h.Refresh("refresh: the search route answers 404", s)
	},
	"search-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		h.users().searchStatus = 500
		h.Refresh("refresh: the search answers 500", s)
	},
	"create-error": func(h *grHarness) {
		h.users().createStatus = 500
		h.Apply("create: the API answers 500", h.null(), userConfig(userA, j{"name": userTestName}))
	},
	"update-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		h.users().updateStatus = 500
		h.Apply("update: the PUT answers 500", s, userConfig(userA, j{"name": userRenamedName}))
	},
	"update-search-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		h.ForgetPrivateState()
		h.users().searchStatus = 500
		h.Apply("update with no private state: the search answers 500", s, userConfig(userA, j{"name": userRenamedName}))
	},
	"update-route-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		// A 404 on the route itself, "Not Found: Not Found", says nothing about the user. It is an error:
		// the resource is not removed from state.
		h.users().updateStatus = 404
		h.Apply("update: the PUT route answers 404", s, userConfig(userA, j{"name": userRenamedName}))
	},
	"delete-route-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		// Delete is a PUT that deactivates. A 404 on the route is an error, not a user that is gone.
		h.users().updateStatus = 404
		h.Destroy("destroy: the PUT route answers 404", s)
	},
	"delete-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		h.users().updateStatus = 500
		h.Destroy("destroy: the PUT answers 500", s)
	},
	"update-without-user": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		// The PUT answers 200 but not with the user, so the write may or may not have happened.
		h.users().updateOmitUsers = true
		h.Apply("update: the PUT answers 200 without the user", s, userConfig(userA, j{"name": userRenamedName}))
	},
	"create-already-exists": func(h *grHarness) {
		h.users().seed(j{"username": userA, "firstName": "Existing"})
		h.Apply("create: the user already exists", h.null(), userConfig(userA, j{"name": userTestName}))
	},
	"create-invited":                userCreateAnswers("CREATE_USER_STATUS_INVITED", ""),
	"create-already-invited":        userCreateAnswers("CREATE_USER_STATUS_ALREADY_INVITED", ""),
	"create-domain-not-allowed":     userCreateAnswers("CREATE_USER_STATUS_DOMAIN_NOT_ALLOWED", "domain is not allowed"),
	"create-failed":                 userCreateAnswers("CREATE_USER_STATUS_FAILED", "synthetic failure message"),
	"create-unspecified-status":     userCreateAnswers("CREATE_USER_STATUS_UNSPECIFIED", ""),
	"create-unknown-status":         userCreateAnswers("CREATE_USER_STATUS_FUTURE_VALUE", ""),
	"create-result-without-user-id": userCreateShape("no-user-id"),
	"create-no-result":              userCreateShape("no-result"),
	"create-result-for-other-user":  userCreateShape("other-username"),
	"groups-non-empty": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		h.users().change(idOf(s), j{"groupIds": l{float64(22), float64(11)}})
		h.RefreshPlan("refresh shows the group ids as strings", s)
	},
	"team-id-never-sent": func(h *grHarness) {
		// The fake fails the test when any request carries a team id: the team comes from the API key.
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		s = h.RefreshPlan("refresh and replan", s)
		s = h.Apply("update", s, userConfig(userA, j{"name": userRenamedName}))
		h.Destroy("destroy", s)
	},
	"user-name-unknown": func(h *grHarness) {
		h.Plan("plan with user_name unknown at validate", h.null(), j{"user_name": unknownJSON{}})
	},
	"name-unknown": func(h *grHarness) {
		s := h.Apply("create", h.null(), userConfig(userA, j{"name": userTestName}))
		h.Plan("plan with name unknown", s, userConfig(userA, j{"name": unknownJSON{}}))
	},
	"name-null-kept-by-use-state-for-unknown": func(h *grHarness) {
		// The user has no name, so the prior value in state is null. UseStateForUnknown keeps that
		// null, and name stays null in the plan, not unknown.
		s := h.Apply("create with name omitted", h.null(), userConfig(userA, j{}))
		s = h.Apply("update active with name omitted: name stays null because UseStateForUnknown keeps the null prior value", s, userConfig(userA, j{"active": false}))
		h.RefreshPlan("refresh and replan", s)
	},
	"import-by-id": func(h *grHarness) {
		id := h.users().seed(j{"username": userA, "firstName": "Test", "lastName": "User"})
		h.users().seed(j{"username": userB})
		s := h.Import("import by user id", id)
		h.Apply("plan matching config", s, userConfig(userA, j{"name": userTestName}))
	},
	"import-by-email": func(h *grHarness) {
		h.users().seed(j{"username": userB})
		h.users().seed(j{"username": userA, "firstName": "Test", "lastName": "User"})
		s := h.Import("import by email", userA)
		h.Apply("plan matching config", s, userConfig(userA, j{"name": userTestName}))
	},
	"import-by-email-different-case": func(h *grHarness) {
		h.users().seed(j{"username": userA, "firstName": "Test", "lastName": "User"})
		s := h.Import("import by email in another letter case", "TF-ACC-USER-1@CORALOGIX.COM")
		h.Apply("plan matching config", s, userConfig(userA, j{"name": userTestName}))
	},
	"import-by-email-partial-match": func(h *grHarness) {
		// The search matches part of a username, so the code filters to the exact one.
		h.users().seed(j{"username": userPartialMatch})
		h.users().seed(j{"username": userA, "firstName": "Test", "lastName": "User"})
		s := h.Import("import by email: the search lists two users and one is exact", userA)
		h.Apply("plan matching config", s, userConfig(userA, j{"name": userTestName}))
	},
	"import-email-not-found": func(h *grHarness) {
		h.users().seed(j{"username": userB})
		h.Import("import by an email that no user has", userA)
	},
	"import-email-ambiguous": func(h *grHarness) {
		h.users().seed(j{"username": userA})
		h.users().seed(j{"username": "TF-ACC-USER-1@coralogix.com"})
		h.Import("import by an email that two users have in different letter case", userA)
	},
	"import-id-not-found": func(h *grHarness) {
		h.users().seed(j{"username": userB})
		h.Import("import by a user id that no user has", missingID)
	},
	"import-status-unspecified": func(h *grHarness) {
		h.users().seed(j{"username": userA, "status": "USER_STATUS_UNSPECIFIED"})
		h.Import("import a user whose status is unspecified", userA)
	},
	"import-by-id-multi-page": func(h *grHarness) {
		h.users().pageLimit = 2
		for _, name := range []string{"tf-acc-user-3@coralogix.com", "tf-acc-user-4@coralogix.com", "tf-acc-user-5@coralogix.com", "tf-acc-user-6@coralogix.com"} {
			h.users().seed(j{"username": name})
		}
		id := h.users().seed(j{"username": userA, "firstName": "Test", "lastName": "User"})
		h.Import("import by user id: the scan reads three pages", id)
	},
	"import-by-id-stuck-page-token": func(h *grHarness) {
		h.users().pageLimit = 2
		h.users().stuckPageToken = 2
		for _, name := range []string{"tf-acc-user-3@coralogix.com", "tf-acc-user-4@coralogix.com", "tf-acc-user-5@coralogix.com", "tf-acc-user-6@coralogix.com"} {
			h.users().seed(j{"username": name})
		}
		id := h.users().seed(j{"username": userA})
		h.Import("import by user id: the page token stops advancing, so the scan stops", id)
	},
}

var userDataSourceScenarios = map[string]func(h *grHarness){
	"read-by-id": func(h *grHarness) {
		h.users().seed(j{"username": userB})
		id := h.users().seed(j{"username": userA, "firstName": "Test", "lastName": "User"})
		h.ReadDataSource("read by id", j{"id": id})
	},
	"read-by-user-name": func(h *grHarness) {
		h.users().seed(j{"username": userB})
		h.users().seed(j{"username": userA, "firstName": "Test", "lastName": "User"})
		h.ReadDataSource("read by user_name", j{"user_name": userA})
	},
	"read-by-user-name-different-case": func(h *grHarness) {
		h.users().seed(j{"username": userA, "firstName": "Test", "lastName": "User"})
		h.ReadDataSource("read by user_name in another letter case: the state keeps the configured case", j{"user_name": "TF-ACC-USER-1@CORALOGIX.COM"})
	},
	"read-by-user-name-partial-match": func(h *grHarness) {
		h.users().seed(j{"username": userPartialMatch})
		h.users().seed(j{"username": userA, "firstName": "Test", "lastName": "User"})
		h.ReadDataSource("read by user_name: the search lists two users and one is exact", j{"user_name": userA})
	},
	"read-with-groups": func(h *grHarness) {
		id := h.users().seed(j{"username": userA, "groupIds": l{float64(22), float64(11)}})
		h.ReadDataSource("read a user with groups", j{"id": id})
	},
	"read-inactive-user": func(h *grHarness) {
		id := h.users().seed(j{"username": userA, "status": "USER_STATUS_INACTIVE"})
		h.ReadDataSource("read an inactive user", j{"id": id})
	},
	"read-unspecified-status": func(h *grHarness) {
		id := h.users().seed(j{"username": userA, "status": "USER_STATUS_UNSPECIFIED"})
		h.ReadDataSource("read a user whose status is unspecified", j{"id": id})
	},
	"read-by-id-multi-page": func(h *grHarness) {
		h.users().pageLimit = 2
		for _, name := range []string{"tf-acc-user-3@coralogix.com", "tf-acc-user-4@coralogix.com", "tf-acc-user-5@coralogix.com", "tf-acc-user-6@coralogix.com"} {
			h.users().seed(j{"username": name})
		}
		id := h.users().seed(j{"username": userA})
		h.ReadDataSource("read by id: the scan reads three pages", j{"id": id})
	},
	"id-and-user-name": func(h *grHarness) {
		id := h.users().seed(j{"username": userA})
		h.ReadDataSource("config sets id and user_name", j{"id": id, "user_name": userA})
	},
	"no-id-or-user-name": func(h *grHarness) {
		h.ReadDataSource("config sets neither id nor user_name", j{})
	},
	"empty-id": func(h *grHarness) {
		h.ReadDataSource("config sets id to an empty string", j{"id": ""})
	},
	"empty-user-name": func(h *grHarness) {
		h.ReadDataSource("config sets user_name to an empty string", j{"user_name": ""})
	},
	"not-found": func(h *grHarness) {
		h.users().seed(j{"username": userB})
		h.ReadDataSource("read an id that does not exist", j{"id": missingID})
	},
	"user-name-not-found": func(h *grHarness) {
		h.users().seed(j{"username": userB})
		h.ReadDataSource("read a user_name that does not exist", j{"user_name": userA})
	},
	"multiple-matches": func(h *grHarness) {
		h.users().seed(j{"username": userA})
		h.users().seed(j{"username": "TF-ACC-USER-1@coralogix.com"})
		h.ReadDataSource("read a user_name that two users have in different letter case", j{"user_name": userA})
	},
	"search-error": func(h *grHarness) {
		h.users().searchStatus = 500
		h.ReadDataSource("read by user_name: the search answers 500", j{"user_name": userA})
	},
	"read-by-id-search-error": func(h *grHarness) {
		id := h.users().seed(j{"username": userA})
		h.users().searchStatus = 500
		h.ReadDataSource("read by id: the search answers 500", j{"id": id})
	},
}

// canonicalizeUserStepValues sorts the group ids, which the state holds as a set.
func canonicalizeUserStepValues(steps []grStep) {
	for i := range steps {
		canonicalizeUserSets(steps[i].Plan)
		canonicalizeUserSets(steps[i].State)
	}
}

func canonicalizeUserSets(value any) {
	switch value := value.(type) {
	case map[string]any:
		for key, item := range value {
			if elements, ok := item.([]any); ok && key == "groups" {
				slices.SortFunc(elements, func(a, b any) int {
					left, _ := a.(string)
					right, _ := b.(string)
					switch {
					case left < right:
						return -1
					case left > right:
						return 1
					}
					return 0
				})
				continue
			}
			canonicalizeUserSets(item)
		}
	case []any:
		for _, item := range value {
			canonicalizeUserSets(item)
		}
	}
}
