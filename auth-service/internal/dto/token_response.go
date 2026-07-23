package dto

type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
}

type MobileRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type MobileLogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type CSRFResponse struct {
	CSRFToken string `json:"csrf_token"`
}
