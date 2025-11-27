package user

import (
	"fmt"
	"strings"
)

const (
	minUsernameLen = 2
	maxUsernameLen = 28
	minPasswordLen = 8
	specialChars   = "!@#$%^&*"
)

func validateCreateUserRequest(req *CreateUserRequest) error {
	if req.Username == "" {
		return fmt.Errorf("username is required")
	}
	if len(req.Username) < minUsernameLen {
		return fmt.Errorf("username must be at least %d characters long, got %d", minUsernameLen, len(req.Username))
	}
	if len(req.Username) > maxUsernameLen {
		return fmt.Errorf("username must be no more than %d characters long, got %d", maxUsernameLen, len(req.Username))
	}

	if req.Email == "" {
		return fmt.Errorf("email is required")
	}
	if err := validateEmail(req.Email); err != nil {
		return fmt.Errorf("invalid email: %w", err)
	}

	if err := validatePassword(req.Password); err != nil {
		return fmt.Errorf("invalid password: %w", err)
	}

	return nil
}

func validateEmail(email string) error {
	// Basic validation - at least has @ with text before and after, and a dot after @
	atIndex := strings.Index(email, "@")
	if atIndex <= 0 {
		return fmt.Errorf("must contain @ with text before it")
	}

	afterAt := email[atIndex+1:]
	if afterAt == "" || !strings.Contains(afterAt, ".") {
		return fmt.Errorf("must have a domain with a dot after @")
	}

	dotIndex := strings.LastIndex(afterAt, ".")
	if dotIndex == 0 || dotIndex == len(afterAt)-1 {
		return fmt.Errorf("invalid domain format")
	}

	return nil
}

func validatePassword(password string) error {
	if len(password) < minPasswordLen {
		return fmt.Errorf("must be at least %d characters, got %d", minPasswordLen, len(password))
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasDigit   bool
		hasSpecial bool
	)

	for _, c := range password {
		switch {
		case 'A' <= c && c <= 'Z':
			hasUpper = true
		case 'a' <= c && c <= 'z':
			hasLower = true
		case '0' <= c && c <= '9':
			hasDigit = true
		case strings.ContainsRune(specialChars, c):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return fmt.Errorf("must contain an uppercase letter")
	}
	if !hasLower {
		return fmt.Errorf("must contain a lowercase letter")
	}
	if !hasDigit {
		return fmt.Errorf("must contain a number")
	}
	if !hasSpecial {
		return fmt.Errorf("must contain a special character (%s)", specialChars)
	}

	return nil
}
