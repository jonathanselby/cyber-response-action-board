# Setup origin-null bug

Observed during Ubuntu/Chrome functional acceptance on 2026-10-22.

## Symptom

First-run two-person bootstrap returned `setup refused`.

Diagnostic request:

```text
remote=127.0.0.1:45634
host=127.0.0.1:8082
origin=null
referer=
```

## Root cause

CRAB's security middleware sent `Referrer-Policy: no-referrer`. For ordinary HTML form POSTs, browsers may serialize the `Origin` request header as `null` under this policy. CRAB's same-origin mutation guard then rejected that request, creating a contradiction between CRAB's response policy and its own request validation.

## Corrective action

Change the response policy to `Referrer-Policy: same-origin`. This keeps referrer data suppressed on cross-origin requests while allowing same-origin CRAB form POSTs to carry usable origin context. The origin guard remains fail-closed for foreign origins.

Regression coverage asserts the intended response policy.
