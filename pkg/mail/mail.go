// Package mail provides e-mail sending faculties.
package mail

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/encryption"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/redis"
	"github.com/microcosm-cc/bluemonday"
	"gopkg.in/gomail.v2"
)

// Message configuration.
type Message struct {
	To       string
	ReplyTo  string
	Subject  string
	Template string // path relative to the templates dir, e.g. "email/verify_email.html"
	Data     map[string]interface{}
}

// LockSending emails to the same address within 24 hours, e.g.: on the signup form to reduce chance for spam abuse.
//
// Call this before calling Send() if you want to throttle the sending. This function will put a key in Redis on
// the first call and return nil; on subsequent calls, if the key still remains, it will return an error.
func LockSending(namespace, email string, expires time.Duration) error {
	var key = fmt.Sprintf("mail/lock-sending/%s/%s", namespace, encryption.Hash([]byte(email)))

	// See if we have already locked it.
	if redis.Exists(key) {
		return errors.New("email was in the lock-sending queue")
	}

	redis.Set(key, email, expires)
	return nil
}

// Send an email.
func Send(msg Message) error {
	conf := config.Current.Mail

	// Verify configuration.
	if !conf.Enabled {
		return errors.New(
			"Email sending is not configured for this app. Please contact the website administrator about this error.",
		)
	} else if conf.Host == "" || conf.Port == 0 || conf.From == "" {
		return errors.New(
			"Email settings are misconfigured for this app. Please contact the website administrator about this error.",
		)
	}

	// Get and render the template to HTML.
	var html bytes.Buffer
	tmpl, err := template.New(msg.Template).ParseFiles(config.TemplatePath + "/" + msg.Template)
	if err != nil {
		return err
	}

	// Execute the template.
	err = tmpl.ExecuteTemplate(&html, "content", msg)
	if err != nil {
		return fmt.Errorf("Mail template execute error: %s", err)
	}

	// Condense the HTML down into the plaintext version.
	rawLines := strings.Split(
		bluemonday.StrictPolicy().Sanitize(html.String()),
		"\n",
	)
	var lines []string
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		lines = append(lines, line)
	}
	plaintext := strings.Join(lines, "\n\n")

	// The email bits.
	var (
		from    = fmt.Sprintf("%s <%s>", config.Title, conf.From)
		to      = msg.To
		replyTo = msg.ReplyTo
		subject = msg.Subject
	)

	// Sending via the MailerSend API?
	if config.Current.Mail.MailerSendAPIKey != "" {
		log.Info("mail.Send: using the MailerSend API")
		return SendViaMailerSend(conf.From, to, replyTo, subject, html.String(), plaintext)
	}

	// Prepare the e-mail!
	m := gomail.NewMessage()
	m.SetHeader("From", from)
	m.SetHeader("To", to)
	if msg.ReplyTo != "" {
		m.SetHeader("Reply-To", replyTo)
	}
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", plaintext)
	m.AddAlternative("text/html", html.String())

	// Deliver asynchronously.
	log.Info("mail.Send: %s (%s) to %s", msg.Subject, msg.Template, msg.To)
	d := gomail.NewDialer(conf.Host, conf.Port, conf.Username, conf.Password)
	go func() {
		if err := d.DialAndSend(m); err != nil {
			log.Error("mail.Send: %s", err.Error())
		}
	}()

	return nil
}
