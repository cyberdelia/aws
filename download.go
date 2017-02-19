package s3

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
)

type chunk struct {
	client    *http.Client
	buf       *bytes.Buffer
	done      chan bool
	readAhead chan bool

	header http.Header
	url    string
	err    error
}

func (c *chunk) Read(p []byte) (int, error) {
	<-c.done
	if c.err != nil {
		return 0, c.err
	}
	n, err := c.buf.Read(p)
	if err != nil {
		if err == io.EOF {
			c.readAhead <- true
			c.buf = nil
		}
		return n, err
	}
	return n, nil
}

func (c *chunk) Download() error {
	defer close(c.done)

	req, err := http.NewRequest("GET", c.url, nil)
	if err != nil {
		return err
	}
	for k := range c.header {
		for _, v := range c.header[k] {
			req.Header.Add(k, v)
		}
	}
	resp, err := retry(retryNoBody(c.client, req), retries)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 206 {
		return newResponseError(resp)
	}
	if _, err := c.buf.ReadFrom(resp.Body); err != nil {
		return err
	}
	return nil
}

type downloader struct {
	r         io.Reader
	chunks    chan *chunk
	readAhead chan bool
	once      sync.Once

	err error
}

// Open opens an S3 object at url and return a io.ReadCloser.
func Open(url string, c *http.Client) (io.ReadCloser, http.Header, error) {
	if c == nil {
		c = DefaultClient
	}

	// Retrieve Content-Length
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return nil, nil, err
	}
	resp, err := retry(retryNoBody(c, req), retries)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, nil, newResponseError(resp)
	}

	s, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return nil, nil, fmt.Errorf("s3: cannot parse content-length")
	}

	d := &downloader{
		chunks:    make(chan *chunk),
		readAhead: make(chan bool, concurrency),
	}

	// Create chunks
	var chunks []*chunk
	for i := int64(0); i < s; {
		size := min64(minPartSize, s)
		c := &chunk{
			done:      make(chan bool),
			buf:       new(bytes.Buffer),
			client:    c,
			url:       url,
			readAhead: d.readAhead,
			header: http.Header{
				"Range": {fmt.Sprintf("bytes=%d-%d", i, i+size-1)},
			},
		}
		chunks = append(chunks, c)
		i += size
	}

	var r []io.Reader
	for _, c := range chunks {
		r = append(r, c)
	}
	d.r = io.MultiReader(r...)

	go func() {
		for _, c := range chunks {
			d.chunks <- c
		}
	}()

	return d, resp.Header, nil
}

func (d *downloader) Read(p []byte) (int, error) {
	if d.err != nil {
		return 0, d.err
	}
	d.once.Do(func() {
		// Start downloading chunks only when requested.
		for i := 0; i < concurrency; i++ {
			go d.download()
		}
	})
	return d.r.Read(p)
}

func (d *downloader) Close() error {
	if d.err != nil {
		return d.err
	}
	return nil
}

func (d *downloader) download() {
	for c := range d.chunks {
		if err := c.Download(); err != nil {
			d.err = err
		}
		<-d.readAhead
	}
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
