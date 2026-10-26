# CRAB — final visual and accessibility acceptance

## Status

**Automated preflight:** executable CI gate.  
**Human rendered review:** required before stable release; deliberately not self-certified by source tests.

The purpose of this gate is to remove mechanical visual/accessibility failures before a human spends time reviewing the rendered product. It does not pretend that source inspection can prove that a browser page or printed sheet looks good.

## Automated contract

`scripts/visual-acceptance-preflight.sh` verifies three groups:

1. **Web:** phone first-report capture remains explicit; operations/navigation surfaces render; responsive breakpoints exist; core controls have practical minimum touch height; keyboard focus is visibly styled; core CSS has no remote-asset dependency.
2. **Print:** Incident Brief renders locally on A4 and Letter; exercise/monochrome meaning does not depend on colour; fixed input renders deterministically.
3. **Application print boundary:** source/provenance marking remains composed from CRAB state and restricted-compartment print remains authorisation-gated.

The acceptance component also closes one concrete defect found during final review: the frozen UX notes required visible keyboard focus, but `app.css` did not explicitly style `:focus-visible`. The correction is CSS-only and does not alter application architecture or security policy.

## Human review still required

Run the composed application with representative Bafflewick data and inspect these surfaces from `UX-PRINT-BUILD-NOTES.md`:

- common operating picture / incident control;
- approval view;
- access administration;
- first-report desktop and phone-width capture;
- evidence register and safe-inline media presentation;
- obligation clocks;
- handover;
- readiness;
- recovery / PIR / closure;
- Incident Brief in colour and monochrome;
- Emergency Response Pack;
- A4 and Letter examples;
- blank contingency forms.

For browser review, confirm keyboard-only operation, visible focus, semantic labels/headings, readable hierarchy, persistent LIVE/EXERCISE text, responsive phone capture, and that status never depends on colour alone.

For print review, confirm no clipping, useful page breaks, readable hierarchy, visible source/version/time markings, monochrome usability and genuinely writable blank forms.

Record defects, fix the failed behaviour, rerun normal CI + Bafflewick + visual preflight, then sign off. Do not replace this with a generic statement that the product is polished.

## Release boundary

A green automated preflight is repository evidence. Human visual sign-off remains empirical release evidence, alongside the physical Raspberry Pi resource-budget run. Neither should be fabricated by CI.

## Review record

### Phase 1 human UAT — FAIL (AC-43)

**Finding:** the browser frontend is not up to enterprise standard.

**Defects confirmed against source at `494bfd9`:**
- no shared application frame;
- navigation injected by HTML rewriting;
- minimal stylesheet with no tablet/desktop layout or dark mode;
- no persistent status/context area (AC-55, AC-88);
- machine-format inputs;
- approval surface showing record types and identifiers (UX 12);
- paper path absent from the web interface.

**Response:** Slice 15 — rebuild the presentation tier on `go-app-shell` v0.1.0 per `UI-SHELL-BUILD-NOTES.md`. No PRD or EDD change. Tests T-166 to T-192 added in `TEST-ACCEPTANCE-NOTES.md` §3. Re-run this human review after S15.3, record results here, and sign off only when they pass.

### Phase 2 human UAT — NOT ACCEPTED (AC-43 remains open)

**Date:** 26 October 2026 (feedback file is labelled `20261028`).  
**Build:** `78bfd4a`, 0.0.15-dev.  
**Reviewer finding:** the Slice 15 frame is structurally accepted, but the visual layer is not. The UAT owner supplied a locked design direction — *CRAB Visual Design Handover — Storm Blue Locked* (v2), a UI design guide, a handover guide, a Command Summary mockup and a new logo — and asked for the interface to be brought to it.

**Additional UAT findings recorded with this review:**
- password setup lacks a special-character requirement (upper and lower case also required);
- a summary page is needed that users see after sign-in and use as their home;
- reports need a simpler print preview that shows the finished PDF;
- six product gaps, all already required by PRD V0.2 and several already routed in `UI-SHELL-BUILD-NOTES.md` §19/§22: time authority (G-9), organisation emergency configuration for the paper pack, related incidents (AC-84), single-person window (G-6, AC-07), governed severity scale (G-1, AC-71), governed checklist templates (G-7).

**Human rows:** T-187 to T-192 were not individually scored in the feedback. They are recorded as *not signed off* and will be re-run against the Storm Blue build.

| Row | Result |
|---|---|
| T-187 phone capture | not scored — re-run |
| T-188 keyboard and zoom | not scored — re-run |
| T-189 colour independence | not scored — re-run (dark and monochrome included) |
| T-190 responsive frame | not scored — re-run |
| T-191 operating picture comprehension | not scored — re-run |
| T-192 approval comprehension | not scored — re-run |

**Response:** triage and proposed plan in `docs/uat-phase2-triage.md`. No code changes until the plan is approved. PRD V0.2 and EDD V0.4 are unchanged; every finding maps to an existing requirement or to presentation quality.
