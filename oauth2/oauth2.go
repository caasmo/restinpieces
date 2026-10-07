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

// ErrUserInfoFetch reports that the user-info request itself failed: it could
// not be made, or the provider answered with a status other than 200.
var ErrUserInfoFetch = errors.New("failed to fetch oauth2 user info")

// UserFromUserInfo fetches the provider's user-info document from userInfoURL
// and maps it to our standard User struct. The response body is read through
// userInfoMaxBytes; anything larger fails to decode.
//
// When integrating a new OAuth2 provider, first add a case statement for the new provider name.
// Then create a raw struct that matches the provider's user info JSON response.
// Map the fields to the standard db.User struct while ensuring required fields
// like email verification are properly validated before returning.
func UserFromUserInfo(client *http.Client, userInfoURL, providerName string) (*db.User, error) {
	resp, err := client.Get(userInfoURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUserInfoFetch, err)
	}

	// The response body is an open network stream; closing it lets the
	// transport reuse the connection.
	defer func() { _ = resp.Body.Close() }()

	// SECURITY: Only HTTP 200 carries user info. A redirect (followed silently
	// by http.Client) or a provider error page would otherwise reach the
	// decoder and surface as a misleading decoding error.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: provider %s returned HTTP status %d", ErrUserInfoFetch, providerName, resp.StatusCode)
	}

	limitedBody := io.LimitReader(resp.Body, userInfoMaxBytes)

	switch providerName {
	case config.OAuth2Google:

		// raw info endpoint response fields (info from pocketbase)
		var raw struct {
			Name          string `json:"name"`
			Picture       string `json:"picture"`
			Email         string `json:"email"`
			EmailVerified bool   `json:"email_verified"`
		}

		decodeErr := json.NewDecoder(limitedBody).Decode(&raw)
		if decodeErr != nil {
			return nil, fmt.Errorf("failed to decode google user info: %w", decodeErr)
		}

		if !raw.EmailVerified {
			return nil, errors.New("google email not verified")
		}

		// The provider's sub is intentionally not stored: the DB assigns
		// User.ID and accounts are deduplicated by email.
		return &db.User{
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
