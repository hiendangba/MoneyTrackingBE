package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"

	"golang.org/x/crypto/bcrypt"
)

func GenerateOTP(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("invalid otp length")
	}

	digits := make([]byte, length)
	for i := range digits {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("generate otp digit: %w", err)
		}
		digits[i] = byte('0' + n.Int64())
	}

	return string(digits), nil
}

func HashOTP(otp string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(otp), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash otp: %w", err)
	}
	return string(hash), nil
}

func CompareOTP(hash, otp string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(otp)); err != nil {
		return fmt.Errorf("compare otp: %w", err)
	}
	return nil
}
