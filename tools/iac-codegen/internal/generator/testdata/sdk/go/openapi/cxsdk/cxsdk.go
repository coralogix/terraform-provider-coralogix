package cxsdk

import (
	"errors"
	"net/http"
)

func NewAPIError(resp *http.Response, err error) error {
	_ = resp
	return err
}

// IsNotFound reports whether err is a 404 for a missing resource. A 404 whose body says
// "Not Found: Not Found" comes from a route that does not exist, so it is not one.
func IsNotFound(err error) bool {
	var body interface{ Body() []byte }
	if errors.As(err, &body) && string(body.Body()) == "Not Found: Not Found" {
		return false
	}
	return Code(err) == http.StatusNotFound
}

// Code returns the HTTP status of an error that has one, and 0 otherwise.
func Code(err error) int {
	var status interface{ StatusCode() int }
	if errors.As(err, &status) {
		return status.StatusCode()
	}
	return 0
}
