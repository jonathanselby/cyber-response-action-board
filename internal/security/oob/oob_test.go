package oob

import (
	"testing"
	"time"
)

func TestCallbackUsesOnlyPreRegisteredRoute(t *testing.T) {
	now := time.Date(2026, 10, 20, 2, 0, 0, 0, time.UTC)
	key := []byte("0123456789abcdef0123456789abcdef")
	session, err := StartCallback(Person{ID: "person-1", Callback: "+61-registered"}, "+61-caller-supplied", now, 10*time.Minute, key)
	if err != nil {
		t.Fatal(err)
	}
	if session.Callback != "+61-registered" || !session.InboundContactIgnored {
		t.Fatalf("callback=%q ignored=%v", session.Callback, session.InboundContactIgnored)
	}
	result, err := CompleteCallback(session, session.Token, "person-1", true, now.Add(time.Minute), key)
	if err != nil || !result.Verified {
		t.Fatalf("verification=%+v err=%v", result, err)
	}
	tampered := session
	tampered.Callback = "+61-caller-supplied"
	result, err = CompleteCallback(tampered, session.Token, "person-1", true, now.Add(time.Minute), key)
	if err != nil || result.Verified {
		t.Fatalf("tampered callback verification=%+v err=%v", result, err)
	}
}
