package oauth2

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/caasmo/restinpieces/db"
)

const (
	// ExternalAuthOAuth2 is the value used in the ExternalAuth field
	// to indicate OAuth2 authentication
	ExternalAuthOAuth2 = "oauth2"
)

// AuthUser defines a standardized OAuth2 user data structure.
// we already havr user. remove.
//type AuthUser struct {
//	Expiry       types.DateTime `json:"expiry"`
//	RawUser      map[string]any `json:"rawUser"`
//	Id           string         `json:"id"`
//	Name         string         `json:"name"`
//	Username     string         `json:"username"`
//	Email        string         `json:"email"`
//	AvatarURL    string         `json:"avatarURL"`
//	AccessToken  string         `json:"accessToken"`
//	RefreshToken string         `json:"refreshToken"`
//}

// UserFromInfoResponse maps provider-specific user info to our standard User struct
func UserFromInfoResponse(resp *http.Response, providerConfig *config.OAuth2ProviderConfig) (*db.User, error) {
	// Decode into generic map
	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode %s user info: %w", providerConfig.Name, err)
	}

	// Create user with default values
	user := &db.User{
		Verified: true,
		Oauth2:   true,
	}

	// Required fields
	idField := providerConfig.ResponseFields["id"]
	if idField == "" {
		return nil, fmt.Errorf("missing required field mapping: id")
	}
	if id, ok := raw[idField]; ok {
		user.ID = fmt.Sprintf("%v", id)
	} else {
		return nil, fmt.Errorf("missing required field: %s", idField)
	}

	emailField := providerConfig.ResponseFields["email"]
	if emailField == "" {
		return nil, fmt.Errorf("missing required field mapping: email")
	}
	if email, ok := raw[emailField]; ok {
		user.Email = fmt.Sprintf("%v", email)
	} else {
		return nil, fmt.Errorf("missing required field: %s", emailField)
	}

	// Optional fields
	if nameField := providerConfig.ResponseFields["name"]; nameField != "" {
		if name, ok := raw[nameField]; ok {
			user.Name = fmt.Sprintf("%v", name)
		}
	}

	if avatarField := providerConfig.ResponseFields["avatar"]; avatarField != "" {
		if avatar, ok := raw[avatarField]; ok {
			user.Avatar = fmt.Sprintf("%v", avatar)
		}
	}

	// Email verification
	if verifiedField := providerConfig.ResponseFields["email_verified"]; verifiedField != "" {
		if verified, ok := raw[verifiedField]; ok {
			if verifiedBool, ok := verified.(bool); ok && !verifiedBool {
				return nil, errors.New("email not verified")
			}
		}
	}

	return user, nil
}
