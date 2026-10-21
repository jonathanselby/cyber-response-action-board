package main

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/adapters/notifier"
	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/config"
)

type notificationChannelDeployment struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Target         string `json:"target"`
	DestinationID  string `json:"destination_id"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

type notificationDeploymentFile struct {
	Channels []notificationChannelDeployment `json:"channels"`
}

func loadNotificationDeployment(cfg config.Config) ([]app.NotificationChannel, error) {
	if strings.TrimSpace(cfg.NotificationConfigFile) == "" {
		return nil, nil
	}
	var file notificationDeploymentFile
	if err := readPrivateJSONFile(cfg.NotificationConfigFile, &file); err != nil {
		return nil, fmt.Errorf("notification deployment config: %w", err)
	}
	channels := make([]app.NotificationChannel, 0, len(file.Channels))
	for _, value := range file.Channels {
		channelType := strings.ToLower(strings.TrimSpace(value.Type))
		if value.TimeoutSeconds < 0 {
			return nil, errors.New("notification timeout_seconds must not be negative")
		}
		channel := app.NotificationChannel{Name: strings.TrimSpace(value.Name), Type: channelType, Target: strings.TrimSpace(value.Target), DestinationID: strings.TrimSpace(value.DestinationID), Timeout: time.Duration(value.TimeoutSeconds) * time.Second}
		switch channelType {
		case notifier.ChannelLocalFile:
			channel.External = false
		case notifier.ChannelWebhook:
			channel.External = true
		default:
			return nil, fmt.Errorf("unsupported notification channel type %q", channelType)
		}
		channels = append(channels, channel)
	}
	if err := notifier.ValidateChannels(channels); err != nil {
		return nil, err
	}
	app.SortNotificationChannels(channels)
	return channels, nil
}

func notificationDeploymentReadiness(cfg config.Config) app.Check {
	if strings.TrimSpace(cfg.NotificationConfigFile) == "" {
		return app.Check{Name: "notification_channels", Status: "not-configured", Message: "no notification deployment file configured; communications remain recordable without delivery"}
	}
	channels, err := loadNotificationDeployment(cfg)
	if err != nil {
		return app.Check{Name: "notification_channels", Status: "fail", Message: err.Error()}
	}
	return app.Check{Name: "notification_channels", Status: "pass", Message: fmt.Sprintf("%d notification channel(s) configured and validated", len(channels))}
}
