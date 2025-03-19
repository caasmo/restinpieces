package oauth2

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/caasmo/restinpieces/config"
	"github.com/caasmo/restinpieces/db"
)

// UserFromUserInfoURL maps provider-specific user info to our standard User struct
func UserFromUserInfoURL(resp *http.Response, providerConfig *config.OAuth2Provider) (*db.User, error) {
	// Decode into interface map to handle both string and boolean values
	var raw map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode %s user info: %w", providerConfig.Name, err)
	}

	// Create user with default values
	user := &db.User{
		Verified: true,
		Oauth2:   true,
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
			user.Email = fmt.Sprintf("%v", value)
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
			user.Name = fmt.Sprintf("%v", value)
		case config.UserInfoFieldAvatar:
			user.Avatar = fmt.Sprintf("%v", value)
		case config.UserInfoFieldEmailVerified:
			if verified, ok := value.(bool); ok && !verified {
				return nil, errors.New("email not verified")
			}
		}
	}

	return user, nil
}
