package main

import (
	"context"
	"strings"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/presentation/web"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

// People administration and enrolment adapters. They resolve the session and
// translate types; every rule is in app.AccessService and app.Authentication.

func (r *operatorRuntime) peopleRows(ctx context.Context, principal authz.Principal) ([]web.PersonRow, error) {
	people, err := r.accessService.People(ctx, principal, r.kernel.Clock().Read().Now)
	if err != nil {
		return nil, err
	}
	rows := make([]web.PersonRow, 0, len(people))
	for _, p := range people {
		row := web.PersonRow{HumanID: p.HumanID, DisplayName: p.DisplayName, Organisation: p.Organisation, Status: p.Status, Role: p.Role,
			External: p.External, Custodian: p.HoldsCustodian, Usable: p.Usable, Legacy: p.Legacy, VisibleIncidents: p.VisibleIncidents, OtherIncidents: p.OtherIncidents,
			CanSuspend: p.CanSuspend, CanReactivate: p.CanReactivate, CanDisable: p.CanDisable, CanReset: p.CanReset, CanRecordIdentity: p.CanRecordIdentity}
		if p.AccountExpiresAt != nil {
			row.AccountExpiresAt = p.AccountExpiresAt.UTC().Format(displayTime)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// ProposePerson proposes a new person's identity and role eligibility
// together. They share a record-ID prefix so Approvals groups them.
func (r *operatorRuntime) ProposePerson(ctx context.Context, token string, in web.PersonInput) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	now := r.kernel.Clock().Read().Now
	prefix := newOperatorID("PERSON")
	change := access.IdentityChange{HumanID: in.HumanID, DisplayName: in.DisplayName, Organisation: in.Organisation, External: in.External, Status: access.IdentityActive, AccountExpiresAt: in.AccountExpiresAt}
	if _, err := r.accessService.ProposeIdentity(ctx, principal, prefix+"-IDENTITY", change, now); err != nil {
		return err
	}
	_, err = r.accessService.ProposeRoleEligibility(ctx, principal, prefix+"-ROLE", in.HumanID, authz.Role(strings.TrimSpace(in.Role)), true, now)
	return err
}

func (r *operatorRuntime) ProposePersonStatus(ctx context.Context, token, humanID, status, reason string) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	_, err = r.accessService.ProposeIdentityStatus(ctx, principal, newOperatorID("IDENTITY"), humanID, status, reason, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) ProposeCredentialReset(ctx context.Context, token, humanID, reason string) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	_, err = r.accessService.ProposeCredentialReset(ctx, principal, newOperatorID("RESET"), humanID, reason, r.kernel.Clock().Read().Now)
	return err
}

func (r *operatorRuntime) ConfirmCredentialReset(ctx context.Context, token, recordID, version string) (web.ResetCodeView, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return web.ResetCodeView{}, err
	}
	if !ok {
		return web.ResetCodeView{}, app.ErrAuthorisationDenied
	}
	issued, err := r.accessService.ConfirmCredentialReset(ctx, principal, r.credentials, recordID, version, r.kernel.Clock().Read().Now)
	if err != nil {
		return web.ResetCodeView{}, err
	}
	return web.ResetCodeView{HumanID: issued.HumanID, Code: issued.Code, ExpiresAt: issued.ExpiresAt.UTC().Format(displayTime)}, nil
}

func (r *operatorRuntime) BeginEnrolment(ctx context.Context, humanID, code string) (web.EnrolmentView, error) {
	e, err := r.auth.BeginEnrolment(ctx, humanID, code, r.kernel.Clock().Read().Now)
	if err != nil {
		return web.EnrolmentView{}, err
	}
	return web.EnrolmentView{HumanID: strings.TrimSpace(humanID), Code: code, NeedsTOTP: e.NeedsTOTP, TOTPSecret: e.TOTPSecret, TOTPURI: e.TOTPURI}, nil
}

func (r *operatorRuntime) CompleteEnrolment(ctx context.Context, humanID, code, password, totp string) (string, error) {
	return r.auth.CompleteEnrolment(ctx, humanID, code, password, totp, r.kernel.Clock().Read().Now)
}
