// Package notifier adapts CRAB notification channels to the Garage notifier
// component (github.com/jonathanselby/notifier). Channel types, transports and
// body shapes belong to the component; this adapter maps CRAB deployment
// configuration onto it and converts outcomes. It holds no policy.
package notifier

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jonathanselby/notifier/notify"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

// CRAB deployment channel types. local_file and webhook are the names CRAB
// deployments have always used; file and http are accepted as the
// component's own names.
const (
	ChannelLocalFile = "local_file"
	ChannelWebhook   = "webhook"
	ChannelEmail     = notify.TypeEmail
	ChannelSMS       = notify.TypeSMSHTTP
	maxTimeout       = 60 * time.Second
)

// Deployment is one channel entry of the notification deployment file. Every
// field after destination_id is passed to the notifier unchanged.
type Deployment struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Target         string `json:"target"`
	DestinationID  string `json:"destination_id"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
	AddressPattern string `json:"address_pattern,omitempty"`
	BodyTemplate   string `json:"body_template,omitempty"`
	BodyShape      string `json:"body_shape,omitempty"`
	From           string `json:"from,omitempty"`
	Subject        string `json:"subject,omitempty"`
	AuthEnv        string `json:"auth_env,omitempty"`
}

// Channels maps deployment entries to CRAB channels and validates them with
// the notifier's own rules plus CRAB's (destination id, timeout bounds, and
// HTTPS for webhooks except loopback test endpoints).
func Channels(entries []Deployment) ([]app.NotificationChannel, error) {
	channels := make([]app.NotificationChannel, 0, len(entries))
	for _, e := range entries {
		if e.TimeoutSeconds < 0 {
			return nil, errors.New("notification timeout_seconds must not be negative")
		}
		kind, err := componentType(e.Type)
		if err != nil {
			return nil, err
		}
		spec := notify.Channel{Name: strings.TrimSpace(e.Name), Type: kind, Target: strings.TrimSpace(e.Target),
			AddressPattern: e.AddressPattern, BodyTemplate: e.BodyTemplate, BodyShape: e.BodyShape, From: e.From, Subject: e.Subject, AuthEnv: e.AuthEnv}
		channels = append(channels, app.NotificationChannel{
			Name: spec.Name, Type: strings.ToLower(strings.TrimSpace(e.Type)), Target: spec.Target, DestinationID: strings.TrimSpace(e.DestinationID),
			External: notify.IsExternal(kind), PerRecipient: kind == notify.TypeEmail || kind == notify.TypeSMSHTTP,
			Timeout: time.Duration(e.TimeoutSeconds) * time.Second, Spec: spec,
		})
	}
	if err := ValidateChannels(channels); err != nil {
		return nil, err
	}
	app.SortNotificationChannels(channels)
	return channels, nil
}

func componentType(t string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case ChannelLocalFile, notify.TypeFile:
		return notify.TypeFile, nil
	case ChannelWebhook, notify.TypeHTTP:
		return notify.TypeHTTP, nil
	case notify.TypeEmail:
		return notify.TypeEmail, nil
	case notify.TypeSMSHTTP:
		return notify.TypeSMSHTTP, nil
	default:
		return "", fmt.Errorf("unsupported notification channel type %q", t)
	}
}

// ValidateChannel applies CRAB's rules and the notifier's to one channel.
func ValidateChannel(channel app.NotificationChannel) error {
	if strings.TrimSpace(channel.Name) == "" || strings.TrimSpace(channel.Target) == "" || strings.TrimSpace(channel.DestinationID) == "" {
		return errors.New("notification channel requires name, target and destination id")
	}
	if channel.Timeout < 0 || channel.Timeout > maxTimeout {
		return errors.New("notification timeout outside supported bounds")
	}
	spec, err := specOf(channel)
	if err != nil {
		return err
	}
	if spec.Type == notify.TypeHTTP && !httpsOrLoopback(spec.Target) {
		return errors.New("webhook target must use HTTPS except loopback test endpoints")
	}
	if channel.External != notify.IsExternal(spec.Type) {
		return errors.New("notification channel external flag contradicts its type")
	}
	return notify.Validate(notify.Config{Channels: []notify.Channel{spec}})
}

// ValidateChannels validates every channel and requires unique names.
func ValidateChannels(channels []app.NotificationChannel) error {
	if len(channels) == 0 {
		return errors.New("at least one notification channel is required")
	}
	seen := map[string]struct{}{}
	for _, channel := range channels {
		if err := ValidateChannel(channel); err != nil {
			return fmt.Errorf("notification channel %q: %w", channel.Name, err)
		}
		if _, ok := seen[channel.Name]; ok {
			return errors.New("notification channel names must be unique")
		}
		seen[channel.Name] = struct{}{}
	}
	return nil
}

// specOf returns the component channel. Channels built in code without a
// spec (tests, older callers) are mapped from their CRAB type and target.
func specOf(channel app.NotificationChannel) (notify.Channel, error) {
	if spec, ok := channel.Spec.(notify.Channel); ok {
		return spec, nil
	}
	kind, err := componentType(channel.Type)
	if err != nil {
		return notify.Channel{}, err
	}
	return notify.Channel{Name: channel.Name, Type: kind, Target: channel.Target}, nil
}

// Sender delivers through the notifier component.
type Sender struct {
	Client *http.Client
	// TLSConfig is the base TLS configuration for SMTP STARTTLS (tests).
	TLSConfig *tls.Config
	// Getenv reads credentials named by auth_env; os.Getenv when nil.
	Getenv func(string) string
}

func (s Sender) service() notify.Service {
	return notify.Service{Client: s.Client, TLSConfig: s.TLSConfig, Getenv: s.Getenv}
}

// Deliver attempts one message on one channel. A per-recipient channel needs
// message.To; the outcome never carries it.
func (s Sender) Deliver(ctx context.Context, channel app.NotificationChannel, message app.NotificationMessage) app.NotificationAttempt {
	start := time.Now()
	if err := ValidateChannel(channel); err != nil {
		return app.NotificationAttempt{Status: domain.NotificationFailed, Detail: err.Error(), Duration: time.Since(start)}
	}
	spec, _ := specOf(channel)
	if spec.Type == notify.TypeFile {
		if dir := filepath.Dir(spec.Target); dir != "." {
			if err := os.MkdirAll(dir, 0700); err != nil {
				return app.NotificationAttempt{Status: domain.NotificationFailed, Detail: "prepare local notification sink: " + err.Error(), Duration: time.Since(start)}
			}
		}
	}
	req := notify.Request{Message: text(message), Exercise: message.Mode == "exercise", TimeoutMS: int(channel.Timeout / time.Millisecond)}
	if channel.PerRecipient {
		ref := message.Ref
		if ref == "" {
			ref = "recipient"
		}
		req.Deliveries = []notify.Delivery{{Channel: spec.Name, To: message.To, Ref: ref}}
	} else {
		req.Channels = []string{spec.Name}
	}
	if err := notify.ValidateRequest(req); err != nil {
		return app.NotificationAttempt{Status: domain.NotificationFailed, Detail: err.Error(), Duration: time.Since(start)}
	}
	result := s.service().Send(ctx, notify.Config{Channels: []notify.Channel{spec}}, req)
	if len(result.Outcomes) != 1 {
		return app.NotificationAttempt{Status: domain.NotificationFailed, Detail: "notifier returned no outcome", Duration: time.Since(start)}
	}
	o := result.Outcomes[0]
	attempt := app.NotificationAttempt{Status: o.Status, Detail: o.Detail, Duration: time.Duration(o.DurationMS) * time.Millisecond}
	switch o.Status {
	case notify.StatusDelivered, notify.StatusFailed, notify.StatusSuppressed:
	default:
		attempt.Status, attempt.Detail = domain.NotificationFailed, "notifier returned invalid status"
	}
	return attempt
}

// AcceptsAddress reports whether a per-recipient channel would accept an
// address, using the notifier's own check (an exercise request: the address
// rule runs before suppression and nothing is sent).
func (s Sender) AcceptsAddress(channel app.NotificationChannel, address string) bool {
	if !channel.PerRecipient || ValidateChannel(channel) != nil {
		return false
	}
	spec, _ := specOf(channel)
	result := notify.Service{}.Send(context.Background(), notify.Config{Channels: []notify.Channel{spec}},
		notify.Request{Message: "check", Exercise: true, Deliveries: []notify.Delivery{{Channel: spec.Name, To: address, Ref: "check"}}})
	return len(result.Outcomes) == 1 && result.Outcomes[0].Status == notify.StatusSuppressed
}

// text is the plain message handed to the notifier: subject and body for a
// communication, the notice itself for a notice.
func text(m app.NotificationMessage) string {
	prefix := ""
	if m.Mode == "exercise" {
		prefix = "EXERCISE — NOT A LIVE INCIDENT\n"
	}
	if strings.TrimSpace(m.Subject) == "" {
		return prefix + m.Body
	}
	return prefix + m.Subject + "\n\n" + m.Body
}

func httpsOrLoopback(target string) bool {
	u, err := url.Parse(target)
	if err != nil || u.Host == "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	switch strings.ToLower(u.Hostname()) {
	case "localhost", "127.0.0.1", "::1":
		return u.Scheme == "http"
	}
	return false
}
