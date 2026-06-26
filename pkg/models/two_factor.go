package models

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image/png"
	"math/rand"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/encryption"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TwoFactor table to hold 2FA TOTP tokens for more secure login.
type TwoFactor struct {
	UserID          uint64 `gorm:"primaryKey"` // owner ID
	Enabled         bool
	EncryptedSecret []byte // encrypted OTP secret (URL format)
	HashedSecret    string // verification hash for the EncryptedSecret being decoded correctly
	BackupCodes     []byte // encrypted backup codes
	CreatedAt       time.Time
	UpdatedAt       time.Time

	// Private vars
	isNew bool // needs creation, didn't exist in DB
}

// IsNew returns if the 2FA record was freshly generated (not in DB yet).
func (tf *TwoFactor) IsNew() bool {
	return tf.isNew
}

// New2FA initializes a TwoFactor config for a user, with randomly generated secrets.
func New2FA(userID uint64) *TwoFactor {
	var tf = &TwoFactor{
		isNew:  true,
		UserID: userID,
	}

	// Generate backup codes.
	if err := tf.GenerateBackupCodes(); err != nil {
		log.Error("New2FA(%d): GenerateBackupCodes: %s", userID, err)
	}
	return tf
}

// Get2FA looks up the TwoFactor config for a user, or returns an empty struct ready to initialize.
func Get2FA(userID uint64) *TwoFactor {
	var (
		tf     = &TwoFactor{}
		result = DB.First(&tf, userID)
	)
	if result.Error != nil {
		return New2FA(userID)
	}
	return tf
}

// SetSecret sets (and encrypts) the EncryptedSecret.
func (tf *TwoFactor) SetSecret(url string) error {
	// Get the hash of the original secret for verification.
	hash := encryption.Hash([]byte(url))

	// Encrypt it.
	ciphertext, err := encryption.EncryptString(url)
	if err != nil {
		return err
	}

	// Store it.
	tf.EncryptedSecret = ciphertext
	tf.HashedSecret = hash
	return nil
}

// GetSecret decrypts and verifies the TOTP secret (URL).
func (tf *TwoFactor) GetSecret() (string, error) {
	// Decrypt it.
	plaintext, err := encryption.DecryptString(tf.EncryptedSecret)
	if err != nil {
		return "", err
	}

	// Verify it.
	if !encryption.VerifyHash([]byte(plaintext), tf.HashedSecret) {
		return "", errors.New("hash of secret did not match: the site AES key may be wrong")
	}

	return plaintext, nil
}

// Validate a given 2FA code or Backup Code.
func (tf *TwoFactor) Validate(code string) error {
	// Reconstruct the stored TOTP key.
	secret, err := tf.GetSecret()
	if err != nil {
		return err
	}

	// Reconstruct the OTP key object.
	key, err := otp.NewKeyFromURL(secret)
	if err != nil {
		return err
	}

	// Check for TOTP secret.
	if totp.Validate(code, key.Secret()) {
		return nil
	}

	// Check for (and burn) a Backup Code.
	if tf.ValidateBackupCode(code) {
		return nil
	}

	return errors.New("not a valid code")
}

// GenerateBackupCodes will generate and reset the backup codes (encrypted).
func (tf *TwoFactor) GenerateBackupCodes() error {
	var (
		codes    = []string{}
		distinct = map[string]interface{}{}
		alphabet = []byte("abcdefghijklmnopqrstuvwxyz0123456789")
	)

	for i := 0; i < config.TwoFactorBackupCodeCount; i++ {
		for {
			var code []byte
			for j := 0; j < config.TwoFactorBackupCodeLength; j++ {
				code = append(code, alphabet[rand.Intn(len(alphabet))])
			}

			// Check for distinctness.
			var codeStr = string(code)
			if _, ok := distinct[codeStr]; ok {
				continue
			}
			distinct[codeStr] = nil

			codes = append(codes, codeStr)
			break
		}
	}

	// Encrypt the codes.
	return tf.SetBackupCodes(codes)
}

