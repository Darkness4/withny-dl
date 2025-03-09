package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/crypto/pbkdf2"

	"github.com/Darkness4/withny-dl/withny/api"
)

var _ api.CredentialsCache = (*FileCache)(nil)

var (
	// Hard-coded private key to encrypt the credentials. This is obviously not secure but permits avoiding plain text credentials.
	hardcodedSecret = []byte(
		"withny-dl-secret-key-0123456789a",
	)
)

const saltSize = 16

// DeriveKey derives a 32-byte AES key from the secret key using PBKDF2.
func deriveKey(secret []byte) []byte {
	// PBKDF2 is used to derive a key from the secret key
	salt := make([]byte, saltSize) // You can use a random salt in production
	return pbkdf2.Key(secret, salt, 100000, 32, sha256.New)
}

// Encrypt creates a new EncryptWriter.
func Encrypt(w io.Writer, secret []byte, plaintext []byte) error {
	// Derive the key from the secret
	key := deriveKey(secret)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("cannot create cipher: %v", err)
	}

	// Create GCM cipher
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("cannot create GCM cipher: %v", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return fmt.Errorf("cannot generate nonce: %v", err)
	}

	// Storing the nonce in the ciphertext since we have no storage.
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	_, err = w.Write(ciphertext)
	return err
}

// Decrypt reads the encrypted data from the reader and returns the decrypted data.
func Decrypt(r io.Reader, secret []byte) ([]byte, error) {
	// Derive the key from the secret
	key := deriveKey(secret)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("cannot create AES cipher: %v", err)
	}

	// Create GCM cipher
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cannot create GCM cipher: %v", err)
	}

	// Read the nonce from the reader (it will be the first part of the encrypted data)
	nonce := make([]byte, gcm.NonceSize())
	_, err = io.ReadFull(r, nonce)
	if err != nil {
		return nil, fmt.Errorf("cannot read nonce: %v", err)
	}

	// Read the ciphertext from the reader
	ciphertext, err := io.ReadAll(r)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("cannot read ciphertext: %v", err)
	}

	// Decrypt the data
	plainText, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("cannot decrypt data: %v", err)
	}

	return plainText, nil
}

// FileCache is a secret cache that reads from a file.
type FileCache struct {
	FilePath string
}

// NewFileCache creates a new file cache.
func NewFileCache(filePath string) *FileCache {
	return &FileCache{
		FilePath: filePath,
	}
}

// NewTmpCache creates a new temporary cache.
func NewTmpCache() *FileCache {
	return &FileCache{
		FilePath: os.TempDir() + "/withny-dl.json",
	}
}

// Get reads the credentials from a file.
func (f *FileCache) Get() (api.Credentials, error) {
	var creds api.Credentials

	file, err := os.Open(f.FilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return creds, errors.New("file does not exist")
		}
		return creds, err
	}
	defer file.Close()

	decrypted, err := Decrypt(file, hardcodedSecret)
	if err != nil {
		return creds, err
	}

	if err := json.Unmarshal(decrypted, &creds); err != nil {
		return creds, err
	}

	return creds, nil
}

// Set writes the credentials to a file.
func (f *FileCache) Set(creds api.Credentials) error {
	file, err := os.OpenFile(f.FilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	// Encrypt the JSON data and write it to the writer
	decrypted, err := json.Marshal(creds)
	if err != nil {
		return err
	}

	return Encrypt(file, hardcodedSecret, decrypted)
}

// Invalidate removes the credentials file.
func (f *FileCache) Invalidate() error {
	return os.Remove(f.FilePath)
}
