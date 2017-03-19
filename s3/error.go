package s3

import (
	"encoding/xml"
	"fmt"
	"net/http"
)

type responseError struct {
	Code    string `xml:"Code"`
	Message string `xml:"Message"`

	r *http.Response
}

func newResponseError(r *http.Response) *responseError {
	e := &responseError{
		r: r,
	}
	defer r.Body.Close()
	xml.NewDecoder(r.Body).Decode(&e)
	return e
}

func (e *responseError) Error() string {
	if e.Code != "" && e.Message != "" {
		return fmt.Sprintf("s3: unexpected error: (%s) %s", e.Code, e.Message)
	}
	return fmt.Sprintf("s3: unexpected error: (%d)", e.r.StatusCode)
}
