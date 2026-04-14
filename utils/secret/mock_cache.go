package secret

import (
	"github.com/Darkness4/withny-dl/withny/api"
)

var _ api.CredentialsCache = (*MockCache)(nil)

// MockCache is an in-memory implementation of api.CredentialsCache for unit testing.
type MockCache struct {
	Credentials api.CachedCredentials
	Hash        string
	Invalidated bool
}

// NewMockCache initializes a mock cache with default values.
func NewMockCache() *MockCache {
	return &MockCache{
		Credentials: api.CachedCredentials{
			Credentials: api.Credentials{
				AccessToken:  "mock-access-token",
				SessionToken: "mock-session-token",
			},
			Hash: "mock-initial-hash",
		},
	}
}

// Get returns the in-memory credentials without any decryption.
func (m *MockCache) Get() (api.CachedCredentials, error) {
	if m.Invalidated {
		return api.CachedCredentials{}, nil
	}
	return m.Credentials, nil
}

// Set updates the in-memory credentials.
func (m *MockCache) Set(creds api.Credentials) error {
	m.Credentials.AccessToken = creds.AccessToken
	m.Credentials.SessionToken = creds.SessionToken
	m.Invalidated = false
	return nil
}

// Init sets the hash and clears tokens, simulating a fresh state.
func (m *MockCache) Init(hash string) error {
	m.Hash = hash
	m.Credentials.AccessToken = ""
	m.Credentials.SessionToken = ""
	m.Invalidated = false
	return nil
}

// Invalidate simulates the removal of the cache.
func (m *MockCache) Invalidate() error {
	m.Invalidated = true
	m.Credentials = api.CachedCredentials{}
	return nil
}
