package s3

import (
	"context"
	"errors"
	"io"
	"net/http"
)

// PutObject uploads the object named by key, reading its content from body.
//
// The object travels in one PUT request; an existing object with the same
// key is replaced. Multipart upload is not implemented, so the object must
// stay under the 5 GiB S3 limit. body is read while the request is sent, so
// the object does not have to fit in memory. If body is also an io.Closer,
// it is closed when the upload ends, on success or failure.
//
// size is the number of bytes in body. Pass a negative value when the size
// is unknown; the request is then sent in chunks.
//
// https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html
func (s3 *S3) PutObject(ctx context.Context, key string, body io.Reader, size int64, optFuncs ...func(*http.Request)) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, s3.URL(key), body)
	if err != nil {
		closer, ok := body.(io.Closer)
		if ok {
			err = errors.Join(err, closer.Close())
		}
		return err
	}

	// a negative size means the caller does not know the length; -1 is the
	// sentinel that makes the request send the body in chunks
	if size < 0 {
		size = -1
	}
	req.ContentLength = size

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
