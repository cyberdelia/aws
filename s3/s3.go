package s3

import (
	"net/http"
	"os"

	"github.com/cyberdelia/aws"
)

// DefaultSigner is the default Signer and is use for all requests.
var DefaultSigner = &aws.V4Signer{
	Region:        os.Getenv("AWS_REGION"),
	AccessKey:     os.Getenv("AWS_ACCESS_KEY_ID"),
	SecretKey:     os.Getenv("AWS_SECRET_ACCESS_KEY"),
	SecurityToken: os.Getenv("AWS_SECURITY_TOKEN"),
	Service:       "s3",
}

// DefaultClient is the default http.Client used for all requests.
var DefaultClient = &http.Client{
	Transport: DefaultSigner.Transport(),
}
