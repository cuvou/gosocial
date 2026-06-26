// Package keygen provides the AES key initializer function.
package keygen

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
)

// NewAESKey returns a 32-byte (AES 256 bit) encryption key.
func NewAESKey() ([]byte, error) {
	var result = make([]byte, 32)
	_, err := rand.Read(result)
	return result, err
}

// EncryptWithAESKey a byte stream using a given AES key.
func EncryptWithAESKey(input []byte, key []byte) ([]byte, error) {
	// Generate a new AES cipher.
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// gcm or Galois/Counter Mode
	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	// Create a new byte array the size of the GCM nonce
	// which must be passed to Seal.
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("populating the nonce: %s", err)
	}

	// Encrypt the text using the Seal function.
	// Seal encrypts and authenticates plaintext, authenticates the
	// additional data and appends the result to dst, returning the
	// updated slice. The nonce must be NonceSize() bytes long and
	// unique for all time, for a given key.
	result := gcm.Seal(nonce, nonce, input, nil)
	return result, nil
}

func DecryptWithAESKey(data []byte, key []byte) ([]byte, error) {
	c, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext data less than nonceSize")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
