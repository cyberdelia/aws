package aws

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSign(t *testing.T) {
	c, err := time.Parse(isoFormat, "20130524T000000Z")
	if err != nil {
		t.Error(err)
	}
	// Examples from : http://docs.aws.amazon.com/AmazonS3/latest/API/sig-v4-header-based-auth.html#example-signature-calculations
	var tests = []struct {
		req    func() *http.Request
		sha256 string
		auth   string
	}{
		{
			req: func() *http.Request {
				req, _ := http.NewRequest("GET", "https://examplebucket.s3.amazonaws.com/test.txt", nil)
				req.Header.Add("User-Agent", "s3")
				req.Header.Add("Range", "bytes=0-9")
				return req
			},
			sha256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			auth:   "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request,SignedHeaders=host;range;x-amz-content-sha256;x-amz-date,Signature=f0e8bdb87c964420e857bd35b5d6ed310bd44f0170aba48dd91039c6036bdb41",
		},
		{
			req: func() *http.Request {
				req, _ := http.NewRequest("PUT", "https://examplebucket.s3.amazonaws.com/test$file.text", strings.NewReader("Welcome to Amazon S3."))
				req.Header.Add("User-Agent", "s3")
				req.Header.Add("Date", c.In(time.FixedZone("GMT", 0)).Format(time.RFC1123))
				req.Header.Add("X-Amz-Storage-Class", "REDUCED_REDUNDANCY")
				return req
			},
			sha256: "44ce7dd67c959e0d3524ffac1771dfbba87d2b6b4b4e99e42034a8b803f8b072",
			auth:   "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request,SignedHeaders=date;host;x-amz-content-sha256;x-amz-date;x-amz-storage-class,Signature=98ad721746da40c64f1a55b78f14c238d841ea1380cd77a1b5971af0ece108bd",
		},
		{
			req: func() *http.Request {
				req, _ := http.NewRequest("GET", "https://examplebucket.s3.amazonaws.com?lifecycle", nil)
				req.Header.Add("User-Agent", "s3")
				return req
			},
			sha256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			auth:   "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request,SignedHeaders=host;x-amz-content-sha256;x-amz-date,Signature=fea454ca298b7da1c68078a5d1bdbfbbe0d65c699e0f91ac7a200a0136783543",
		},
		{
			req: func() *http.Request {
				req, _ := http.NewRequest("GET", "https://examplebucket.s3.amazonaws.com/?max-keys=2&prefix=J", nil)
				req.Header.Add("User-Agent", "s3")
				return req
			},
			sha256: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			auth:   "AWS4-HMAC-SHA256 Credential=AKIAIOSFODNN7EXAMPLE/20130524/us-east-1/s3/aws4_request,SignedHeaders=host;x-amz-content-sha256;x-amz-date,Signature=34b48302e7b5fa45bde8084f4b7868a86f0a534bc59db6670ed5711ef69dc6f7",
		},
	}
	s := &V4Signer{
		Clock: func() time.Time {
			return c
		},
		Region:    "us-east-1",
		AccessKey: "AKIAIOSFODNN7EXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		Service:   "s3",
	}
	for i, ts := range tests {
		req := ts.req()
		s.Sign(req)
		if date := req.Header.Get("X-Amz-Date"); date != c.Format(isoFormat) {
			t.Errorf("(%d) incorrect x-amz-date: expected %s, got %s", i, date, c.Format(isoFormat))
		}
		if sha := req.Header.Get("X-Amz-Content-Sha256"); sha != ts.sha256 {
			t.Errorf("(%d) incorrect x-amz-content-sha256: expected %s, got %s", i, ts.sha256, sha)
		}
		if auth := req.Header.Get("Authorization"); auth != ts.auth {
			t.Errorf("(%d) incorrect authorization: expected %s, got %s", i, ts.auth, auth)
		}
	}
}

func BenchmarkSign(b *testing.B) {
	s := &V4Signer{
		Region:    "us-east-1",
		AccessKey: "AKIAIOSFODNN7EXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		Service:   "s3",
	}
	req, err := http.NewRequest("GET", "https://s3.amazonaws.com/examplebucket/test.txt", nil)
	if err != nil {
		b.Error(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Sign(req)
	}
}
