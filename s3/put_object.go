package s3

import (
	"bytes"
	"context"
	"errors"
	"net/http"
)

// PutObject uploads data as the object named by key.
//
// The object travels in one PUT request; an existing object with the
// same key is replaced. Objects larger than 5 GiB cannot be uploaded
// this way, and the whole object is passed as a byte slice, so it must
// fit in memory.
//
// https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html
func (s3 *S3) PutObject(ctx context.Context, key string, data []byte, optFuncs ...func(*http.Request)) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, s3.URL(key), bytes.NewReader(data))
	if err != nil {
		return err
	}

	// apply optional request funcs
	for _, fn := range optFuncs {
		if fn != nil {
			fn(req)
		}
	}

	resp, err := s3.SignAndSend(req)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()

	return nil
}