// SetBackupCodes encrypts and stores the codes to DB.
func (tf *TwoFactor) SetBackupCodes(codes []string) error {
	ciphertext, err := encryption.EncryptString(strings.Join(codes, ","))
	if err != nil {
		return err
	}

	tf.BackupCodes = ciphertext
	return nil
}

// GetBackupCodes returns the list of still-valid backup codes.
func (tf *TwoFactor) GetBackupCodes() ([]string, error) {
	// Decrypt the backup codes.
	plaintext, err := encryption.DecryptString(tf.BackupCodes)
	if err != nil {
		return nil, err
	}

	return strings.Split(plaintext, ","), nil
}

// ValidateBackupCode will check if the code is a backup code and burn it if so.
func (tf *TwoFactor) ValidateBackupCode(code string) bool {
	var (
		codes, err = tf.GetBackupCodes()
		newCodes   = []string{} // in case of burning one
	)
	if err != nil {
		log.Error("ValidateBackupCode: %s", err)
		return false
	}

	// Check for a match to our backup codes.
	code = strings.ToLower(code)
	var matched bool
	for _, check := range codes {
		if check == code {
			// Successful match!
			matched = true
		} else {
			newCodes = append(newCodes, check)
		}
	}

	// If we found a match, burn the code.
	if matched {
		if err := tf.SetBackupCodes(newCodes); err != nil {
			log.Error("ValidateBackupCode: SetBackupCodes: %s", err)
			return false
		}

		// Save it to DB.
		if err := tf.Save(); err != nil {
			log.Error("ValidateBackupCode: saving changes to DB: %s", err)
			return false
		}
	}

	return matched
}

// QRCodeAsDataURL returns an HTML img tag that embeds the 2FA QR code as a PNG data URL.
func (tf *TwoFactor) QRCodeAsDataURL(key *otp.Key) (string, error) {
	var buf bytes.Buffer
	img, err := key.Image(200, 200)
	if err != nil {
		return "", err
	}
	png.Encode(&buf, img)

	var dataURL = fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(buf.Bytes()))
	return fmt.Sprintf(`<img src="%s" alt="QR Code">`, dataURL), nil
}

// Save the note.
func (tf *TwoFactor) Save() error {
	log.Error("SAVE 2FA: %+v", tf)
	if tf.isNew {
		return DB.Create(tf).Error
	}
	return DB.Save(tf).Error
}

// Delete the DB entry.
func (tf *TwoFactor) Delete() error {
	if tf.isNew {
		return nil
	}
	return DB.Delete(tf).Error
}

// TwoFactorMap helps map a set of users to whether they have 2FA set up.
type TwoFactorMap map[uint64]bool

// MapTwoFactor looks up a set of users and maps which of them have 2FA set up.
func MapTwoFactor(users []*User) (TwoFactorMap, error) {
	var (
		TwoFactorMap = TwoFactorMap{}
		set          = map[uint64]interface{}{}
		distinct     = []uint64{}
	)

	// Uniqueify the IDs.
	for _, user := range users {
		if _, ok := set[user.ID]; ok {
			continue
		}
		set[user.ID] = nil
		distinct = append(distinct, user.ID)
	}

	var (
		tf     = []*TwoFactor{}
		result = DB.Model(&TwoFactor{}).Where(
			"user_id IN ? AND enabled IS TRUE",
			distinct,
		).Find(&tf)
	)

	if result.Error == nil {
		for _, row := range tf {
			TwoFactorMap[row.UserID] = true
		}
	}

	return TwoFactorMap, result.Error
}

// Has a photo ID in the map?
func (m TwoFactorMap) Has(id uint64) bool {
	_, ok := m[id]
	return ok
}

// Get a photo from the TwoFactorMap.
func (m TwoFactorMap) Get(id uint64) bool {
	if v, ok := m[id]; ok {
		return v
	}
	return false
}
