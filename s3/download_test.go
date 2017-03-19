package s3

import (
	"io"
	"os"
)

func ExampleOpen() {
	r, _, err := Open("s3://s3-us-west-2.amazonaws.com/bucket_name/file.txt", nil)
	if err != nil {
		return
	}
	f, _ := os.Open("file.txt")
	io.Copy(f, r)
}
