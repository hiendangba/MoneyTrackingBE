package utils

import (
	"fmt"
	"regexp"
	"strings"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
var menuCodeRegex = regexp.MustCompile(`^[a-z0-9]+(?:[-_][a-z0-9]+)*$`)

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func ValidateEmail(email string) error {
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email")
	}
	return nil
}

func ValidatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	return nil
}

func ValidateOTP(otp string) error {
	if len(otp) != 6 {
		return fmt.Errorf("otp must be 6 digits")
	}
	for _, ch := range otp {
		if ch < '0' || ch > '9' {
			return fmt.Errorf("otp must contain digits only")
		}
	}
	return nil
}

func ValidateMenuCode(code string) error {
	if code == "" {
		return fmt.Errorf("code is required")
	}
	if !menuCodeRegex.MatchString(code) {
		return fmt.Errorf("code must be slug-like and contain lowercase letters, numbers, hyphen, or underscore")
	}
	return nil
}

func ValidateMenuRoute(route string) error {
	if route == "" {
		return nil
	}
	if !strings.HasPrefix(route, "/") {
		return fmt.Errorf("route must start with /")
	}
	return nil
}
