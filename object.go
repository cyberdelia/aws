package s3

import (
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type objectInfo struct {
	name    string
	size    int64
	dir     bool
	modTime time.Time
	sys     *Object
}

func (f *objectInfo) Name() string { return f.name }
func (f *objectInfo) Size() int64  { return f.size }

func (f *objectInfo) Mode() os.FileMode {
	if f.dir {
		return 0755 | os.ModeDir
	}
	return 0644
}

func (f *objectInfo) ModTime() time.Time {
	if f.modTime.IsZero() && f.sys != nil {
		f.modTime, _ = time.Parse(time.RFC3339Nano, f.sys.LastModified)
	}
	return f.modTime
}

func (f *objectInfo) IsDir() bool      { return f.dir }
func (f *objectInfo) Sys() interface{} { return f.sys }

// Object represents an S3 object.
type Object struct {
	Key          string
	LastModified string
	ETag         string
	Size         string
	StorageClass string
	OwnerID      string `xml:"Owner>ID"`
	OwnerName    string `xml:"Owner>DisplayName"`
}

// SkipDir is used as a return value from WalkFuncs to indicate that
// the directory named in the call is to be skipped. It is not returned
// as an error by any function.
var SkipDir = errors.New("skip this directory")

// WalkFunc is the type of the function called for each objects visited by Walk.
type WalkFunc func(name string, info os.FileInfo) error

// Walk walks the bucket starting at prefix, calling walkFn for each objects
// in the bucket.
func Walk(uri string, walkFn WalkFunc, client *http.Client) error {
	c := client
	if c == nil {
		c = DefaultClient
	}
	u, err := url.Parse(uri)
	if err != nil {
		return err
	}
	path := strings.Split(u.Path, "/")
	u.Path = strings.Join(path[:2], "/")
	prefix := strings.Join(path[2:], "/")
	objects, err := readObjects(u, prefix, c)
	if err != nil {
		return err
	}
	if err := walk(u, objects, walkFn, c); err != nil {
		return err
	}
	return nil
}

func walk(u *url.URL, objects []os.FileInfo, walkFn WalkFunc, c *http.Client) error {
	for _, o := range objects {
		if err := walkFn(o.Name(), o); err != nil {
			if o.IsDir() && err == SkipDir {
				return nil
			}
			return err
		}
		if o.IsDir() {
			d, err := readObjects(u, o.Name()+"/", c)
			if err != nil {
				return err
			}
			if err := walk(u, d[1:], walkFn, c); err != nil {
				return err
			}
		}
	}
	return nil
}

func readObjects(u *url.URL, prefix string, c *http.Client) (objects []os.FileInfo, err error) {
	var completed bool
	q := url.Values{
		"list-type":   []string{"2"},
		"delimiter":   []string{"/"},
		"fetch-owner": []string{"true"},
		"prefix":      []string{prefix},
	}
	for !completed {
		u.RawQuery = q.Encode()
		req, err := http.NewRequest("GET", u.String(), nil)
		if err != nil {
			return nil, err
		}
		resp, err := retry(retryNoBody(c, req), retries)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != 200 {
			return nil, newResponseError(resp)
		}
		var l struct {
			Truncated bool     `xml:"IsTruncated"`
			Token     string   `xml:"NextContinuationToken"`
			Objects   []Object `xml:"Contents"`
			Prefixes  []string `xml:"CommonPrefixes>Prefix"`
		}
		if err := xml.NewDecoder(resp.Body).Decode(&l); err != nil {
			// A 200 OK response can contain valid or invalid XML.
			// http://docs.aws.amazon.com/AmazonS3/latest/API/v2-RESTBucketGET.html#v2-RESTBucketGET-description
			if err == io.EOF {
				return nil, nil
			}
			return nil, err
		}

		// Stop iteration if needed.
		completed = !l.Truncated
		// Or continues it.
		q.Set("continuation-token", l.Token)
		for _, c := range l.Objects {
			// Parse ETag properly
			c.ETag = c.ETag[1 : len(c.ETag)-1]

			size, _ := strconv.ParseInt(c.Size, 10, 0)
			name, dir := c.Key, false
			if size == 0 && strings.HasSuffix(c.Key, "/") {
				name = strings.TrimRight(c.Key, "/")
				dir = true
			}
			objects = append(objects, &objectInfo{
				name: name,
				size: size,
				dir:  dir,
				sys:  &c,
			})
		}
		for _, p := range l.Prefixes {
			name := strings.TrimRight(p, "/")
			objects = append(objects, &objectInfo{
				name: name,
				size: 0,
				dir:  true,
			})
		}
	}
	return objects, nil
}
