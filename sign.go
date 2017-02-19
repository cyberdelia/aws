package s3

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
)

var regionMatcher = regexp.MustCompile(`\w{2}-(\w+|gov-\w+)-\d+`)

const (
	prefix      = "AWS4-HMAC-SHA256"
	isoFormat   = "20060102T150405Z"
	shortFormat = "20060102"
)

var ignoredHeaders = map[string]bool{
	"Authorization":  true,
	"Content-Type":   true,
	"Content-Length": true,
	"User-Agent":     true,
}

// DefaultSigner is the default Signer and is use for all requests.
var DefaultSigner = &AWSSigner{
	Region:        os.Getenv("AWS_REGION"),
	AccessKey:     os.Getenv("AWS_ACCESS_KEY_ID"),
	SecretKey:     os.Getenv("AWS_SECRET_ACCESS_KEY"),
	SecurityToken: os.Getenv("AWS_SECURITY_TOKEN"),
}

// Signer allows to sign a request..
type Signer interface {
	Sign(*http.Request)
}

// AWSSigner allows to sign a request following AWS V4 signature requirements.
type AWSSigner struct {
	Region        string
	SecretKey     string
	AccessKey     string
	SecurityToken string

	Clock func() time.Time
}

// Transport returns a http.RoundTripper using this signer to sign requests.
func (s *AWSSigner) Transport() http.RoundTripper {
	return &Transport{
		Signer: s,
	}
}

// Sign signs the given http.Request.
func (s *AWSSigner) Sign(r *http.Request) {
	clock := s.Clock
	if clock == nil {
		clock = time.Now
	}

	region := s.region(r.URL.Host)
	now := clock().UTC()

	// Set STS token if present.
	if s.SecurityToken != "" {
		r.Header.Set("X-Amz-Security-Token", s.SecurityToken)
	}

	// Set time
	r.Header.Set("X-Amz-Date", now.Format(isoFormat))

	// Compute credential
	credential := strings.Join([]string{
		now.Format(shortFormat),
		region,
		"s3",
		"aws4_request",
	}, "/")

	// Compute signed headers
	headers := []string{"host"}
	for k := range r.Header {
		if _, ok := ignoredHeaders[http.CanonicalHeaderKey(k)]; ok {
			continue
		}
		headers = append(headers, strings.ToLower(k))
	}
	sort.Strings(headers)
	signedHeaders := strings.Join(headers, ";")

	// Compute canonical headers
	headerValues := make([]string, len(headers))
	for i, k := range headers {
		if k == "host" {
			headerValues[i] = "host:" + r.URL.Host
		} else {
			headerValues[i] = k + ":" +
				strings.Join(r.Header[http.CanonicalHeaderKey(k)], ",")
		}
	}
	canonicalHeaders := strings.Join(headerValues, "\n")

	r.URL.RawQuery = strings.Replace(r.URL.Query().Encode(), "+", "%20", -1)
	uri := r.URL.Opaque
	if uri != "" {
		uri = "/" + strings.Join(strings.Split(uri, "/")[3:], "/")
	} else {
		uri = r.URL.EscapedPath()
	}
	if uri == "" {
		uri = "/"
	}

	// Compute digest
	digest := r.Header.Get("X-Amz-Content-Sha256")
	if digest == "" {
		if r.GetBody != nil {
			b, _ := r.GetBody()
			s := sha256.New()
			io.Copy(s, b)
			digest = hex.EncodeToString(s.Sum(nil))
		} else {
			digest = hex.EncodeToString(sha([]byte{}))
		}
		r.Header.Add("X-Amz-Content-Sha256", digest)
	}

	// Compute canonical string
	canonical := strings.Join([]string{
		r.Method,
		uri,
		r.URL.RawQuery,
		canonicalHeaders + "\n",
		signedHeaders,
		digest,
	}, "\n")

	// Compute string to sign
	stringToSign := strings.Join([]string{
		prefix,
		now.Format(isoFormat),
		credential,
		hex.EncodeToString(sha([]byte(canonical))),
	}, "\n")

	dateHMAC := sign([]byte("AWS4"+s.SecretKey), []byte(now.Format(shortFormat)))
	regionHMAC := sign(dateHMAC, []byte(region))
	serviceHMAC := sign(regionHMAC, []byte("s3"))
	credentialsHMAC := sign(serviceHMAC, []byte("aws4_request"))
	signature := hex.EncodeToString(sign(credentialsHMAC, []byte(stringToSign)))

	// Compose
	parts := []string{
		prefix + " Credential=" + s.AccessKey + "/" + credential,
		"SignedHeaders=" + signedHeaders,
		"Signature=" + signature,
	}
	r.Header.Set("Authorization", strings.Join(parts, ","))
}

func (s *AWSSigner) region(host string) string {
	switch host {
	case "s3.amazonaws.com", "s3-external-1.amazonaws.com":
		return "us-east-1"
	default:
		if region := regionMatcher.FindString(host); region != "" {
			return region
		}
		// Couldn't detect a region based on hostname, so use the given one.
		// This allow to support S3 accelerate too.
		return s.Region
	}
}

func sign(key []byte, data []byte) []byte {
	hash := hmac.New(sha256.New, key)
	hash.Write(data)
	return hash.Sum(nil)
}

func sha(data []byte) []byte {
	hash := sha256.New()
	hash.Write(data)
	return hash.Sum(nil)
}
