package secret

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/Darkness4/withny-dl/withny/api"
)

var _ api.CredentialsCache = (*FileCache)(nil)

var (
	// Hard-coded private key to encrypt the credentials. This is obviously not secure but permits avoiding plain text credentials.
	hardcodedSecret = []byte(
		"withny-dl-secret-key-0123456789a",
	)
)

// EncryptWriter implements io.Writer interface and writes encrypted data.
type EncryptWriter struct {
	writer io.Writer
	stream cipher.Stream
}

// NewEncryptWriter initializes an EncryptWriter with a CFB stream cipher.
func NewEncryptWriter(w io.Writer, key []byte) (*EncryptWriter, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	// Write IV to the writer first
	if _, err := w.Write(iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	return &EncryptWriter{
		writer: w,
		stream: stream,
	}, nil
}

// Write encrypts the data and writes it to the underlying writer.
func (ew *EncryptWriter) Write(p []byte) (n int, err error) {
	encrypted := make([]byte, len(p))
	ew.stream.XORKeyStream(encrypted, p)
	return ew.writer.Write(encrypted)
}

// DecryptReader implements io.Reader interface and reads decrypted data.
type DecryptReader struct {
	reader io.Reader
	stream cipher.Stream
}

// NewDecryptReader initializes a DecryptReader with a CFB stream cipher.
func NewDecryptReader(r io.Reader, key []byte) (*DecryptReader, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(r, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCFBDecrypter(block, iv)
	return &DecryptReader{
		reader: r,
		stream: stream,
	}, nil
}

// Read decrypts the data and reads it from the underlying reader.
func (dr *DecryptReader) Read(p []byte) (n int, err error) {
	n, err = dr.reader.Read(p)
	if err != nil {
		return n, err
	}
	dr.stream.XORKeyStream(p[:n], p[:n])
	return n, nil
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

	decryptReader, err := NewDecryptReader(file, hardcodedSecret)
	if err != nil {
		return creds, err
	}

	if err := json.NewDecoder(decryptReader).Decode(&creds); err != nil {
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
	encryptWriter, err := NewEncryptWriter(file, hardcodedSecret)
	if err != nil {
		return err
	}

	return json.NewEncoder(encryptWriter).Encode(creds)
}

// Invalidate removes the credentials file.
func (f *FileCache) Invalidate() error {
	return os.Remove(f.FilePath)
}
