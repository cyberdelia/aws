package s3

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"testing/quick"
)

func roundTrip(name string, payload []byte) bool {
	if name == "" {
		return true
	}

	uri := fmt.Sprintf("s3://%s/%s", os.Getenv("S3_BUCKET"), name)
	w, err := Create(uri, nil, nil)
	if err != nil {
		return false
	}
	if _, err := w.Write(payload); err != nil {
		return false
	}
	w.Close()

	r, _, err := Open(uri, nil)
	if err != nil {
		return false
	}
	defer r.Close()

	b, err := ioutil.ReadAll(r)
	if err != nil {
		return false
	}

	if err := Remove(uri, nil); err != nil {
		return false
	}

	return bytes.Equal(b, payload)
}

func TestRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	if err := quick.Check(roundTrip, nil); err != nil {
		t.Error(err)
	}
}

func ExampleWalk() {
	walkFn := func(name string, info os.FileInfo) error {
		if info.IsDir() {
			return SkipDir
		}
		return nil
	}
	if err := Walk("s3://s3-us-west-2.amazonaws.com/bucket_name/", walkFn, nil); err != nil {
		return
	}
}

func ExampleRemove() {
	if err := Remove("s3://s3-us-west-2.amazonaws.com/bucket_name/object.txt", nil); err != nil {
		return
	}
}
