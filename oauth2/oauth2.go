package oauth2

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/caasmo/restinpieces/config"
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

// UserFromUserInfoURL maps provider-specific user info to our standard User struct
func UserFromUserInfoURL(resp *http.Response, providerConfig *config.OAuth2Provider) (*db.User, error) {
	// Decode into string map
	var raw map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode %s user info: %w", providerConfig.Name, err)
	}

	// Create user with default values
	user := &db.User{
		Verified: true,
		Oauth2:   true,
		ExternalAuth: config.ExternalAuthOAuth2,
	}

	// Process required fields
	for _, field := range providerConfig.UserInfoFields.Required() {
		fieldMapping := providerConfig.UserInfoFields[field]
		if fieldMapping == "" {
			return nil, fmt.Errorf("missing required field mapping for: %s", field)
		}
		
		value, ok := raw[fieldMapping]
		if !ok {
			return nil, fmt.Errorf("missing required field: %s", fieldMapping)
		}

		switch field {
		case config.UserInfoFieldEmail:
			user.Email = value
		}
	}

	// Process optional fields  
	for _, field := range providerConfig.UserInfoFields.Optional() {
		fieldMapping := providerConfig.UserInfoFields[field]
		if fieldMapping == "" {
			continue
		}
		
		value, ok := raw[fieldMapping]
		if !ok {
			continue
		}

		switch field {
		case config.UserInfoFieldName:
			user.Name = value
		case config.UserInfoFieldAvatar:
			user.Avatar = value
		case config.UserInfoFieldEmailVerified:
			if value == "false" {
				return nil, errors.New("email not verified")
			}
		}
	}

	return user, nil
}
