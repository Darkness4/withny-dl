// Package secret provides a way to read user credentials.
package secret

import (
	"errors"
	"os"

	"github.com/Darkness4/withny-dl/withny/api"
	"gopkg.in/yaml.v3"
)

var (
	// ErrInvalidSecret is returned when the secret is invalid.
	ErrInvalidSecret = errors.New("invalid secret")
)

var _ api.CredentialsReader = (*Reader)(nil)

// ReadCredentialFile reads the credentials from a file.
func ReadCredentialFile(path string) (api.SavedCredentials, error) {
	var cf api.SavedCredentials
	b, err := os.ReadFile(path)
	if err != nil {
		return cf, err
	}
	err = yaml.Unmarshal(b, &cf)
	return cf, err
}

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
func (s *Reader) Read() (api.SavedCredentials, error) {
	creds, err := ReadCredentialFile(s.FilePath)
	return creds, err
}

var _ api.CredentialsReader = (*CredentialsFromEnv)(nil)

// CredentialsFromEnv is a user password reader from the environment.
type CredentialsFromEnv struct{}

// Read returns the email and password from the environment.
func (CredentialsFromEnv) Read() (api.SavedCredentials, error) {
	return api.SavedCredentials{
		Username:     os.Getenv("WITHNY_USERNAME"),
		Password:     os.Getenv("WITHNY_PASSWORD"),
		Token:        os.Getenv("WITHNY_ACCESS_TOKEN"),
		RefreshToken: os.Getenv("WITHNY_REFRESH_TOKEN"),
	}, nil
}

var _ api.CredentialsReader = (*Static)(nil)

// Static is a static user password reader.
type Static struct {
	api.SavedCredentials
}

// Read returns the email and password.
func (u Static) Read() (api.SavedCredentials, error) {
	return u.SavedCredentials, nil
}
