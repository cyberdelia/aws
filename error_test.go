package s3

import (
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

func TestResponseError(t *testing.T) {
	var tests = []struct {
		Response *http.Response
		Error    string
	}{
		{
			Response: &http.Response{
				StatusCode: 400,
				Body:       ioutil.NopCloser(strings.NewReader("")),
			},
			Error: "s3: unexpected error: (400)",
		},
		{
			Response: &http.Response{
				StatusCode: 400,
				Body: ioutil.NopCloser(strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
<Error>
  <Code>NoSuchKey</Code>
  <Message>The resource you requested does not exist</Message>
  <Resource>/mybucket/myfoto.jpg</Resource>
  <RequestId>4442587FB7D0A2F9</RequestId>
</Error>`)),
			},
			Error: "s3: unexpected error: (NoSuchKey) The resource you requested does not exist",
		},
	}
	for _, test := range tests {
		err := newResponseError(test.Response)
		if err.Error() != test.Error {
			t.Errorf("expected %s, got %s", err.Error(), test.Error)
		}
	}
}
