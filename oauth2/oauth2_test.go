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

func TestUserFromUserInfoURL(t *testing.T) {
	testCases := []struct {
		name         string
		providerName string
		responseBody string
		wantUser     *db.User
		wantErr      error
	}{
		{
			name:         "google valid user",
			providerName: config.OAuth2ProviderGoogle,
			responseBody: `{"sub": "123", "name": "Test User", "picture": "http://example.com/avatar.png", "email": "test@example.com", "email_verified": true}`,
			wantUser: &db.User{
				ID:       "123",
				Email:    "test@example.com",
				Name:     "Test User",
				Avatar:   "http://example.com/avatar.png",
				Verified: true,
				Oauth2:   true,
			},
			wantErr: nil,
		},
		{
			name:         "google email not verified",
			providerName: config.OAuth2ProviderGoogle,
			responseBody: `{"sub": "123", "name": "Test User", "picture": "http://example.com/avatar.png", "email": "test@example.com", "email_verified": false}`,
			wantUser:     nil,
			wantErr:      errors.New("google email not verified"),
		},
		{
			name:         "unsupported provider",
			providerName: "facebook",
			responseBody: `{}`,
			wantUser:     nil,
			wantErr:      errors.New("unsupported provider: facebook"),
		},
		{
			name:         "malformed json",
			providerName: config.OAuth2ProviderGoogle,
			responseBody: `{"sub": "123", "name": "Test User",`,
			wantUser:     nil,
			wantErr:      errors.New("failed to decode google user info: unexpected EOF"),
		},
		{
			name:         "empty response body",
			providerName: config.OAuth2ProviderGoogle,
			responseBody: ``,
			wantUser:     nil,
			wantErr:      errors.New("failed to decode google user info: EOF"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock http.Response
			resp := &http.Response{
				Body: io.NopCloser(bytes.NewReader([]byte(tc.responseBody))),
			}

			user, err := UserFromUserInfoURL(resp, tc.providerName)

			// Check for error
			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("UserFromUserInfoURL() error = nil, want %v", tc.wantErr)
				}
				// Using strings.Contains because the json decoding error can be complex
				if !strings.Contains(err.Error(), tc.wantErr.Error()) {
					t.Errorf("UserFromUserInfoURL() error = %v, want error containing %v", err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("UserFromUserInfoURL() unexpected error = %v", err)
			}

			// Check for user struct equality
			if !reflect.DeepEqual(user, tc.wantUser) {
				t.Errorf("UserFromUserInfoURL() user = %v, want %v", user, tc.wantUser)
			}
		})
	}
}
