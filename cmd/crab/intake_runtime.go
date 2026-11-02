package main

import (
	"context"
	"errors"
	"strings"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/presentation/web"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

// Machine intake adapters (addendum A4). They translate types and outcomes;
// every rule is in app.IntakeService and app.AccessService.

func (r *operatorRuntime) SubmitIntakeReport(ctx context.Context, token, source string, in web.IntakeReportInput) (web.IntakeResult, error) {
	receipt, err := r.intake.Submit(ctx, token, source, newOperatorID("REPORT"), app.IntakeReport{
		Description: in.Description, Reporter: in.Reporter, ContactRoute: in.ContactRoute, Affected: in.Affected,
		ImmediateImpact: in.ImmediateImpact, ReceivedAt: in.ReceivedAt, TimeConfidence: in.TimeConfidence, ExternalRef: in.ExternalRef,
	}, r.kernel.Clock().Read().Now)
	switch {
	case err == nil && receipt.Created:
		return web.IntakeResult{Outcome: web.IntakeCreated, ReportID: receipt.ReportID}, nil
	case err == nil:
		return web.IntakeResult{Outcome: web.IntakeExisting, ReportID: receipt.ReportID}, nil
	case errors.Is(err, app.ErrIntakeRefused):
		return web.IntakeResult{Outcome: web.IntakeRefused}, nil
	case errors.Is(err, app.ErrIntakeRateLimited):
		return web.IntakeResult{Outcome: web.IntakeRateLimited}, nil
	case errors.Is(err, app.ErrIntakeInvalid):
		return web.IntakeResult{Outcome: web.IntakeInvalid, Message: err.Error()}, nil
	}
	return web.IntakeResult{}, err
}

func (r *operatorRuntime) ProposeIntakeKey(ctx context.Context, token string, in web.IntakeKeyInput) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	change := access.IntakeKeyChange{KeyName: in.KeyName, Action: strings.TrimSpace(in.Action), AccountableHuman: in.AccountableHuman, ExpiresAt: in.ExpiresAt, OverlapMinutes: in.OverlapMinutes, Reason: in.Reason}
	_, err = r.accessService.ProposeIntakeKey(ctx, principal, newOperatorID("INTAKEKEY"), change, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) ConfirmIntakeKey(ctx context.Context, token, recordID, version string) (web.IntakeKeySecretView, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return web.IntakeKeySecretView{}, err
	}
	if !ok {
		return web.IntakeKeySecretView{}, app.ErrAuthorisationDenied
	}
	issued, err := r.accessService.ConfirmIntakeKey(ctx, principal, r.access, recordID, version, r.kernel.Clock().Read().Now)
	if err != nil {
		return web.IntakeKeySecretView{}, err
	}
	view := web.IntakeKeySecretView{KeyName: issued.KeyName, AccountableHuman: issued.AccountableHuman, Action: issued.Action, Secret: issued.Secret, ExpiresAt: issued.ExpiresAt.UTC().Format(displayTime)}
	if issued.PreviousUntil != nil {
		view.PreviousUntil = issued.PreviousUntil.UTC().Format(displayTime)
	}
	return view, nil
}

func (r *operatorRuntime) intakeKeyRows(ctx context.Context, principal authz.Principal) ([]web.IntakeKeyRow, error) {
	keys, err := r.accessService.IntakeKeys(ctx, principal, r.kernel.Clock().Read().Now)
	if err != nil {
		return nil, err
	}
	rows := make([]web.IntakeKeyRow, 0, len(keys))
	for _, k := range keys {
		row := web.IntakeKeyRow{KeyName: k.KeyName, AccountableHuman: k.AccountableHuman, Status: k.Status, ExpiresAt: k.ExpiresAt.UTC().Format(displayTime), IssuedAt: k.IssuedAt.UTC().Format(displayTime), CanRotate: k.CanRotate, CanRevoke: k.CanRevoke}
		if k.RotatedAt != nil {
			row.RotatedAt = k.RotatedAt.UTC().Format(displayTime)
		}
		if k.PreviousUntil != nil {
			row.PreviousUntil = k.PreviousUntil.UTC().Format(displayTime)
		}
		rows = append(rows, row)
	}
	return rows, nil
}
