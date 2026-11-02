package notifier

import (
	"context"
	"encoding/json"
	"io"
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
		path := filepath.Join(t.TempDir(), "sub", "notifications.log")
		channel := app.NotificationChannel{Name: "local", Type: ChannelLocalFile, Target: path, DestinationID: "local-ops"}
		attempt := (Sender{}).Deliver(context.Background(), channel, app.NotificationMessage{CommunicationID: "COMM-1", Mode: "live", Subject: "Update", Body: "Approved body", Audience: []string{"incident-team"}})
		if attempt.Status != domain.NotificationDelivered {
			t.Fatalf("local attempt=%+v", attempt)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), "Update") || !strings.Contains(string(raw), "Approved body") {
			t.Fatalf("local sink=%s", raw)
		}
	})

	t.Run("webhook", func(t *testing.T) {
		var got map[string]string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Error(err)
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte("sensitive downstream response body"))
		}))
		defer server.Close()
		channel := app.NotificationChannel{Name: "hook", Type: ChannelWebhook, Target: server.URL, DestinationID: "regulator-endpoint", External: true}
		attempt := (Sender{Client: server.Client()}).Deliver(context.Background(), channel, app.NotificationMessage{CommunicationID: "COMM-2", Mode: "live", Subject: "Notice", Body: "Approved notice", Audience: []string{"regulator"}})
		if attempt.Status != domain.NotificationDelivered {
			t.Fatalf("webhook attempt=%+v", attempt)
		}
		if strings.Contains(attempt.Acknowledgement, "sensitive") || strings.Contains(attempt.Detail, "sensitive") {
			t.Fatal("webhook response body leaked into outcome metadata")
		}
		if got["message"] != "Notice\n\nApproved notice" {
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
	if attempt.Status != domain.NotificationFailed || redirected.Load() != 0 || !strings.Contains(attempt.Detail, "307") {
		t.Fatalf("redirect was not refused attempt=%+v redirected=%d", attempt, redirected.Load())
	}
}

func TestDeploymentMapsToNotifierChannels(t *testing.T) {
	channels, err := Channels([]Deployment{
		{Name: "local", Type: "local_file", Target: "/tmp/crab.log", DestinationID: "ops-log"},
		{Name: "teams", Type: "webhook", Target: "https://teams.example/hook", DestinationID: "ir-channel", BodyShape: "text"},
		{Name: "mail", Type: "email", Target: "smtp.example:587", DestinationID: "relay", AddressPattern: `^[^@\s]+@forensic\.example$`, From: "crab@forensic.example", AuthEnv: "CRAB_SMTP_AUTH"},
		{Name: "sms", Type: "sms-http", Target: "https://sms.example/send", DestinationID: "gateway", AddressPattern: `^\+61[0-9]{9}$`, BodyTemplate: `{"to":"{to}","text":"{message}"}`},
	})
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]app.NotificationChannel{}
	for _, c := range channels {
		byName[c.Name] = c
	}
	if byName["local"].External || byName["local"].PerRecipient || !byName["teams"].External || byName["teams"].PerRecipient {
		t.Fatalf("fixed channels mapped wrongly: %+v", channels)
	}
	if !byName["mail"].PerRecipient || !byName["mail"].External || !byName["sms"].PerRecipient {
		t.Fatalf("per-person channels mapped wrongly: %+v", channels)
	}
	bad := [][]Deployment{
		{{Name: "mail", Type: "email", Target: "smtp.example:587", DestinationID: "relay", From: "crab@forensic.example"}},                                             // no pattern
		{{Name: "mail", Type: "email", Target: "smtp.example:587", DestinationID: "relay", AddressPattern: "forensic", From: "crab@forensic.example"}},                 // unanchored
		{{Name: "sms", Type: "sms-http", Target: "http://sms.example/send", DestinationID: "g", AddressPattern: "^x$", BodyTemplate: `{"to":"{to}","t":"{message}"}`}}, // plain HTTP
		{{Name: "x", Type: "pager", Target: "x", DestinationID: "x"}},
	}
	for i, b := range bad {
		if _, err := Channels(b); err == nil {
			t.Fatalf("bad deployment %d accepted", i)
		}
	}
}

func TestPerPersonDeliveryNeverRecordsTheAddress(t *testing.T) {
	var body string
	gateway := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer gateway.Close()
	channels, err := Channels([]Deployment{{Name: "sms", Type: "sms-http", Target: gateway.URL, DestinationID: "gateway", AddressPattern: `^\+61[0-9]{9}$`, BodyTemplate: `{"to":"{to}","text":"{message}"}`}})
	if err != nil {
		t.Fatal(err)
	}
	sender := Sender{Client: gateway.Client()}
	attempt := sender.Deliver(context.Background(), channels[0], app.NotificationMessage{Mode: "live", Body: "CRAB: 2 items on INC-7 are waiting for your decision. Sign in to CRAB.", To: "+61400000789", Ref: "tom"})
	if attempt.Status != domain.NotificationFailed || attempt.Detail != "HTTP status 502" || strings.Contains(attempt.Detail, "+61400000789") {
		t.Fatalf("attempt=%+v", attempt)
	}
	if !strings.Contains(body, "+61400000789") {
		t.Fatalf("gateway did not get the address: %s", body)
	}
	if !sender.AcceptsAddress(channels[0], "+61400000789") || sender.AcceptsAddress(channels[0], "+1555000111") || sender.AcceptsAddress(channels[0], "") {
		t.Fatal("address check does not follow the channel pattern")
	}
	// Exercise: suppressed before I/O.
	body = ""
	attempt = sender.Deliver(context.Background(), channels[0], app.NotificationMessage{Mode: "exercise", Body: "x", To: "+61400000789", Ref: "tom"})
	if attempt.Status != domain.NotificationSuppressed || body != "" {
		t.Fatalf("exercise attempt=%+v body=%q", attempt, body)
	}
}
