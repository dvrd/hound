package keystore_test

import (
	"testing"

	"github.com/dvrd/hound/internal/keystore"
)

func TestValidatePasswordStrengthValid(t *testing.T) {
	valid := []string{
		"MyP@ssw0rd123",
		"Str0ng!Pass#1",
		"Abcdefghijk1!",
		"UPPER_lower_1!",
	}

	for _, pw := range valid {
		t.Run(pw, func(t *testing.T) {
			if err := keystore.ValidatePasswordStrength(pw); err != nil {
				t.Errorf("ValidatePasswordStrength(%q) = %v, want nil", pw, err)
			}
		})
	}
}

func TestValidatePasswordStrengthTooShort(t *testing.T) {
	err := keystore.ValidatePasswordStrength("Sh0rt!")
	if err == nil {
		t.Error("password too short should return error")
	}
}

func TestValidatePasswordStrengthNoUppercase(t *testing.T) {
	err := keystore.ValidatePasswordStrength("nouppercase1!")
	if err == nil {
		t.Error("password without uppercase should return error")
	}
}

func TestValidatePasswordStrengthNoLowercase(t *testing.T) {
	err := keystore.ValidatePasswordStrength("NOLOWERCASE1!")
	if err == nil {
		t.Error("password without lowercase should return error")
	}
}

func TestValidatePasswordStrengthNoDigit(t *testing.T) {
	err := keystore.ValidatePasswordStrength("NoDigitHere!!")
	if err == nil {
		t.Error("password without digit should return error")
	}
}

func TestValidatePasswordStrengthNoSpecial(t *testing.T) {
	err := keystore.ValidatePasswordStrength("NoSpecial1234")
	if err == nil {
		t.Error("password without special character should return error")
	}
}

func TestValidatePasswordStrengthEmpty(t *testing.T) {
	err := keystore.ValidatePasswordStrength("")
	if err == nil {
		t.Error("empty password should return error")
	}
}

func TestValidatePasswordStrengthExactly12Chars(t *testing.T) {
	// Exactly 12 chars with all requirements
	err := keystore.ValidatePasswordStrength("Abcdefghij1!")
	if err != nil {
		t.Errorf("12-char valid password should pass: %v", err)
	}
}

func TestValidatePasswordStrength11Chars(t *testing.T) {
	// 11 chars — should fail
	err := keystore.ValidatePasswordStrength("Abcdefghi1!")
	if err == nil {
		t.Error("11-char password should return error")
	}
}
