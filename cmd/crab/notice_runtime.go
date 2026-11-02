package main

import (
	"context"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/presentation/web"
)

// noticeInterval is how often the notice pass reads the ledger and the timed
// wait list. The pass is idempotent, so the interval bounds latency only.
const noticeInterval = 30 * time.Second

// NoticePass runs one notice pass at the governed time.
func (r *operatorRuntime) NoticePass(ctx context.Context) (app.NoticeReport, error) {
	if r.noticeStore == nil {
		return app.NoticeReport{}, nil
	}
	return r.notices.Pass(ctx, r.kernel.Clock().Read().Now)
}

// RunNotices runs the notice pass until ctx ends. It is started by crab serve
// only, so tests and one-shot commands never send notices on their own. With
// no notification deployment there is nothing to send and it does not run.
func (r *operatorRuntime) RunNotices(ctx context.Context) {
	if len(r.notices.Channels) == 0 {
		return
	}
	ticker := time.NewTicker(noticeInterval)
	defer ticker.Stop()
	for {
		if !r.kernel.IntegrityFailed() {
			_, _ = r.NoticePass(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *operatorRuntime) Account(ctx context.Context, token string) (web.AccountPage, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return web.AccountPage{}, err
	}
	if !ok {
		return web.AccountPage{}, app.ErrAuthorisationDenied
	}
	v, err := r.routes.View(ctx, principal, principal.HumanID)
	if err != nil {
		return web.AccountPage{}, err
	}
	return web.AccountPage{HumanID: principal.HumanID, Route: web.RouteForm{Channel: v.Channel, Hint: v.Hint, Channels: v.Channels}}, nil
}

func (r *operatorRuntime) SetNotificationRoute(ctx context.Context, token, humanID, channel, address string) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	if humanID == "" {
		humanID = principal.HumanID
	}
	return r.routes.Set(ctx, principal, humanID, channel, address, r.kernel.Clock().Read().Now)
}

// personRoute is the route form for a person in access administration, or
// nil when the viewer may not manage it.
func (r *operatorRuntime) personRoute(ctx context.Context, viewer app.RouteViewer, humanID string) *web.RouteForm {
	v, err := r.routes.View(ctx, viewer, humanID)
	if err != nil {
		return nil
	}
	return &web.RouteForm{HumanID: humanID, Channel: v.Channel, Hint: v.Hint, Channels: v.Channels}
}
