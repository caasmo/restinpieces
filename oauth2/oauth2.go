package oauth2

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
)

// userInfoMaxBytes caps how much of a provider's user-info response is read.
// A user info document is small (id, email, name, picture URL); a provider
// that streams more is misbehaving or compromised, and no response may grow
// the process memory without bound.
const userInfoMaxBytes = 1 << 20 // 1 MiB (1,048,576 bytes)

// UserFromUserInfo maps provider-specific user info to our standard User struct.
// The response body is read through userInfoMaxBytes; anything larger fails to
// decode.
//
// When integrating a new OAuth2 provider, first add a case statement for the new provider name.
// Then create a raw struct that matches the provider's user info JSON response.
// Map the fields to the standard db.User struct while ensuring required fields
// like email verification are properly validated before returning.
func UserFromUserInfoURL(resp *http.Response, providerName string) (*db.User, error) {
	limitedBody := io.LimitReader(resp.Body, userInfoMaxBytes)

	switch providerName {
	case config.OAuth2ProviderGoogle:

		// raw info endpoint response fields (info from pocketbase)
		var raw struct {
			Id            string `json:"sub"`
			Name          string `json:"name"`
			Picture       string `json:"picture"`
			Email         string `json:"email"`
			EmailVerified bool   `json:"email_verified"`
		}

		err := json.NewDecoder(limitedBody).Decode(&raw)
		if err != nil {
			return nil, fmt.Errorf("failed to decode google user info: %w", err)
		}

		if !raw.EmailVerified {
			return nil, errors.New("google email not verified")
		}

		return &db.User{
			ID:       raw.Id,
			Email:    raw.Email,
			Name:     raw.Name,
			Avatar:   raw.Picture,
			Verified: true,
			Oauth2:   true,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported provider: %s", providerName)
	}
}
