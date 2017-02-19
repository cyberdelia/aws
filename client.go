package s3

import (
	"io"
	"net/http"
	"net/url"
)

// DefaultClient is the default http.Client used for all requests.
var DefaultClient = &http.Client{
	Transport: new(Transport),
}

// Get issues a GET to the specified URL.
func Get(url string) (resp *http.Response, err error) {
	return DefaultClient.Get(url)
}

// Post issues a POST to the specified URL.
func Post(url string, contentType string, body io.Reader) (resp *http.Response, err error) {
	return DefaultClient.Post(url, contentType, body)
}

// PostForm issues a POST to the specified URL, with data's keys and
// values URL-encoded as the request body.
func PostForm(url string, data url.Values) (resp *http.Response, err error) {
	return DefaultClient.PostForm(url, data)
}

// Head issues a HEAD to the specified URL.
func Head(url string) (resp *http.Response, err error) {
	return DefaultClient.Head(url)
}
