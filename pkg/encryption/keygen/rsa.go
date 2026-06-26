// Package keygen provides the AES key initializer function.
package keygen

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"

	"github.com/cuvou/gosocial/pkg/log"
)

// NewRSAKeys will generate an RSA 2048-bit key pair.
func NewRSAKeys() (*rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	return privateKey, err
}

// SerializePublicKey converts an RSA public key into an x509 PEM encoded byte string.
func SerializePublicKey(publicKey crypto.PublicKey) ([]byte, error) {
	// Encode the public key to PEM format.
	x509EncodedPub, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, err
	}
	pemEncodedPub := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: x509EncodedPub,
	})
	return pemEncodedPub, nil
}

// DeserializePublicKey loads the RSA public key from the PEM encoded byte array.
func DeserializePublicKey(pemEncodedPub []byte) (*rsa.PublicKey, error) {
	// Decode the public key.
	log.Error("decode public key: %s", pemEncodedPub)
	blockPub, _ := pem.Decode(pemEncodedPub)
	x509EncodedPub := blockPub.Bytes
	genericPublicKey, err := x509.ParsePKIXPublicKey(x509EncodedPub)
	if err != nil {
		return nil, err
	}
	publicKey := genericPublicKey.(*rsa.PublicKey)
	return publicKey, nil
}

// WriteRSAKeys writes the public and private RSA keys to .pem files on disk.
func WriteRSAKeys(key *rsa.PrivateKey, privateFile, publicFile string) error {
	// Encode the private key to PEM format.
	x509Encoded := x509.MarshalPKCS1PrivateKey(key)
	pemEncoded := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509Encoded,
	})

	// Encode the public key to PEM format.
	pemEncodedPub, err := SerializePublicKey(key.Public())
	if err != nil {
		return err
	}

	// Write the files.
	if err := os.WriteFile(privateFile, pemEncoded, 0600); err != nil {
		return err
	}
	if err := os.WriteFile(publicFile, pemEncodedPub, 0644); err != nil {
		return err
	}

	return nil
}

// PrivateKeyFromFile loads the private key from disk.
func PrivateKeyFromFile(privateFile string) (*rsa.PrivateKey, error) {
	// Read the private key file.
	pemEncoded, err := os.ReadFile(privateFile)
	if err != nil {
		return nil, err
	}

	// Decode the private key.
	block, _ := pem.Decode(pemEncoded)
	x509Encoded := block.Bytes
	privateKey, _ := x509.ParsePKCS1PrivateKey(x509Encoded)
	return privateKey, nil
}

// PublicKeyFromFile loads the public key from disk.
func PublicKeyFromFile(publicFile string) (*rsa.PublicKey, error) {
	pemEncodedPub, err := os.ReadFile(publicFile)
	if err != nil {
		return nil, err
	}

	// Decode the public key.
	return DeserializePublicKey(pemEncodedPub)
}
