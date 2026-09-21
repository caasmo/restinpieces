package s3

import (
	"context"
	"errors"
	"net/http"
)

// DeleteObject deletes a single object by its key.
//
// https://docs.aws.amazon.com/AmazonS3/latest/API/API_DeleteObject.html
func (s3 *S3) DeleteObject(ctx context.Context, key string, optFuncs ...func(*http.Request)) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s3.URL(key), nil)
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
