// Package cxsdk is the handwritten part of the fake SDK. It copies the names
// and signatures of the real cxsdk package that the generated resources use:
// ClientSet, one accessor for each service, NewAPIError, and Code.
package cxsdk

import (
	"errors"
	"fmt"
	"net/http"

	fakeboards "github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk/go/openapi/gen/fake_boards_service"
	fakerules "github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk/go/openapi/gen/fake_rules_service"
	fakesettings "github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk/go/openapi/gen/fake_settings_service"
	fakeviews "github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/fakesdk/go/openapi/gen/fake_views_service"
)

// ClientSet holds one client for each service.
type ClientSet struct {
	fakeBoards   *fakeboards.FakeBoardsServiceAPIService
	fakeSettings *fakesettings.FakeSettingsServiceAPIService
	fakeRules    *fakerules.FakeRulesServiceAPIService
	fakeViews    *fakeviews.FakeViewsServiceAPIService
}

// NewClientSet returns a ClientSet whose clients send requests to url.
func NewClientSet(url string) *ClientSet {
	boards := fakeboards.NewConfiguration()
	boards.Servers = fakeboards.ServerConfigurations{{URL: url}}
	settings := fakesettings.NewConfiguration()
	settings.Servers = fakesettings.ServerConfigurations{{URL: url}}
	rules := fakerules.NewConfiguration()
	rules.Servers = fakerules.ServerConfigurations{{URL: url}}
	views := fakeviews.NewConfiguration()
	views.Servers = fakeviews.ServerConfigurations{{URL: url}}
	return &ClientSet{
		fakeBoards:   fakeboards.NewAPIClient(boards).FakeBoardsServiceAPI,
		fakeSettings: fakesettings.NewAPIClient(settings).FakeSettingsServiceAPI,
		fakeRules:    fakerules.NewAPIClient(rules).FakeRulesServiceAPI,
		fakeViews:    fakeviews.NewAPIClient(views).FakeViewsServiceAPI,
	}
}

// FakeRules returns the FakeRulesServiceAPIService client.
func (c *ClientSet) FakeRules() *fakerules.FakeRulesServiceAPIService {
	return c.fakeRules
}

// FakeViews returns the FakeViewsServiceAPIService client.
func (c *ClientSet) FakeViews() *fakeviews.FakeViewsServiceAPIService {
	return c.fakeViews
}

// FakeSettings returns the FakeSettingsServiceAPIService client.
func (c *ClientSet) FakeSettings() *fakesettings.FakeSettingsServiceAPIService {
	return c.fakeSettings
}

// FakeBoards returns the FakeBoardsServiceAPIService client.
func (c *ClientSet) FakeBoards() *fakeboards.FakeBoardsServiceAPIService {
	return c.fakeBoards
}

// APIError is an error with the HTTP status and the response body.
type APIError struct {
	StatusCode int
	Body       []byte
	Err        error
}

func (e *APIError) Error() string {
	if len(e.Body) != 0 {
		return fmt.Sprintf("%d: %s — %s", e.StatusCode, e.Err, e.Body)
	}
	return fmt.Sprintf("%d: %s", e.StatusCode, e.Err)
}

func (e *APIError) Unwrap() error { return e.Err }

// openAPIError is the method set of the generated GenericOpenAPIError.
type openAPIError interface {
	Body() []byte
}

// NewAPIError creates an APIError from an SDK call result.
func NewAPIError(resp *http.Response, err error) error {
	if err == nil {
		return nil
	}
	apiErr := &APIError{Err: err}
	if resp != nil {
		apiErr.StatusCode = resp.StatusCode
	}
	var oapiErr openAPIError
	if errors.As(err, &oapiErr) {
		apiErr.Body = oapiErr.Body()
	}
	return apiErr
}

// Code returns the HTTP status code of err, or 0.
func Code(err error) int {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	return 0
}
