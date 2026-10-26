# CRAB go-app-shell dependency decision

## Decision

Add `github.com/jonathanselby/go-app-shell` `v0.1.0` as a direct runtime dependency of the CRAB browser presentation tier.

This is the short explicit design note EDD V0.4 §26 requires for an additional direct runtime dependency. It does not reopen the EDD: the dependency implements EDD §15 (Go `net/http`, `html/template`, embedded local assets, no Node/SPA build chain) more completely than CRAB's current hand-built chrome.

Status: **accepted for Slice 15**, conditional on the gate below.

## Why the standard library and existing dependencies are insufficient

They are sufficient in principle; CRAB's current web tier is built from them. Phase 1 human UAT showed that the result does not meet AC-43 (web visual quality). Rebuilding a responsive, accessible, light/dark application frame inside CRAB would duplicate a released Garage component built for that job.

`go-app-shell` is presentation infrastructure only. It owns:

- the application frame;
- responsive navigation (phone drawer, tablet rail, desktop rail);
- layout tokens;
- light/dark presentation;
- accessible landmarks, focus and touch targets;
- optional static-only PWA plumbing.

It owns no business logic, authentication, authorisation, routing, persistence or client state.

## Properties relied upon

- Public importable Go package; `Renderer.Render(io.Writer, Page)`; independent renderer instances, no global engine.
- Standard-library-only runtime; adds no transitive modules to CRAB's module graph.
- `go.mod` declares `go 1.22`. This is inside CRAB's Go 1.25 module baseline, and the shell's supported CI is Go 1.27, matching CRAB CI 1.27.2.
- Linux arm64 build is part of the shell's CI.
- Assets are embedded with system fonts. There are no CDN, external font or runtime network dependencies.
- No inline `<script>` or `style` attributes in shell markup; compatible with CRAB's existing `Content-Security-Policy` (`default-src 'self'; style-src 'self'`).
- Zero required JavaScript; core navigation works with JavaScript and CSS disabled.
- Application content slots are explicit `template.HTML` trust boundaries, so CRAB keeps the trust decision.
- The PWA service worker only serves explicitly listed same-origin static URLs plus an optional offline page, and never queues or replays non-GET requests.
- First-party CSS + optional PWA JS ≤ 32 KiB (14,503 bytes at the shell's release evidence).
- Released `v0.1.0` (tag `b340bd6`) after automated acceptance and human UAT on Linux Chrome, Linux Firefox and Android Chrome.

## Boundary rules in CRAB

- Only `internal/presentation/web` may import the shell. This is enforced by `internal/architecture/imports_test.go` (T-166).
- The shell is used unmodified, with no fork, vendored patch or local edit.
- CRAB's security headers, session handling, same-origin mutation guard and application-tier authorisation are unchanged.
- Version is pinned through `go.mod`/`go.sum`. Upgrading is a deliberate change with the visual preflight and human review re-run.

## Rejected alternatives

**Continue hand-building chrome in CRAB.** This is what UAT rejected. It would duplicate a released Garage component inside an application that should own meaning, not layout.

**Node/SPA frontend (React, Vue, etc.).** Rejected by EDD §15 and §31 locked removals, and by the UX stop sign against Node/SPA complexity for visual problems.

**HTMX/Alpine or similar.** These add a client runtime for interactions CRAB's server-rendered forms already perform; the shell explicitly does not require them.

**CSS framework (Bootstrap/Tailwind).** It brings either a build step or a large stylesheet, does not provide the responsive application frame or PWA boundary, and would still leave the layout to CRAB.

**Extract CRAB's chrome into a new micro-component.** BUILD-NOTES §20 forbids speculative extraction; the reusable component already exists.

## Gate

If S15.3 human review shows the shell cannot meet the AC-43 bar for CRAB, fix the defect upstream in go-app-shell through its own release process, or replace the shell. Do not weaken the product-quality requirement to keep the dependency, and do not patch the shell inside CRAB.

Safari/iOS behaviour was not certified in shell v0.1.0 UAT. CRAB's human review should include WebKit/Safari where available and record the result rather than infer it.
