# Requirement from CRAB to Garage `notifier` v0.2

**Date:** 2 November 2026
**From:** CRAB post-UAT remediation (`docs/uat-remediation-triage.md` §3.5, S25N; decisions U-13 and U-20)
**To:** `jonathanselby/notifier` (current v0.1.0: `file` and `http` channels, fixed targets, no retry)
**Sequence (decided):** notifier → component tests → CRAB integration → CRAB tests. CRAB builds no temporary notification plumbing in the meantime.

> **CRAB contains the truth. Notifications tell someone to look at CRAB.**

## What CRAB needs

CRAB must notify **a specific person** (an action owner, an eligible confirmer, a clock owner, a Sponsor) through a shared transport such as an SMTP relay or SMS gateway. In v0.1 a channel is a fixed target and a request names channels only, so a person can be reached only by a per-person channel in deployment configuration. That puts personal data in deployment config and needs a restart for every new person.

## Required changes

1. **Per-recipient addressing.** A channel may declare `addressing: "per_recipient"`. A request then carries `deliveries`, each with the channel name, the address (`to`) and an opaque caller reference (`ref`). Fixed-target channels and v0.1-style requests (`channels: [...]`) keep working unchanged.
2. **Address allow-pattern per channel.** A per-recipient channel declares `address_pattern` (a regular expression). A `to` that does not match is refused as `failed` with detail `address refused` before any I/O, so the component cannot become an open relay.
3. **Outcomes echo `ref`, never `to`.** The address must not appear in outcomes, errors or logs, so callers can record outcomes in permanent audit stores without copying personal data.
4. **Channel types:**
   - `email`, via SMTP submission with STARTTLS required, authentication referenced by environment variable name, and the sender fixed in config;
   - `sms-http`, an HTTPS POST to an organisation gateway with a body template supporting `{to}` and `{message}`;
   - `http` with an optional body shape for Teams/Slack-style incoming webhooks.

   No vendor SDKs.
5. **Unchanged v0.1 guarantees:**
   - exercise suppression before I/O for every external channel;
   - per-attempt timeout;
   - every requested delivery gets exactly one `delivered | failed | suppressed` outcome;
   - one failure does not stop later deliveries;
   - no retry, queue or delivery guarantee;
   - a 2xx is not treated as end-recipient receipt.
6. **External is derived from type.** `email`, `sms-http` and `http` are always external. `file` is never external.

## Shape (illustrative; the component owns the final form)

```json
// config
{"channels": [
  {"name": "sms", "type": "sms-http", "target": "https://gw.example/send",
   "addressing": "per_recipient", "address_pattern": "^\\+[1-9][0-9]{7,14}$",
   "body_template": "{\"to\":\"{to}\",\"text\":\"{message}\"}"},
  {"name": "mail", "type": "email", "target": "smtp.example:587",
   "addressing": "per_recipient", "address_pattern": "^[^@\\s]+@[^@\\s]+$",
   "from": "crab@example.org", "auth_env": "CRAB_SMTP_AUTH"},
  {"name": "ir-leadership", "type": "http", "target": "https://teams.example/webhook/abc",
   "body_shape": "teams"}
]}

// request
{"message": "CRAB: 6 items on INC-007 are waiting for your decision. Sign in to CRAB.",
 "exercise": false,
 "timeout_ms": 3000,
 "deliveries": [
   {"channel": "mail", "to": "priya@example.org", "ref": "priya.n"},
   {"channel": "sms",  "to": "+61400000789",      "ref": "tom.b"}
 ]}

// result
{"outcomes": [
   {"channel": "mail", "ref": "priya.n", "status": "delivered", "duration_ms": 412},
   {"channel": "sms",  "ref": "tom.b",   "status": "failed", "detail": "gateway timeout", "duration_ms": 3000}
 ],
 "all_succeeded": false}
```

## Out of scope for the component

- Composing messages.
- Choosing recipients.
- Storing addresses.
- Debounce.
- Templates beyond the body template.
- Retries.
- Read receipts.
- Two-way messaging.
- Escalation chains.

CRAB owns all of these that it needs, in its application tier.

## Component acceptance CRAB will rely on

- Per-recipient delivery to each channel type against local test servers (SMTP, HTTPS).
- Pattern refusal before I/O.
- `to` absent from outcomes, errors and logs.
- Exercise suppression before I/O for each external type.
- Mixed fixed and per-recipient requests.
- v0.1 request compatibility.
- Timeouts bounded per delivery.
