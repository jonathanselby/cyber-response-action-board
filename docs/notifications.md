# Notifications

> **CRAB contains the truth. Notifications tell someone to look at CRAB.**

CRAB sends two kinds of notification:

- **Issued communications** (situation reports, regulator notices and so on), sent by a person with the *Send issued communications* permission, to the fixed channels named on the communication.
- **Notices**, sent by CRAB on its own to the person who must act, for exactly five events.

Delivery goes through the Garage `notifier` component. CRAB never retries automatically: a failure stays visible until someone resends it.

## Deployment file

`CRAB_NOTIFICATION_CONFIG_FILE` names a private JSON file (mode 0600):

```json
{"channels": [
  {"name": "ops-log", "type": "local_file", "target": "/var/lib/crab/notify.log", "destination_id": "ops-log"},
  {"name": "regulator", "type": "webhook", "target": "https://portal.example/crab", "destination_id": "regulator-portal", "timeout_seconds": 5},
  {"name": "ir-channel", "type": "webhook", "target": "https://teams.example/webhook/abc", "destination_id": "ir-channel", "body_shape": "text"},
  {"name": "mail", "type": "email", "target": "smtp.example.org:587", "destination_id": "smtp-relay",
   "address_pattern": "^[^@\\s]+@example\\.org$", "from": "crab@example.org", "subject": "CRAB needs you", "auth_env": "CRAB_SMTP_AUTH"},
  {"name": "sms", "type": "sms-http", "target": "https://sms.example/send", "destination_id": "sms-gateway",
   "address_pattern": "^\\+61[0-9]{9}$", "body_template": "{\"to\":\"{to}\",\"text\":\"{message}\"}", "auth_env": "CRAB_SMS_AUTH"}
 ],
 "notices": {
  "escalation_enabled": true,
  "escalation_to_members": true,
  "escalation_shared_channels": ["ir-channel"],
  "decision_window_minutes": 15,
  "access_expiry_lead_hours": 24
 }}
```

- `local_file` and `webhook` are **fixed** channels. They carry issued communications and, if listed under `escalation_shared_channels`, the content-free escalation notice.
- `email` and `sms-http` are **per-person** channels. They carry notices only, to a person's route. Each needs an `address_pattern` anchored with `^` and `$`; an address outside it is refused before anything is sent.
- `auth_env` names an environment variable. For email it holds `user:password`, used for SMTP AUTH after STARTTLS. For sms-http it holds the `Authorization` header value. Credentials never go in the file.
- Webhooks must use HTTPS, except loopback test endpoints.

Without a deployment file, communications are recordable but nothing is sent and no notices run.

## Routes

Each person may have **one** route: a per-person channel and an address.

- The person sets it on **Your account**; a Custodian can set anyone's from **Access admin → People**.
- The address is supporting state in `crab.notification_routes`, included in backups. It is never in the ledger. The ledger records `security.notification_route_changed`, with who changed whose route, the channel and a masked hint (`…789`, `j…@example.org`).
- Setting a route is recorded, not dual-controlled: a notice carries no content and approves nothing.
- A route is never used to sign in, recover an account or verify anyone.
- Disabling a person deletes their route (the audit records stay). Suspension keeps the route; nothing is sent while the account is unusable.
- Readiness warns when Majors or Custodians have no route.

## The five notice events

| Event | Sent to | Notice |
|---|---|---|
| Assignment | the new owner of an action, or holder of an operational role (not the person who made or confirmed it) | *CRAB: you have a new assignment on INC-7. Sign in to CRAB.* |
| Decision needed | each member who may decide **and read** pending items; at most one per person per incident per window, only when something new arrived | *CRAB: 3 items on INC-7 are waiting for your decision. Sign in to CRAB.* |
| Deadline | the obligation clock owner, when a warning or deadline in the timed wait list falls due on an active clock | *CRAB: a deadline you own on INC-7 needs attention. Sign in to CRAB.* |
| Severity or phase change (only if enabled) | members, and shared destinations | members: *…an incident you are on on INC-7 has a confirmed severity or phase change…*; shared: *CRAB: an incident has a confirmed severity or phase change. Sign in to CRAB.* (no incident ID) |
| Access expiring | the Sponsor (incident membership) or Controller (restricted compartment) who can renew temporary access expiring within the lead time | *CRAB: temporary access you can renew on INC-7 expires soon. Sign in to CRAB.* |

Nothing else notifies: no facts, timeline entries, comments, evidence or ordinary changes.

A notice says what kind of thing needs the person and, only if they may read the incident, its ID. It has no titles, summaries, compartment names or links, so it cannot reveal restricted content and cannot approve anything. A decision count includes only items the person may decide and read.

Each notice is recorded as `incident.notice` with its outcome: `delivered`, `failed`, `suppressed` (exercise incidents) or `no route`. With no route nothing is sent, and the item stays on the person's Command Summary. The incident's notices are listed on its Communications screen.

`crab serve` runs the notice pass every 30 seconds. Each notice has an ID derived from its event and recipient, so the pass never sends the same notice twice, including after a restart. On first start it records where the ledger is, so history before notifications were enabled never notifies.

## Delivery of issued communications

Sending records a *notification request* first, then one outcome per channel, linked to it. The Communications screen shows each request:

- channel, time and outcome in words;
- *Failed — not retried automatically*;
- *Not attempted*, for channels a request named but CRAB stopped before trying.

**Resend to failed** sends the same approved content again, only to the channels whose latest attempt failed. It needs the same permission and records a new request linked to the earlier attempts. Exercise suppression still applies. A failure stays on the incident Board until it is resent.
