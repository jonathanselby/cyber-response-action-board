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

// noticeDeployment is the organisation's notice settings (R-05). Escalation
// notices (event 4) are off unless enabled here.
type noticeDeployment struct {
	EscalationEnabled        bool     `json:"escalation_enabled,omitempty"`
	EscalationToMembers      bool     `json:"escalation_to_members,omitempty"`
	EscalationSharedChannels []string `json:"escalation_shared_channels,omitempty"`
	DecisionWindowMinutes    int      `json:"decision_window_minutes,omitempty"`
	AccessExpiryLeadHours    int      `json:"access_expiry_lead_hours,omitempty"`
}

type notificationDeploymentFile struct {
	Channels []notifier.Deployment `json:"channels"`
	Notices  noticeDeployment      `json:"notices"`
}

type notificationDeployment struct {
	Channels []app.NotificationChannel
	Policy   app.NoticePolicy
}

func loadNotificationDeployment(cfg config.Config) (notificationDeployment, error) {
	if strings.TrimSpace(cfg.NotificationConfigFile) == "" {
		return notificationDeployment{Policy: app.DefaultNoticePolicy()}, nil
	}
	var file notificationDeploymentFile
	if err := readPrivateJSONFile(cfg.NotificationConfigFile, &file); err != nil {
		return notificationDeployment{}, fmt.Errorf("notification deployment config: %w", err)
	}
	channels, err := notifier.Channels(file.Channels)
	if err != nil {
		return notificationDeployment{}, err
	}
	n := file.Notices
	if n.DecisionWindowMinutes < 0 || n.DecisionWindowMinutes > 24*60 || n.AccessExpiryLeadHours < 0 || n.AccessExpiryLeadHours > 7*24 {
		return notificationDeployment{}, errors.New("notices: decision_window_minutes must be 0–1440 and access_expiry_lead_hours 0–168")
	}
	policy := app.DefaultNoticePolicy()
	policy.EscalationEnabled, policy.EscalationToMembers, policy.EscalationSharedChannels = n.EscalationEnabled, n.EscalationToMembers, n.EscalationSharedChannels
	if n.DecisionWindowMinutes > 0 {
		policy.DecisionWindow = time.Duration(n.DecisionWindowMinutes) * time.Minute
	}
	if n.AccessExpiryLeadHours > 0 {
		policy.AccessExpiryLead = time.Duration(n.AccessExpiryLeadHours) * time.Hour
	}
	if err := app.ValidateNoticePolicy(policy, channels); err != nil {
		return notificationDeployment{}, err
	}
	return notificationDeployment{Channels: channels, Policy: policy}, nil
}

func notificationDeploymentReadiness(cfg config.Config) app.Check {
	if strings.TrimSpace(cfg.NotificationConfigFile) == "" {
		return app.Check{Name: "notification_channels", Status: "not-configured", Message: "no notification deployment file configured; communications remain recordable without delivery"}
	}
	d, err := loadNotificationDeployment(cfg)
	if err != nil {
		return app.Check{Name: "notification_channels", Status: "fail", Message: err.Error()}
	}
	return app.Check{Name: "notification_channels", Status: "pass", Message: fmt.Sprintf("%d notification channel(s) configured and validated", len(d.Channels))}
}
