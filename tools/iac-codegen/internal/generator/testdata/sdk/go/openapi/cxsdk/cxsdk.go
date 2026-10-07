package cxsdk

import (
	"errors"
	"net/http"
)

func NewAPIError(resp *http.Response, err error) error {
	_ = resp
	return err
}

// Code returns the HTTP status of an error that has one, and 0 otherwise.
func Code(err error) int {
	var status interface{ StatusCode() int }
	if errors.As(err, &status) {
		return status.StatusCode()
	}
	return 0
}
