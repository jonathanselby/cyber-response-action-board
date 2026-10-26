# CRAB build evidence — 0.0.16-dev

**Date:** 26 October 2026  
**Status:** development checkpoint. UAT Phase 2 response, Stage 1 (quick corrections).  
**Governing notes:** `UAT2-BUILD-NOTES.md` §2, `TEST-ACCEPTANCE-NOTES.md` §4

## Scope

| Item | Result | Tests |
|---|---|---|
| Password rule: 12+ characters with upper, lower and symbol, in one `authn` function | done | T-193 |
| One time authority: host `adjtimex` reading, kernel caps every unsigned write | done | T-194, T-195, T-196 |
| Readiness, status line, print mark and first-report default use the live clock | done | T-195, T-196 |
| `TIME UNVERIFIED` notice on every authenticated page while the clock is unverified | done | T-195 |
| PDF *Preview* (inline, new tab) beside every PDF download | done | T-197 |
| Organisation Emergency Response Pack on Readiness (`/pack.pdf`) | done | T-197 |

PRD V0.2 and EDD V0.4 are unchanged. No dependency, table or migration was added.

## Gates run in the build environment

| Gate | Result |
|---|---|
| `gofmt -l ./cmd ./internal` | clean |
| `go vet ./...` | pass |
| `go test -race -count=1 ./...` with PostgreSQL 16 | pass |
| `scripts/visual-acceptance-preflight.sh` | pass |
| `scripts/bafflewick-acceptance.sh` | 8/8 |
| `scripts/build.sh` linux/amd64 | pass; 17,506,567 bytes |
| `scripts/build.sh` linux/arm64 | pass |

## Decisions recorded during build

- **Time status is an upper bound.** Constructors keep the caller's qualification; the kernel stores the weaker of that and the clock. A record can be downgraded by the caller (for example a paper record marked approximate) but never stronger than the clock at write.
- **Decided records are stamped at decision time.** The access and corpus projections no longer compare a decision's time status with its proposal's. They still bind version, payload, maker, compartment and mode.
- **Inline PDFs carry a PDF-specific policy.** `frame-ancestors 'none'`, `nosniff`, `no-store` and `X-Frame-Options: DENY`. The HTML policy (`object-src 'none'`) is not applied to the PDF body because some browser viewers treat it as blocking the viewer itself. Evidence attachments are unchanged.
- **This build host reports `untrusted`.** The sandbox clock is not NTP-disciplined, which exercised the unverified path end to end. GitHub-hosted runners are synchronised.

## Build-environment note

As in 0.0.15-dev, the sandbox could not reach the Go module proxy. Go 1.27.2 was built from the official `github.com/golang/go` tag; modules were fetched from their GitHub sources and checked against the committed `go.sum`; `golang.org/x/*` were mapped to their GitHub mirrors through a workspace file outside the repository. Nothing about this is committed. CI resolves modules normally.

## Open evidence

- Browser behaviour of the inline PDF viewer (Chrome, Firefox, Safari) is checked in Phase 3 UAT (T-212).
