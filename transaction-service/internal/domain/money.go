package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var ErrInvalidDecimal = errors.New("invalid decimal")

func ParseMoney(raw string) (int64, error) {
	return parseScaledDecimal(raw, 2)
}

func ParseRatio(raw string) (int64, error) {
	return parseScaledDecimal(raw, 6)
}

func FormatMoney(minor int64) string {
	sign := ""
	if minor < 0 {
		sign = "-"
		minor = -minor
	}
	return fmt.Sprintf("%s%d.%02d", sign, minor/100, minor%100)
}

func FormatRatio(value int64) string {
	sign := ""
	if value < 0 {
		sign = "-"
		value = -value
	}
	return fmt.Sprintf("%s%d.%06d", sign, value/1_000_000, value%1_000_000)
}

func parseScaledDecimal(raw string, scale int) (int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, ErrInvalidDecimal
	}
	if strings.HasPrefix(value, "+") {
		value = strings.TrimPrefix(value, "+")
	}
	if strings.HasPrefix(value, "-") {
		return 0, ErrInvalidDecimal
	}
	parts := strings.Split(value, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, ErrInvalidDecimal
	}
	if len(parts) == 2 && len(parts[1]) > scale {
		return 0, ErrInvalidDecimal
	}
	for len(parts) == 1 || len(parts[1]) < scale {
		if len(parts) == 1 {
			parts = append(parts, "")
		}
		parts[1] += "0"
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, ErrInvalidDecimal
	}
	fraction := int64(0)
	if scale > 0 {
		fraction, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return 0, ErrInvalidDecimal
		}
	}
	multiplier := int64(1)
	for range scale {
		multiplier *= 10
	}
	if whole > (int64(^uint64(0)>>1)-fraction)/multiplier {
		return 0, ErrInvalidDecimal
	}
	return whole*multiplier + fraction, nil
}
