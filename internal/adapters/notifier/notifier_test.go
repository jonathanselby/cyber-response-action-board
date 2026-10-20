package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

func TestLocalSinkAndWebhookUseApprovedMessageWithoutResponseBodyLeak(t *testing.T) {
	t.Run("local", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "notifications.jsonl")
		channel := app.NotificationChannel{Name: "local", Type: ChannelLocalFile, Target: path, DestinationID: "local-ops"}
		attempt := (Sender{}).Deliver(context.Background(), channel, app.NotificationMessage{CommunicationID: "COMM-1", Mode: "live", Subject: "Update", Body: "Approved body", Audience: []string{"incident-team"}})
		if attempt.Status != domain.NotificationDelivered {
			t.Fatalf("local attempt=%+v", attempt)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), "Approved body") {
			t.Fatalf("local sink=%s", raw)
		}
	})

	t.Run("webhook", func(t *testing.T) {
		var got app.NotificationMessage
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Error(err)
			}
			w.Header().Set("X-Request-ID", "ack-123")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte("sensitive downstream response body"))
		}))
		defer server.Close()
		channel := app.NotificationChannel{Name: "hook", Type: ChannelWebhook, Target: server.URL, DestinationID: "regulator-endpoint", External: true}
		attempt := (Sender{Client: server.Client()}).Deliver(context.Background(), channel, app.NotificationMessage{CommunicationID: "COMM-2", Mode: "live", Subject: "Notice", Body: "Approved notice", Audience: []string{"regulator"}})
		if attempt.Status != domain.NotificationDelivered || !strings.Contains(attempt.Acknowledgement, "HTTP 202") || !strings.Contains(attempt.Acknowledgement, "ack-123") {
			t.Fatalf("webhook attempt=%+v", attempt)
		}
		if strings.Contains(attempt.Acknowledgement, "sensitive") || strings.Contains(attempt.Detail, "sensitive") {
			t.Fatal("webhook response body leaked into outcome metadata")
		}
		if got.Body != "Approved notice" || got.CommunicationID != "COMM-2" {
			t.Fatalf("webhook body=%+v", got)
		}
	})
}

func TestWebhookRejectsPlainExternalHTTP(t *testing.T) {
	channel := app.NotificationChannel{Name: "hook", Type: ChannelWebhook, Target: "http://example.com/hook", DestinationID: "external", External: true}
	if err := ValidateChannel(channel); err == nil {
		t.Fatal("plain external HTTP webhook accepted")
	}
}

func TestWebhookDoesNotFollowRedirects(t *testing.T) {
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirected.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer target.Close()

	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()

	channel := app.NotificationChannel{Name: "hook", Type: ChannelWebhook, Target: redirect.URL, DestinationID: "configured-endpoint", External: true}
	attempt := (Sender{Client: redirect.Client()}).Deliver(context.Background(), channel, app.NotificationMessage{CommunicationID: "COMM-REDIRECT", Mode: "live", Subject: "Notice", Body: "Approved notice", Audience: []string{"regulator"}})
	if attempt.Status != domain.NotificationFailed || redirected.Load() != 0 || !strings.Contains(attempt.Detail, "HTTP 307") {
		t.Fatalf("redirect was not refused attempt=%+v redirected=%d", attempt, redirected.Load())
	}
}
