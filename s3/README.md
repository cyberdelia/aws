# S3

s3 provides signing of HTTP requests to Amazon S3, along with parallelized and
streamed upload and download of S3 objects.

## Features

- *Streaming*: Parallel and streaming upload and download for efficient operations.
- *Integrity checks*: Integrity checks are done during multi-part upload.
- *Retries*: Every call to s3 are retried according to Amazon S3 recommendations.
- *Memory conscious*: s3 tries to make a concious usage of memory during upload and download.

## Installation

Download and install:

```
$ go get github.com/cyberdelia/aws/s3
```

Add it to your code:

```go
import "github.com/cyberdelia/aws/s3"
```

## Usage

```go
w, err := s3.Create("s3://s3-us-west-2.amazonaws.com/bucket_name/file.txt", nil, nil)
if err != nil {
    //...
}

r, _, err := s3.Open("s3://s3-us-west-2.amazonaws.com/bucket_name/file.txt", nil)
if err != nil {
    //...
}

walkFn := func(name string, info os.FileInfo) error {
    if info.IsDir() {
        return s3.SkipDir
    }
    return nil
}
if err := s3.Walk("s3://s3-us-west-2.amazonaws.com/bucket_name/", walkFn, nil); err != nil {
    // ...
}
```

## See also

* [s3gof3r](https://github.com/rlmcpherson/s3gof3r)
* [s3util](https://github.com/kr/s3)
