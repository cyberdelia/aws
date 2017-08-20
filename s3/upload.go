package s3

import (
	"bytes"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

const (
	minPartSize = 5 * 1024 * 1024
	maxPartSize = 5 * 1024 * 1024 * 1024
	retries     = 3
)

var concurrency = runtime.NumCPU()

type part struct {
	PartNumber int    `xml:"PartNumber"`
	ETag       string `xml:"ETag"`

	uploadID string
	url      string
	md5      []byte
	client   *http.Client

	io.ReadSeeker `xml:"-"`
}

func (p *part) Reset() {
	p.ReadSeeker = nil
}

func (p *part) Upload() error {
	v := url.Values{
		"partNumber": []string{strconv.Itoa(p.PartNumber)},
		"uploadId":   []string{p.uploadID},
	}
	resp, err := retry(func() (*http.Response, error) {
		_, err := p.ReadSeeker.Seek(0, 0)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequest("PUT", p.url+"?"+v.Encode(), p.ReadSeeker)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-MD5", base64.StdEncoding.EncodeToString(p.md5))
		resp, err := p.client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == 500 {
			return nil, newResponseError(resp)
		}
		return resp, nil
	}, retries)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return newResponseError(resp)
	}
	s := resp.Header.Get("ETag")
	if len(s) < 2 {
		return fmt.Errorf("s3: received invalid checksum: %q", s)
	}
	p.ETag = s[1 : len(s)-1]
	if eTag := hex.EncodeToString(p.md5); p.ETag != eTag {
		return fmt.Errorf("s3: mismatching checksum: %q != %q", p.ETag, eTag)
	}
	return nil
}

type uploader struct {
	XMLName string  `xml:"CompleteMultipartUpload"`
	Parts   []*part `xml:"Part"`

	buf    *bytes.Buffer
	client *http.Client
	md5    hash.Hash
	parts  chan *part
	wg     sync.WaitGroup
	w      io.Writer

	size     int
	url      string
	uploadID string
	err      error
}

// Create creates an S3 object at url and sends multipart upload requests as
// data is written.
func Create(uri string, h http.Header, c *http.Client) (io.WriteCloser, error) {
	if c == nil {
		c = DefaultClient
	}

	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}
	u.Scheme = "https"

	buf := new(bytes.Buffer)
	m := md5.New()
	up := &uploader{
		client: c,
		size:   minPartSize,
		buf:    buf,
		parts:  make(chan *part),
		url:    u.String(),
		md5:    m,
		w:      io.MultiWriter(buf, m),
	}

	// Create multi-part upload.
	req, err := http.NewRequest("POST", u.String()+"?uploads", nil)
	if err != nil {
		return nil, err
	}
	for k := range h {
		for _, v := range h[k] {
			req.Header.Add(k, v)
		}
	}
	resp, err := retry(retryNoBody(up.client, req), retries)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, newResponseError(resp)
	}
	var mu struct {
		UploadID string `xml:"UploadId"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&mu); err != nil {
		return nil, err
	}
	up.uploadID = mu.UploadID

	// Start uploading parts.
	for i := 0; i < concurrency; i++ {
		go up.upload()
	}
	return up, nil
}

func (u *uploader) upload() {
	for p := range u.parts {
		if err := p.Upload(); err != nil {
			u.err = err
		}
		p.Reset()
		u.wg.Done()
	}
}

func (u *uploader) abort() {
	v := url.Values{
		"uploadId": []string{u.uploadID},
	}
	req, err := http.NewRequest("DELETE", u.url+"?"+v.Encode(), nil)
	if err != nil {
		return
	}
	resp, err := retry(retryNoBody(u.client, req), retries)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

func (u *uploader) Write(p []byte) (int, error) {
	if u.err != nil {
		u.abort()
		return 0, u.err
	}
	n, err := u.w.Write(p)
	if err != nil {
		return n, err
	}
	if u.buf.Len() >= u.size {
		u.flush()
	}
	return n, nil
}

func (u *uploader) ReadFrom(r io.Reader) (n int64, err error) {
	buf := make([]byte, minPartSize)
	for {
		m, err := r.Read(buf)
		if err != nil && err != io.EOF {
			return n, err
		}
		m, err = u.Write(buf[:m])
		n += int64(m)
		if err != nil || err == io.EOF {
			return n, err
		}
	}
}

func (u *uploader) flush() {
	u.wg.Add(1)
	u.size = min(u.size+u.size/1000, maxPartSize)
	b := cloneBytes(u.buf.Bytes())
	c := u.md5.Sum(nil)
	p := &part{
		PartNumber: len(u.Parts) + 1,
		ReadSeeker: bytes.NewReader(b),
		url:        u.url,
		client:     u.client,
		uploadID:   u.uploadID,
		md5:        c,
	}
	u.Parts = append(u.Parts, p)
	u.parts <- p
	u.buf.Reset()
	u.md5.Reset()
}

func (u *uploader) complete() error {
	body, err := xml.Marshal(u)
	if err != nil {
		return err
	}
	b := bytes.NewReader(body)
	v := url.Values{
		"uploadId": []string{u.uploadID},
	}
	resp, err := retry(func() (*http.Response, error) {
		if _, err := b.Seek(0, 0); err != nil {
			return nil, err
		}
		req, err := http.NewRequest("POST", u.url+"?"+v.Encode(), b)
		if err != nil {
			return nil, err
		}
		resp, err := u.client.Do(req)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode == 500 {
			return nil, newResponseError(resp)
		}
		return resp, nil
	}, retries)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return newResponseError(resp)
	}
	defer resp.Body.Close()
	var e struct {
		XMLName string `xml:"CompleteMultipartUploadResult"`
		ETag    string `xml:"ETag"`
	}
	if err := xml.NewDecoder(resp.Body).Decode(&e); err != nil {
		return err
	}
	r := strings.Split(strings.Trim(e.ETag, "\""), "-")[0]
	if len(r) == 0 {
		return fmt.Errorf("s3: no checksum found")
	}
	u.md5.Reset()
	for _, p := range u.Parts {
		u.md5.Write(p.md5)
	}
	if hex.EncodeToString(u.md5.Sum(nil)) != r {
		return fmt.Errorf("s3: mismatching checksum: %q", e.ETag)
	}
	return nil
}

func (u *uploader) Close() error {
	u.flush()
	u.wg.Wait()
	if u.err != nil {
		return u.err
	}
	return u.complete()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func cloneBytes(b []byte) []byte {
	c := make([]byte, len(b))
	copy(c, b)
	return c
}
