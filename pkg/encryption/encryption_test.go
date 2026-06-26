package encryption_test

import (
	"testing"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/encryption"
)

func TestEncryption(t *testing.T) {
	var tests = []struct {
		Input  []byte
		Output []byte
		Key    []byte
	}{
		{
			Input:  []byte("Hello, world!"),
			Output: []byte("Hello, world!"),
			Key:    []byte("passphrasewhichneedstobe32bytes!"),
		},
	}

	for i, tc := range tests {
		if len(tc.Key) != 32 {
			t.Errorf("Test #%d: key is not 32 bytes", i)
			continue
		}

		config.Current.Encryption.AESKey = tc.Key
		cipher, err := encryption.Encrypt(tc.Input)
		if err != nil {
			t.Errorf("Test #%d: unexpected error from Encrypt: %s", i, err)
			continue
		}

		result, err := encryption.Decrypt(cipher)
		if err != nil {
			t.Errorf("Test #%d: unexpected error from Decrypt: %s", i, err)
			continue
		}

		if !EqualSlice(result, tc.Output) {
			t.Errorf("Test #%d: didn't get expected decrypted output", i)
		}
	}
}

func TestNonces(t *testing.T) {
	// Verify that the same text encrypted twice has a different output (nonce),
	// but both decrypt all the same.
	var (
		key       = []byte("passphrasewhichneedstobe32bytes!")
		plaintext = []byte("Hello, world!!")
	)

	config.Current.Encryption.AESKey = key

	// Encrypt them both.
	cipherA, err := encryption.Encrypt(plaintext)
	if err != nil {
		t.Errorf("Unexpected failure when encrypting cipherA: %s", err)
	}

	cipherB, err := encryption.Encrypt(plaintext)
	if err != nil {
		t.Errorf("Unexpected failure when encrypting cipherB: %s", err)
	}

	// They should not be equal.
	if EqualSlice(cipherA, cipherB) {
		t.Errorf("The two ciphertexts were unexpectedly equal!")
	}

	// Decrypt them both.
	resultA, err := encryption.Decrypt(cipherA)
	if err != nil {
		t.Errorf("Unexpected failure when decrypting cipherA: %s", err)
	}

	resultB, err := encryption.Decrypt(cipherB)
	if err != nil {
		t.Errorf("Unexpected failure when decrypting cipherB: %s", err)
	}

	// Expect them to be equal.
	if !EqualSlice(resultA, resultB) {
		t.Errorf("The two decrypted slices were expected to be equal, but were not!")
	}
}

func EqualSlice(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}

	for i, value := range a {
		if b[i] != value {
			return false
		}
	}

	return true
}
