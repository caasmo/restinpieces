package oauth2

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/caasmo/restinpieces/db"
)

// UserFromUserInfo maps provider-specific user info to our standard User struct
func UserFromUserInfoURL(resp *http.Response, providerName string) (*db.User, error) {
	switch providerName {
	case "google":
		
		// raw info endpoint response fields (from pocketbase)
		var raw struct {
			Id            string `json:"sub"`
			Name          string `json:"name"`
			Picture       string `json:"picture"`
			Email         string `json:"email"`
			EmailVerified bool   `json:"email_verified"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
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
