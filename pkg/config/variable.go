package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/cuvou/gosocial/pkg/encryption/keygen"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/google/uuid"
)

// Version of the config format - when new fields are added, it will attempt
// to write the settings.toml to disk so new defaults populate.
var currentVersion = 12

// Current loaded settings.json
var Current = DefaultVariable()

// Variable configuration attributes (loaded from settings.json).
type Variable struct {
	Version             int
	BaseURL             string
	AdminEmail          string
	CronAPIKey          string
	Mail                Mail
	Redis               Redis
	Database            Database
	BareRTC             BareRTC
	Maintenance         Maintenance
	Encryption          Encryption
	SignedPhoto         SignedPhoto
	WebPush             WebPush
	Cloudflare          Cloudflare
	Turnstile           Turnstile
	UseXForwardedFor    bool
	EmergencyKillSwitch EmergencyKillSwitch
}

// DefaultVariable returns the default settings.json data.
func DefaultVariable() Variable {
	return Variable{
		BaseURL: "http://localhost:8080",
		Mail: Mail{
			Enabled: false,
			Host:    "localhost",
			Port:    25,
			From:    "no-reply@localhost",
		},
		Redis: Redis{
			Host: "localhost",
			Port: 6379,
		},
		Database: Database{
			SQLite:       "database.sqlite",
			Postgres:     "host=localhost user=gosocial password=gosocial dbname=gosocial port=5679 sslmode=disable TimeZone=America/Los_Angeles",
			MaxIdleConns: 10,
			MaxOpenConns: 50,
		},
		CronAPIKey: uuid.New().String(),
		EmergencyKillSwitch: EmergencyKillSwitch{
			OwnerUserID:    1,
			DaysMissingTTL: 30,
			Headline:       "The website's owner may have gone missing",
			Message:        "The owner of this website has not logged in for a while.",
		},
	}
}

// LoadSettings loads the settings.json file or, if not existing, creates it with the default settings.
func LoadSettings() {
	var writeSettings bool

	if _, err := os.Stat(SettingsPath); !os.IsNotExist(err) {
		log.Info("Loading settings from %s", SettingsPath)
		content, err := os.ReadFile(SettingsPath)
		if err != nil {
			panic(fmt.Sprintf("LoadSettings: couldn't read settings.json: %s", err))
		}

		var v Variable
		err = json.Unmarshal(content, &v)
		if err != nil {
			panic(fmt.Sprintf("LoadSettings: couldn't parse settings.json: %s", err))
		}

		Current = v
	} else {
		WriteSettings()
		log.Warn("NOTICE: Created default settings.json file - review it and configure mail servers and database!")
	}

	// If there is no DB configured, exit now.
	if !Current.Database.IsSQLite && !Current.Database.IsPostgres {
		log.Error("No database configured in settings.json. Choose SQLite or Postgres and update the DB connector string!")
		os.Exit(1)
	}

	// Initialize the AES encryption key.
	if len(Current.Encryption.AESKey) == 0 {
		log.Warn("NOTICE: rolling a random 32-byte (256-bit) AES encryption key for the settings file")
		aesKey, err := keygen.NewAESKey()
		if err != nil {
			log.Error("Couldn't generate AES key: %s", err)
			os.Exit(1)
		}
		Current.Encryption.AESKey = aesKey
		writeSettings = true
	}

	// Initialize the VAPID keys for Web Push Notification.
	if len(Current.WebPush.VAPIDPublicKey) == 0 {
		privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
		if err != nil {
			log.Error("Initializing VAPID keys for Web Push: %s", err)
			os.Exit(1)
		}

		Current.WebPush.VAPIDPrivateKey = privateKey
		Current.WebPush.VAPIDPublicKey = publicKey
		writeSettings = true
	}

	// Initialize JWT token for SignedPhoto feature.
	if Current.SignedPhoto.JWTSecret == "" {
		Current.SignedPhoto.JWTSecret = uuid.New().String()
		writeSettings = true
	}

	// Have we added new config fields? Save the settings.json.
	if Current.Version != currentVersion || writeSettings {
		log.Warn("New options are available for your settings.json file. Your settings will be re-saved now.")
		Current.Version = currentVersion
		if err := WriteSettings(); err != nil {
			log.Error("Couldn't write your settings.json file: %s", err)
		}
	}
}

// WriteSettings will commit the settings.json to disk.
func WriteSettings() error {
	log.Error("Note: initial settings.json was written to disk.")

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "    ")
	err := enc.Encode(Current)
	if err != nil {
		panic(fmt.Sprintf("WriteSettings: couldn't marshal settings: %s", err))
	}

	return os.WriteFile(SettingsPath, buf.Bytes(), 0600)
}

// Mail settings.
type Mail struct {
	Enabled  bool
	Host     string // localhost
	Port     int    // 25
	From     string // noreply@localhost
	Username string // SMTP credentials
	Password string

	// MailerSend API key: if set, use MailerSend API instead of SMTP.
	MailerSendAPIKey string
}

// Redis settings.
type Redis struct {
	Host string
	Port int
	DB   int
}

// Database settings.
type Database struct {
	IsSQLite     bool
	IsPostgres   bool
	SQLite       string
	Postgres     string
	MaxIdleConns int
	MaxOpenConns int
}

// BareRTC chat room settings.
type BareRTC struct {
	JWTSecret string
	URL       string
}

// Maintenance mode settings.
type Maintenance struct {
	Headline          string
	Message           string
	MessageOnAllPages bool
	PauseSignup       bool
	PauseLogin        bool
	PauseChat         bool
	PauseInteraction  bool
}

// EmergencyKillSwitch settings.
type EmergencyKillSwitch struct {
	Enabled        bool
	Activated      bool
	OwnerUserID    uint64
	DaysMissingTTL int
	Headline       string
	Message        string
}

// Encryption settings.
type Encryption struct {
	AESKey []byte
}

// SignedPhoto settings.
type SignedPhoto struct {
	Enabled   bool
	JWTSecret string
}

// WebPush settings.
type WebPush struct {
	VAPIDPublicKey  string
	VAPIDPrivateKey string
}

// Turnstile (Cloudflare CAPTCHA) settings.
type Turnstile struct {
	Enabled   bool
	SiteKey   string
	SecretKey string
}

// Cloudflare CDN settings.
type Cloudflare struct {
	Enabled  bool
	APIToken string
	Email    string
	ZoneID   string
}
