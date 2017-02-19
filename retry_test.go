package s3

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRetry(t *testing.T) {
	var count int
	var tests = []struct {
		RetryFn retryFunc
		Retries int
	}{
		{
			RetryFn: func() (*http.Response, error) {
				count++
				return nil, errors.New("retry")
			},
			Retries: 3,
		},
		{
			RetryFn: func() (*http.Response, error) {
				count++
				if count <= 2 {
					return nil, errors.New("retry")
				}
				return nil, nil
			},
			Retries: 3,
		},
	}
	for _, test := range tests {
		retry(test.RetryFn, retries)
		if count != test.Retries {
			t.Errorf("expected %d retries made %d", retries, count)
		}
		count = 0
	}
}

func TestRetryNoBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	req, _ := http.NewRequest("GET", ts.URL, nil)
	resp, err := retryNoBody(http.DefaultClient, req)()
	if err != nil {
		t.Error("unexpected error")
	}
	if resp.StatusCode != 200 {
		t.Errorf("unexpected status code (%d)", resp.StatusCode)
	}
}

func TestRetryNoBodyError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	req, _ := http.NewRequest("GET", ts.URL, nil)
	_, err := retryNoBody(http.DefaultClient, req)()
	if err == nil {
		t.Error("unexpected success")
	}
}

func TestRetryWithBody(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("should have paniced")
		}
	}()
	req, _ := http.NewRequest("GET", "http://example.org", strings.NewReader("<>"))
	retryNoBody(http.DefaultClient, req)()
}
