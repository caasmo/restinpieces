package oauth2

import ()

// AuthUser defines a standardized OAuth2 user data structure.
// we already havr user. remove.
type AuthUser struct {
	Expiry       types.DateTime `json:"expiry"`
	RawUser      map[string]any `json:"rawUser"`
	Id           string         `json:"id"`
	Name         string         `json:"name"`
	Username     string         `json:"username"`
	Email        string         `json:"email"`
	AvatarURL    string         `json:"avatarURL"`
	AccessToken  string         `json:"accessToken"`
	RefreshToken string         `json:"refreshToken"`
}

// for the moment each porvider has here a function to return the User given the raw extracted (pb) field from the provider
// data, err := p.FetchRawUserInfo(token)
// data is []byte

// TODO make general func User() with switch name from config
// idea, maybe derive password hash from token. user will not be able to log with password. 
// but will be able to request new password per email.
// check each provided to tell you if email verified by them, only by us if by them.
//func GoogleUser(data []byte, token?) (*User, error) {
//	//token *oauth2.Token
//	extracted := struct {
//		Id            string `json:"sub"`
//		Name          string `json:"name"`
//		Picture       string `json:"picture"`
//		Email         string `json:"email"`
//		EmailVerified bool   `json:"email_verified"`
//	}{}
//	if err := json.Unmarshal(data, &extracted); err != nil {
//		return nil, err
//	}
//
//	user := &User{
//		Id:           extracted.Id,
//		Name:         extracted.Name,
//		AvatarURL:    extracted.Picture,
//		//RawUser:      rawUser,
//		//AccessToken:  token.AccessToken,
//		//RefreshToken: token.RefreshToken,
//	}
//
//	if extracted.EmailVerified {
//		user.Email = extracted.Email
//	}
//
//
//
//
//}

