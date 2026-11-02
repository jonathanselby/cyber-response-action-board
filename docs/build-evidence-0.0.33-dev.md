# CRAB build evidence — 0.0.33-dev

**Date:** 2 November 2026
**Status:** development checkpoint. Post-UAT remediation, chunk C12 (notifications).
**Governing documents:** `docs/uat-remediation-triage.md` §3.5, S25N; `docs/notifier-v0.2-requirement.md`; `docs/remediation-build-plan.md` C12; `TEST-ACCEPTANCE-NOTES.md` (T-233, T-234, T-251, T-252)
**Commits:**

| Commit | Content |
|---|---|
| `56292f6` | implementation |
| `67f2aa9` | review fixes |
| `b444202` | review fix: ordering |

**Dependency:** `github.com/jonathanselby/notifier` v0.2.0, released this session. The component's own CI is green, and its evidence is pinned to `ad9ddf1`.

## Sequence followed

The decided sequence (U-13) was notifier, then component tests, then CRAB integration, then CRAB tests. The notifier was at v0.1.0, with no per-person addressing and no importable package. With the product owner's go-ahead, v0.2.0 was built in `jonathanselby/notifier` to the requirement:

- a public `notify` package;
- `deliveries[{channel,to,ref}]`;
- an anchored `address_pattern`, checked before any I/O;
- outcomes that carry `ref`, never the address;
- `email` (STARTTLS required) and `sms-http` channels, and a `text` body shape for webhooks;
- v0.1 configuration, requests and outcomes unchanged.

Its component acceptance runs against local SMTP (STARTTLS) and HTTPS servers. CRAB then moved to it.

## Scope

| Item | Result | Tests |
|---|---|---|
| CRAB delivers through notifier v0.2.0. The in-repo transport is replaced by an adapter that maps the deployment file onto the component. `local_file` and `webhook` still work; `email` and `sms-http` are added | done | `TestDeploymentMapsToNotifierChannels`, adapter tests |
| Notification request recorded before any attempt, with attempts linked to it. Delivery table showing *Failed — not retried automatically* and *Not attempted*. The failure stays on the Board until resent | done | T-233 |
| Resend to failed channels: same permission, same approved content (digest checked), failed channels only, a new request linked to the earlier attempts, suppressed in exercise | done | T-234 |
| Routes. One per person, in supporting state (`crab.notification_routes`). The ledger gets `security.notification_route_changed` with a masked hint. Set on *Your account* or by a Custodian. Checked against the channel pattern. Deleted on disable. Readiness warns when Majors or Custodians have none | done | T-252 |
| The five V1 events, with content-free text and scope-safe, debounced decision counts. Shared destinations get only the content-free escalation notice. *No route* is recorded. Notice IDs are deterministic, so nothing is sent twice. Exercise notices are suppressed | done | T-251 |
| Migration 022 (requests, notices, routes, cursor). The schema is now 22 | done | upgrade proof |

## Deliberate choices, stated plainly

- **Settings in the deployment file.** Escalation (event 4), its shared destinations, the decision window and the access lead time are set in the notification deployment file, not in an in-app organisation setting. Event 4 is off unless enabled there.
- **Decision window.** It is measured from the person's last decision notice, recorded in the ledger. No new rows are added to the timed wait list. The behaviour is the same, and no new state is needed.
- **The notice pass.** It runs every 30 seconds under `crab serve`, reading new ledger entries (events 1 and 4), pending proposals (event 2), the timed wait list (event 3) and expiring memberships (event 5). It does not run when no channels are configured, so the load and soak proofs are unaffected.
- **Send permission.** *Send issued communications* (notify) had no route for granting it in the interface, so nobody could send or resend. It is now a governed output permission, next to print and download.
- **Webhook bodies.** These are now the notifier's `{"message": …}`: the subject and body as text, instead of CRAB's earlier JSON object. The webhook acknowledgement header is no longer recorded.
- **Escalation to members.** The text reads *…an incident you are on on INC-7…*. The doubled "on" is in the wording and is left for UAT to judge.

## Review gate

This was a self red-team review.

| Finding | Classification | Action |
|---|---|---|
| Decision counts included pending access changes, so a Minor could be told to decide them | real | counted for Custodians only; asserted in `TestNotifications` |
| A failing notice pass was silent | real | `notice_pass` readiness check; `TestNoticePassFailureIsNeverSilent` |
| Access-expiring notices for a restricted compartment were recorded in GENERAL | real | recorded in the membership's compartment |
| Two attempts in the same second were ordered by record ID, so after a resend the old failure could count as the latest (found by CI under the race detector) | real | ledger sequence stored and used for ordering (`b444202`) |
| The notice pass on a second kernel broke the ledger tail check | harness | the e2e test uses the served runtime (`e2e.served`) |

## Gates

| Gate | Build environment | GitHub CI (`b444202`) |
|---|---|---|
| gofmt, vet (with and without `-tags proof`) | clean | pass |
| `go test -race -count=1 ./...` | pass | pass |
| Bafflewick | **10/10** | pass |
| visual preflight | pass | pass |
| uat-browser (journeys and axe; `/account` added) | Chromium journeys pass | pass, three engines |
| proof (destructive, database, backup, upgrade to schema 22, load, soak) | — | pass |
