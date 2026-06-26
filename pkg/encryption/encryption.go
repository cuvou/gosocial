// Package encryption provides functions to encode/decode AES encrypted secrets.
//
// Encryption is used to store sensitive information in the database, such as 2FA TOTP secrets
// for users who have 2FA authentication enabled.
//
// For new key generation, see pkg/config/variable.go#NewAESKey.
package encryption

import (
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/encryption/keygen"
)

// Encrypt a byte stream using the site's AES passphrase.
func Encrypt(input []byte) ([]byte, error) {
	if len(config.Current.Encryption.AESKey) == 0 {
		return nil, errors.New("AES key not configured")
	}

	return keygen.EncryptWithAESKey(input, config.Current.Encryption.AESKey)
}

// EncryptString encrypts a string value and returns the cipher text.
func EncryptString(input string) ([]byte, error) {
	return Encrypt([]byte(input))
}

// Decrypt a byte stream using the site's AES passphrase.
func Decrypt(data []byte) ([]byte, error) {
	if len(config.Current.Encryption.AESKey) == 0 {
		return nil, errors.New("AES key not configured")
	}

	return keygen.DecryptWithAESKey(data, config.Current.Encryption.AESKey)
}

// DecryptString decrypts a string value from ciphertext.
func DecryptString(data []byte) (string, error) {
	decoded, err := Decrypt(data)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

// Hash a byte array as SHA256 and returns the hex string.
func Hash(input []byte) string {
	h := sha256.New()
	h.Write(input)
	return fmt.Sprintf("%x", h.Sum(nil))
}

// VerifyHash hashes a byte array and checks the result.
func VerifyHash(input []byte, expect string) bool {
	return Hash(input) == expect
}
