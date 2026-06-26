package keygen_test

import (
	"testing"

	"github.com/cuvou/gosocial/pkg/encryption/keygen"
)

func TestAES(t *testing.T) {
	type testCase struct {
		AESKey    []byte // AES key, nil = generate a new one
		Input     []byte // input text to encrypt
		Encrypted []byte // already encrypted text
		Expect    []byte // expected output on decrypt
	}

	var tests = []testCase{
		{
			Input:  []byte("hello world"),
			Expect: []byte("hello world"),
		},
		{
			AESKey:    []byte{170, 94, 243, 132, 85, 247, 149, 238, 245, 39, 140, 125, 226, 178, 134, 161, 17, 151, 139, 248, 16, 94, 165, 8, 102, 238, 214, 183, 86, 138, 219, 52},
			Encrypted: []byte{146, 217, 250, 254, 70, 201, 27, 221, 92, 145, 77, 213, 211, 197, 63, 189, 220, 188, 78, 8, 217, 108, 136, 89, 156, 23, 179, 54, 209, 54, 244, 170, 182, 150, 242, 52, 112, 191, 216, 46},
			Expect:    []byte("goodbye mars"),
		},
	}

	for i, test := range tests {
		if len(test.AESKey) == 0 {
			key, err := keygen.NewAESKey()
			if err != nil {
				t.Errorf("Test #%d: failed to generate new AES key: %s", i, err)
				continue
			}
			test.AESKey = key
		}

		if len(test.Encrypted) == 0 {
			enc, err := keygen.EncryptWithAESKey(test.Input, test.AESKey)
			if err != nil {
				t.Errorf("Test #%d: failed to encrypt input: %s", i, err)
				continue
			}
			test.Encrypted = enc
		}

		// t.Errorf("Key: %+v\nEnc: %+v", test.AESKey, test.Encrypted)

		dec, err := keygen.DecryptWithAESKey(test.Encrypted, test.AESKey)
		if err != nil {
			t.Errorf("Test #%d: failed to decrypt: %s", i, err)
			continue
		}

		// compare the results
		var ok = true
		if len(dec) != len(test.Expect) {
			ok = false
		} else {
			for j := range dec {
				if test.Expect[j] != dec[j] {
					ok = false
				}
			}
		}
		if !ok {
			t.Errorf("Test #%d: got unexpected result from decrypt. Expected %s, got %s", i, test.Expect, dec)
			continue
		}
	}
}
