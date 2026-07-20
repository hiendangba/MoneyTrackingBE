package dto

type VerifyOTPRequest struct {
	Email   string `json:"email"`
	OTP     string `json:"otp"`
	Purpose string `json:"purpose"`
}
