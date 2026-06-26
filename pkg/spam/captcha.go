package spam

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
)

// ValidateTurnstileCAPTCHA tests a Cloudflare Turnstile CAPTCHA token.
func ValidateTurnstileCAPTCHA(token, actionName string) error {
	if !config.Current.Turnstile.Enabled {
		return errors.New("Cloudflare Turnstile CAPTCHA is not enabled in the server settings")
	}

	// Prepare the request.
	form := url.Values{}
	form.Add("secret", config.Current.Turnstile.SecretKey)
	form.Add("response", token)
	url := "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	req, err := http.NewRequest("POST", url, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Make the request.
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Read the response.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Reading response body from Cloudflare: %s", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Error("Turnstile CAPTCHA error (status %d): %s", resp.StatusCode, string(body))
		return fmt.Errorf("CAPTCHA validation error: status code %d", resp.StatusCode)
	}

	// Parse the response JSON.
	type response struct {
		Success     bool      `json:"success"`
		ErrorCodes  []string  `json:"error-codes"`
		ChallengeTS time.Time `json:"challenge_ts"`
		Hostname    string    `json:"hostname"`
	}
	var result response
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parsing result json from Cloudflare: %s", err)
	}

	if !result.Success {
		log.Error("Turnstile CAPTCHA error (status %d): %s", resp.StatusCode, string(body))
		return errors.New("verification failed")
	}

	return nil
}
