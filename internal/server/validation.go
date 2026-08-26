package server

import (
	"regexp"
	"unicode"
	"unicode/utf8"
)

const minimumPasswordLength = 8

var emailPattern = regexp.MustCompile(`^[A-Za-z0-9._%+-]+@[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+$`)

func isValidEmail(email string) bool {
	return emailPattern.MatchString(email)
}

func isValidPassword(password string) bool {
	if utf8.RuneCountInString(password) < minimumPasswordLength {
		return false
	}

	hasLetter := false
	hasNumber := false
	hasSpecialCharacter := false

	for _, character := range password {
		switch {
		case unicode.IsLetter(character):
			hasLetter = true
		case unicode.IsDigit(character):
			hasNumber = true
		case unicode.IsPunct(character), unicode.IsSymbol(character):
			hasSpecialCharacter = true
		}
	}

	return hasLetter && hasNumber && hasSpecialCharacter
}
