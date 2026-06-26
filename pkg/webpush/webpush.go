// Package webpush provides Web Push Notification functionality.
package webpush

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/controller/api"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	webpush "github.com/SherClockHolmes/webpush-go"
)

// VAPIDPublicKey returns the site's public key as an endpoint.
func VAPIDPublicKey(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(config.Current.WebPush.VAPIDPublicKey))
}

// UnregisterAll resets a user's stored push notification subscriptions.
func UnregisterAll() http.HandlerFunc {
	var next = "/settings/notifications"

	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "You must be logged in to do that!")
			templates.Redirect(w, next)
			return
		}

		if err := models.DeletePushNotificationSubscriptions(currentUser); err != nil {
			session.FlashError(w, r, "Error removing your subscriptions: %s", err)
		} else {
			session.Flash(w, r, "Your push notification subscriptions have been reset!")
		}
		templates.Redirect(w, next)
	}
}

// Register endpoint for push notification.
func Register() http.HandlerFunc {
	type Request struct {
		Endpoint       string  `json:"endpoint"`
		ExpirationTime float64 `json:"expirationTime"`
		Keys           struct {
			Auth   string `json:"auth"`
			P256DH string `json:"p256dh"`
		} `json:"keys"`
	}

	type Response struct {
		OK    bool   `json:"OK"`
		Error string `json:"error,omitempty"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			api.SendJSON(w, http.StatusUnauthorized, Response{
				Error: "You must be logged in to do that!",
			})
			return
		}

		// Parse request payload.
		var req Request
		if err := api.ParseJSON(r, &req); err != nil {
			api.SendJSON(w, http.StatusBadRequest, Response{
				Error: fmt.Sprintf("Error with request payload: %s", err),
			})
			return
		}

		// Validate it looked correct.
		if req.Endpoint == "" || req.Keys.Auth == "" || req.Keys.P256DH == "" {
			api.SendJSON(w, http.StatusBadRequest, Response{
				Error: "Subscription fields were missing.",
			})
			return
		}

		// Serialize and store it in the database.
		buf, err := json.Marshal(req)
		if err != nil {
			api.SendJSON(w, http.StatusInternalServerError, Response{
				Error: "Couldn't reserialize your subscription!",
			})
			return
		}

		_, err = models.RegisterPushNotification(currentUser, string(buf))
		if err != nil {
			api.SendJSON(w, http.StatusInternalServerError, Response{
				Error: "Couldn't create the registration in the database!",
			})
			return
		}

		api.SendJSON(w, http.StatusCreated, Response{
			OK: true,
		})
	}
}

// Payload sent in push notifications.
type Payload struct {
	Topic string `json:"-"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

// SendNotification sends a push notification to a user, broadcast to all of their subscriptions.
func SendNotification(user *models.User, body Payload) error {
	// Send to all of their subscriptions.
	subs, err := models.GetPushNotificationSubscriptions(user)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	for _, sub := range subs {
		if err := SendRawNotification(user, payload, body.Topic, sub.Subscription); err != nil {
			log.Error("SendNotification: %s", err)
		}
	}

	return nil
}

// SendNotificationToSubscription sends to a specific push subscriber.
func SendNotificationToSubscription(user *models.User, subscription string, body Payload) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	return SendRawNotification(user, payload, body.Topic, subscription)
}

// SendRawNotification sends out a push message.
func SendRawNotification(user *models.User, message []byte, topic, subscription string) error {
	// Decode the subscription.
	var (
		s       = &webpush.Subscription{}
		err     = json.Unmarshal([]byte(subscription), s)
		options = &webpush.Options{
			Topic:           topic,
			Subscriber:      user.Email,
			VAPIDPublicKey:  config.Current.WebPush.VAPIDPublicKey,
			VAPIDPrivateKey: config.Current.WebPush.VAPIDPrivateKey,
			TTL:             30,
		}
	)
	if err != nil {
		return err
	}

	resp, err := webpush.SendNotification(message, s, options)
	if err != nil {
		return fmt.Errorf("webpush.SendNotification: %s", err)
	}
	defer resp.Body.Close()

	// Handle error response codes.
	if resp.StatusCode >= 400 {
		log.Error("Got StatusCode %d when sending push notification; removing the subscription from DB", resp.StatusCode)
		models.DeletePushNotification(user, subscription)
	}

	return err
}
