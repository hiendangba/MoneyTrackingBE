package domain

import "time"

const (
	OTPPurposeRegister = "register"
	OTPPurposeReset    = "reset_password"
)

type OTPPayload struct {
	Email           string                `json:"email"`
	Purpose         string                `json:"purpose"`
	OTPHash         string                `json:"otp_hash"`
	ExpiresAt       time.Time             `json:"expires_at"`
	AttemptCount    int                   `json:"attempt_count"`
	RegisterPayload *RegisterDraftPayload `json:"register_payload,omitempty"`
}

type RegisterDraftPayload struct {
	Fullname string `json:"fullname"`
	Email    string `json:"email"`
	Password string `json:"password"`
}
