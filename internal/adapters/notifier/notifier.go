package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

const (
	ChannelLocalFile = "local_file"
	ChannelWebhook   = "webhook"
	defaultTimeout   = 3 * time.Second
	maxTimeout       = 60 * time.Second
)

type Sender struct {
	Client *http.Client
}

func ValidateChannel(channel app.NotificationChannel) error {
	if strings.TrimSpace(channel.Name) == "" || strings.TrimSpace(channel.Target) == "" || strings.TrimSpace(channel.DestinationID) == "" {
		return errors.New("notification channel requires name, target and destination id")
	}
	if channel.Timeout < 0 || channel.Timeout > maxTimeout {
		return errors.New("notification timeout outside supported bounds")
	}
	switch channel.Type {
	case ChannelLocalFile:
		if channel.External {
			return errors.New("local notification sink cannot be external")
		}
	case ChannelWebhook:
		if !channel.External {
			return errors.New("webhook channel must be external")
		}
		u, err := url.Parse(channel.Target)
		if err != nil || u.Host == "" || (u.Scheme != "https" && !(u.Scheme == "http" && loopbackHost(u.Hostname()))) {
			return errors.New("webhook target must use HTTPS except loopback test endpoints")
		}
	default:
		return errors.New("unsupported notification channel type")
	}
	return nil
}

func ValidateChannels(channels []app.NotificationChannel) error {
	if len(channels) == 0 {
		return errors.New("at least one notification channel is required")
	}
	seen := map[string]struct{}{}
	for _, channel := range channels {
		if err := ValidateChannel(channel); err != nil {
			return err
		}
		if _, ok := seen[channel.Name]; ok {
			return errors.New("notification channel names must be unique")
		}
		seen[channel.Name] = struct{}{}
	}
	return nil
}

func (s Sender) Deliver(ctx context.Context, channel app.NotificationChannel, message app.NotificationMessage) app.NotificationAttempt {
	start := time.Now()
	if err := ValidateChannel(channel); err != nil {
		return app.NotificationAttempt{Status: domain.NotificationFailed, Detail: err.Error(), Duration: time.Since(start)}
	}
	timeout := channel.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var acknowledgement string
	var err error
	switch channel.Type {
	case ChannelLocalFile:
		acknowledgement, err = deliverLocal(cctx, channel.Target, message)
	case ChannelWebhook:
		acknowledgement, err = s.deliverWebhook(cctx, channel.Target, message)
	default:
		err = errors.New("unsupported notification channel type")
	}
	attempt := app.NotificationAttempt{Duration: time.Since(start), Acknowledgement: acknowledgement}
	if err != nil {
		attempt.Status = domain.NotificationFailed
		attempt.Detail = err.Error()
		attempt.Acknowledgement = ""
		return attempt
	}
	attempt.Status = domain.NotificationDelivered
	return attempt
}

func deliverLocal(ctx context.Context, target string, message app.NotificationMessage) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil && filepath.Dir(target) != "." {
		return "", fmt.Errorf("prepare local notification sink: %w", err)
	}
	f, err := os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return "", fmt.Errorf("open local notification sink: %w", err)
	}
	defer f.Close()
	payload, err := json.Marshal(message)
	if err != nil {
		return "", fmt.Errorf("encode local notification: %w", err)
	}
	if _, err := f.Write(append(payload, '\n')); err != nil {
		return "", fmt.Errorf("write local notification sink: %w", err)
	}
	return "local sink appended", nil
}

func (s Sender) deliverWebhook(ctx context.Context, target string, message app.NotificationMessage) (string, error) {
	payload, err := json.Marshal(message)
	if err != nil {
		return "", fmt.Errorf("encode webhook notification: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call webhook: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}
	ack := fmt.Sprintf("HTTP %d", resp.StatusCode)
	if requestID := safeMetadata(resp.Header.Get("X-Request-ID")); requestID != "" {
		ack += " request_id=" + requestID
	}
	return ack, nil
}

func safeMetadata(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 128 {
		value = value[:128]
	}
	return strings.Map(func(r rune) rune {
		if r >= 32 && r <= 126 {
			return r
		}
		return -1
	}, value)
}

func loopbackHost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}
