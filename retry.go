package s3

import "net/http"

type retryFunc func() (*http.Response, error)

func retry(f retryFunc, r int) (resp *http.Response, err error) {
	for i := 0; i < r; i++ {
		if resp, err = f(); err == nil {
			return resp, nil
		}
	}
	return nil, err
}

func retryNoBody(c *http.Client, req *http.Request) retryFunc {
	if req.GetBody != nil {
		panic("request should not contain a body")
	}
	return func() (resp *http.Response, err error) {
		if resp, err = c.Do(req); err != nil {
			return nil, err
		}
		// Retry on internal errors as recommended.
		// http://docs.aws.amazon.com/AmazonS3/latest/dev/ErrorBestPractices.html#UsingErrorsRetry
		if resp.StatusCode == 500 {
			return nil, newResponseError(resp)
		}
		return resp, nil
	}
}
