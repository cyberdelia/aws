package s3

import "net/http"

// Transport allows to make signed calls to AWS S3 endpoints.
type Transport struct {
	// Signer is the underlying request signer used when making requests.
	// It will default to DefaultSigner if nil.
	Signer Signer

	// Transport is the underlying HTTP transport to use when making requests.
	// It will default to http.DefaultTransport if nil.
	Transport http.RoundTripper
}

// RoundTrip implements the RoundTripper interface.
func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = cloneRequest(r)

	t.signer().Sign(r)

	return t.transport().RoundTrip(r)
}

func (t *Transport) signer() Signer {
	if t.Signer != nil {
		return t.Signer
	}
	return DefaultSigner
}

func (t *Transport) transport() http.RoundTripper {
	if t.Transport != nil {
		return t.Transport
	}
	return http.DefaultTransport
}

func cloneRequest(r *http.Request) *http.Request {
	c := new(http.Request)
	*c = *r
	c.Header = make(http.Header, len(r.Header))
	for k, s := range r.Header {
		c.Header[k] = append([]string(nil), s...)
	}
	return c
}
