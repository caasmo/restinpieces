package oauth2

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
)

// roundTripFunc adapts a function to an http.RoundTripper for tests.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestUserFromUserInfo(t *testing.T) {
	testCases := []struct {
		name         string
		providerName string
		statusCode   int
		responseBody string
		wantUser     *db.User
		wantErrText  string
		wantErrIs    error
	}{
		{
			name:         "google valid user",
			providerName: config.OAuth2Google,
			statusCode:   http.StatusOK,
			responseBody: `{"sub": "123", "name": "Test User", "picture": "http://example.com/avatar.png", "email": "test@example.com", "email_verified": true}`,
			wantUser: &db.User{
				ID:       "123",
				Email:    "test@example.com",
				Name:     "Test User",
				Avatar:   "http://example.com/avatar.png",
				Verified: true,
				Oauth2:   true,
			},
		},
		{
			name:         "google email not verified",
			providerName: config.OAuth2Google,
			statusCode:   http.StatusOK,
			responseBody: `{"sub": "123", "name": "Test User", "picture": "http://example.com/avatar.png", "email": "test@example.com", "email_verified": false}`,
			wantErrText:  "google email not verified",
		},
		{
			name:         "unsupported provider",
			providerName: "facebook",
			statusCode:   http.StatusOK,
			responseBody: `{}`,
			wantErrText:  "unsupported provider: facebook",
		},
		{
			name:         "malformed json",
			providerName: config.OAuth2Google,
			statusCode:   http.StatusOK,
			responseBody: `{"sub": "123", "name": "Test User",`,
			wantErrText:  "failed to decode google user info: unexpected EOF",
		},
		{
			name:         "empty response body",
			providerName: config.OAuth2Google,
			statusCode:   http.StatusOK,
			responseBody: ``,
			wantErrText:  "failed to decode google user info: EOF",
		},
		{
			name:         "body exceeds read limit",
			providerName: config.OAuth2Google,
			statusCode:   http.StatusOK,
			responseBody: `{"sub": "123", "name": "` + strings.Repeat("a", userInfoMaxBytes) + `", "email": "test@example.com", "email_verified": true}`,
			wantErrText:  "failed to decode google user info: unexpected EOF",
		},
		{
			name:         "non-200 status",
			providerName: config.OAuth2Google,
			statusCode:   http.StatusInternalServerError,
			responseBody: `{"error": "server error"}`,
			wantErrIs:    ErrUserInfoFetch,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			client := &http.Client{
				Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
					return &http.Response{
						StatusCode: tc.statusCode,
						Body:       io.NopCloser(bytes.NewReader([]byte(tc.responseBody))),
					}, nil
				}),
			}

			user, err := UserFromUserInfo(client, "https://example.com/userinfo", tc.providerName)

			if tc.wantErrText != "" || tc.wantErrIs != nil {
				if err == nil {
					t.Fatalf("UserFromUserInfo() error = nil, want error")
				}
				// Using strings.Contains because the json decoding error can be complex
				if tc.wantErrText != "" && !strings.Contains(err.Error(), tc.wantErrText) {
					t.Errorf("UserFromUserInfo() error = %v, want error containing %v", err, tc.wantErrText)
				}
				if tc.wantErrIs != nil && !errors.Is(err, tc.wantErrIs) {
					t.Errorf("UserFromUserInfo() error = %v, want errors.Is %v", err, tc.wantErrIs)
				}
			} else if err != nil {
				t.Fatalf("UserFromUserInfo() unexpected error = %v", err)
			}

			if !reflect.DeepEqual(user, tc.wantUser) {
				t.Errorf("UserFromUserInfo() user = %v, want %v", user, tc.wantUser)
			}
		})
	}
}

func TestUserFromUserInfo_TransportError(t *testing.T) {
	client := &http.Client{
		Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			return nil, errors.New("dial failed")
		}),
	}

	_, err := UserFromUserInfo(client, "https://example.com/userinfo", config.OAuth2Google)
	if !errors.Is(err, ErrUserInfoFetch) {
		t.Errorf("UserFromUserInfo() error = %v, want errors.Is %v", err, ErrUserInfoFetch)
	}
}
