package app

import (
	"errors"
	"testing"
	"time"
)

func TestNoticePassFailureIsNeverSilent(t *testing.T) {
	if c := NoticePassCheck(true, errors.New("database unavailable"), time.Now()); c.Status != "fail" {
		t.Fatalf("failed pass reported as %s", c.Status)
	}
	if c := NoticePassCheck(true, nil, time.Time{}); c.Status != "warn" {
		t.Fatalf("pass that never ran reported as %s", c.Status)
	}
	if c := NoticePassCheck(false, nil, time.Time{}); c.Status != "pass" {
		t.Fatalf("unconfigured notices reported as %s", c.Status)
	}
}
