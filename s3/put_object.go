package s3

import (
	"context"
	"errors"
	"io"
	"net/http"
)

// ErrContentLengthRequired is returned by PutObject when the length is
// unknown and the client is configured to require a content length. The
// caller has to find the length and pass it.
var ErrContentLengthRequired = errors.New("the service requires a content length and none was given")

// PutObject uploads the object named by key, reading its content from body.
//
// The object travels in one PUT request; an existing object with the same
// key is replaced. Multipart upload is not implemented, so the object must
// stay under the 5 GiB S3 limit. body is read while the request is sent, so
// the object does not have to fit in memory. If body is also an io.Closer,
// PutObject closes it when it returns, whether the upload succeeded or
// failed.
//
// contentLength is the number of bytes in body. Pass a negative value when
// the length is unknown; the request is then sent in chunks, with no length.
//
// Not every service accepts a chunked upload: Cloudflare R2, for example,
// rejects it. Set RequireContentLength for those services; PutObject then
// returns ErrContentLengthRequired for an unknown length instead of sending,
// so find the length first and pass it.
//
// https://docs.aws.amazon.com/AmazonS3/latest/API/API_PutObject.html
func (s3 *S3) PutObject(ctx context.Context, key string, body io.Reader, contentLength int64, optFuncs ...func(*http.Request)) (err error) {
	// a service that requires a length cannot accept the chunked upload
	// that an unknown length produces; stop before sending anything
	if contentLength < 0 && s3.RequireContentLength {
		// The request client closes body itself when the request is sent.
		// This path never gets there, so guard for an io.Closer and close
		// body here.
		closer, ok := body.(io.Closer)
		if !ok {
			return ErrContentLengthRequired
		}
		closeErr := closer.Close()
		return errors.Join(ErrContentLengthRequired, closeErr)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, s3.URL(key), body)
	if err != nil {
		// The request client never receives the body, so guard for an
		// io.Closer and close it here.
		closer, ok := body.(io.Closer)
		if !ok {
			return err
		}
		closeErr := closer.Close()
		return errors.Join(err, closeErr)
	}

	// a negative value means the caller does not know the length; -1 is the
	// sentinel that makes the request send the body in chunks
	if contentLength < 0 {
		contentLength = -1
	}
	req.ContentLength = contentLength

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
