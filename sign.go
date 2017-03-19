package aws

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
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

// Signer allows to sign a request..
type Signer interface {
	Sign(*http.Request)
}

// V4Signer allows to sign a request following AWS V4 signature requirements.
type V4Signer struct {
	Region        string
	SecretKey     string
	AccessKey     string
	SecurityToken string
	Service       string

	Clock func() time.Time
}

// Transport returns a http.RoundTripper using this signer to sign requests.
func (s *V4Signer) Transport() http.RoundTripper {
	return &Transport{
		Signer: s,
	}
}

// Sign signs the given http.Request.
func (s *V4Signer) Sign(r *http.Request) {
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

	// Compute credential
	credential := strings.Join([]string{
		now.Format(shortFormat),
		region,
		s.Service,
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
	uri := uriEncode(r.URL.Path)
	// if uri != "" {
	// 	uri = "/" + strings.Join(strings.Split(uri, "/")[3:], "/")
	// } else {
	// 	uri = r.URL.EscapedPath()
	// }
	if uri == "" {
		uri = "/"
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
	serviceHMAC := sign(regionHMAC, []byte(s.Service))
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

func (s *V4Signer) region(host string) string {
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

func uriEncode(s string) string {
	spaceCount, hexCount := 0, 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if shouldEscape(c) {
			if c == ' ' {
				spaceCount++
			} else {
				hexCount++
			}
		}
	}

	if spaceCount == 0 && hexCount == 0 {
		return s
	}

	t := make([]byte, len(s)+2*hexCount)
	j := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case shouldEscape(c):
			t[j] = '%'
			t[j+1] = "0123456789ABCDEF"[c>>4]
			t[j+2] = "0123456789ABCDEF"[c&15]
			j += 3
		default:
			t[j] = s[i]
			j++
		}
	}
	return string(t)
}

func shouldEscape(c byte) bool {
	if 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' {
		return false
	}
	switch c {
	case '-', '_', '.', '~':
		return false
	case '/':
		return false
	}
	return true
}
