package s3

import (
	"io"
	"os"
)

func ExampleCreate() {
	f, _ := os.Open("file.txt")
	w, err := Create("s3://s3-us-west-2.amazonaws.com/bucket_name/file.txt", nil, nil)
	if err != nil {
		return
	}
	io.Copy(w, f)
}
