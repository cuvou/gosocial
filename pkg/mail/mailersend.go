package mail

import (
	"context"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/mailersend/mailersend-go"
)

// SendViaMailerSend delivers an email by using the MailerSend API instead of SMTP.
//
// Note: this is done if the MailerSendAPIKey is defined in settings.json. If not
// defined, the fallback SMTP config is used instead.
func SendViaMailerSend(from, to, replyTo, subject, html, plaintext string) error {

	ms := mailersend.NewMailersend(config.Current.Mail.MailerSendAPIKey)

	mailFrom := mailersend.From{
		Name:  config.Title,
		Email: from,
	}

	rcptTo := []mailersend.Recipient{
		{
			Email: to,
		},
	}

	message := ms.Email.NewMessage()
	message.SetFrom(mailFrom)
	message.SetRecipients(rcptTo)
	message.SetSubject(subject)
	message.SetHTML(html)
	message.SetText(plaintext)

	if replyTo != "" {
		message.SetReplyTo(mailersend.Recipient{
			Email: replyTo,
		})
	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	_, err := ms.Email.Send(ctx, message)
	// log.Debug("MailerSend response: %+v", res)

	return err
}
