package s3

import "testing"

func TestResponseError_Error(t *testing.T) {
	testCases := []struct {
		name     string
		err      *ResponseError
		expected string
	}{
		{
			name:     "code and message",
			err:      &ResponseError{Status: 403, Code: "AccessDenied", Message: "Access Denied"},
			expected: "403 AccessDenied: Access Denied",
		},
		{
			name:     "code only",
			err:      &ResponseError{Status: 404, Code: "NoSuchKey"},
			expected: "404 NoSuchKey",
		},
		{
			name:     "message only",
			err:      &ResponseError{Status: 500, Message: "Internal Error"},
			expected: "500 S3ResponseError: Internal Error",
		},
		{
			name:     "status only",
			err:      &ResponseError{Status: 500},
			expected: "500 S3ResponseError",
		},
		{
			name:     "raw body",
			err:      &ResponseError{Status: 500, Raw: []byte("<Error/>")},
			expected: "500 S3ResponseError\n(RAW: <Error/>)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.Error()
			if got != tc.expected {
				t.Errorf("Error() = %q, want %q", got, tc.expected)
			}
		})
	}
}
