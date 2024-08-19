// Package secret provides a way to read user credentials.
package secret

import (
	"encoding/base64"
	"errors"
	"os"
	"strings"
)

var (
	// ErrInvalidSecret is returned when the secret is invalid.
	ErrInvalidSecret = errors.New("invalid secret")
)

// Reader is a secret reader from a file.
type Reader struct {
	FilePath string
}

// NewReader creates a new secret reader.
func NewReader(filePath string) *Reader {
	return &Reader{
		FilePath: filePath,
	}
}

// Read reads the username and password from the file.
//
// Format is 'usernameb64:passwordb64' with usernameb64 and passwordb64 encoded in base64 (like basic authentication).
func (s *Reader) Read() (username string, password string, err error) {
	b, err := os.ReadFile(s.FilePath)
	if err != nil {
		return "", "", err
	}

	usernameB64, passwordB64, found := strings.Cut(strings.TrimSpace(string(b)), ":")
	if !found {
		return "", "", ErrInvalidSecret
	}

	usernameB, err := base64.StdEncoding.DecodeString(usernameB64)
	if err != nil {
		return "", "", err
	}

	passwordB, err := base64.StdEncoding.DecodeString(passwordB64)
	if err != nil {
		return "", "", err
	}

	return string(usernameB), string(passwordB), nil
}

// UserPasswordFromEnv is a user password reader from the environment.
type UserPasswordFromEnv struct{}

// Read returns the email and password from the environment.
func (UserPasswordFromEnv) Read() (email, password string, err error) {
	email = os.Getenv("WITHNY_EMAIL")
	password = os.Getenv("WITHNY_PASSWORD")
	return
}

// UserPasswordStatic is a static user password reader.
type UserPasswordStatic struct {
	Email    string
	Password string
}

// Read returns the email and password.
func (u UserPasswordStatic) Read() (email, password string, err error) {
	return u.Email, u.Password, nil
}
