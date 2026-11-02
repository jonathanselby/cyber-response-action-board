# First-report intake API

**Status:** CRAB 0.0.27-dev. Contract addendum V0.2.1 A4 (approved 2 November 2026).

A security tool or script can submit a first report to CRAB. That report is an ordinary first report: it waits on the Intake screen for a person to triage it, and it never becomes an incident without a human decision.

## Getting a key

1. A Custodian opens **Access → Intake keys → Issue an intake key**. They enter a key name (for example `siem-prod`), the person accountable for what the system sends, an expiry (at most 365 days ahead) and a reason.
2. A different Custodian confirms it in **Approvals**. The secret is shown to them **once**. They should put it straight into the sending system's secret store.

The key can submit first reports and nothing else. It cannot read anything, it is not a session, and it never needs an authenticator code. CRAB stores only a hash of the secret.

- **Rotate:** propose a rotation, with an overlap of 0 to 24 hours. Once a second Custodian confirms it, they are shown a new secret. The old secret keeps working until the overlap ends.
- **Revoke:** propose a revocation. Once a second Custodian confirms it, the key stops working immediately.
- **Accountable person:** if the accountable person is suspended, disabled or their account ends, the key stops working too.

## Request

```bash
curl -sS https://crab.example.org/api/v1/first-reports \
  -H "Authorization: Bearer siem-prod.<secret>" \
  -H "Content-Type: application/json" \
  -d '{"description":"EDR: credential dumping on FIN-WS-22","affected":"FIN-WS-22","external_ref":"edr-4411"}'
```

The request is one JSON object of at most 64 KiB. TLS is required, except from loopback. Unknown fields are refused, and attachments cannot be sent.

| Field | Required | Meaning |
|---|---|---|
| `description` | yes | What was seen |
| `affected` | no | Systems, people or services affected |
| `immediate_impact` | no | Impact now |
| `reporter` | no | Who or what saw it. The default is the key name |
| `contact_route` | no | How to reach the reporter |
| `received_at` | no | When the sender saw it (RFC 3339). If you leave it out, CRAB uses its own time and marks it trusted |
| `time_confidence` | no | `trusted`, `drifting` or `untrusted`. Used only with `received_at`; the default is `untrusted` |
| `external_ref` | no | The sender's own ID, at most 128 characters. A repeat with the same key returns the original report and records nothing new |

Every report is shown with its source as *intake key `<name>`*. It is recorded in the ledger with the accountable person as maker.

## Responses

| Status | Body | Meaning |
|---|---|---|
| `201` | `{"report_id":"…","status":"recorded"}` | Recorded |
| `200` | `{"report_id":"…","status":"already recorded"}` | Repeat of an earlier `external_ref`. Nothing new was recorded |
| `400` | text | Malformed request or report |
| `404` | `not found` | The key was refused |
| `429` | text, `Retry-After: 60` | Over the limit. Nothing was recorded or queued |

A key is refused if it is expired, revoked, rotated out, wrong, unknown or malformed, or if its accountable person cannot act. Every one of these gets the same `404`, the same response as any other API refusal.

## Limits

- **Per key:** 60 accepted submissions a minute. A request over the limit is not queued; send it again later. Using `external_ref` makes retries safe.
- **Per source:** 50 refused key presentations from one address in 15 minutes. After that, every presentation from that address is refused until the window passes.

## What this is not

It is not a general API login, an OAuth server, a service account, a vendor connector, a polling client or an outbound webhook.
