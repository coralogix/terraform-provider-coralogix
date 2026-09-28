package cxsdk

import (
	"net/http"
)

func NewAPIError(resp *http.Response, err error) error {
	_ = resp
	return err
}

func Code(err error) int {
	_ = err
	return 0
}
