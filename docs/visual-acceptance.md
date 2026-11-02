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

**Response:** triage in `docs/uat-phase2-triage.md`; built in Stages S16–S20 (0.0.16-dev to 0.0.20-dev) per `UAT2-BUILD-NOTES.md`, tests T-193 to T-213 in `TEST-ACCEPTANCE-NOTES.md` §4. PRD V0.2 and EDD V0.4 unchanged.

### Phase 2 human UAT — NOT ACCEPTED (AC-43 remains open)

**Date:** 26 October 2026 (feedback file is labelled `20261028`).  
**Build:** `78bfd4a`, 0.0.15-dev.  
**Reviewer finding:** the Slice 15 frame is structurally accepted, but the visual layer is not. The UAT owner supplied a locked design direction — *CRAB Visual Design Handover — Storm Blue Locked* (v2), a UI design guide, a handover guide, a Command Summary mockup and a new logo — and asked for the interface to be brought to it.

**Additional UAT findings recorded with this review:**
- password setup lacks a special-character requirement (upper and lower case also required);
- a summary page is needed that users see after sign-in and use as their home;
- reports need a simpler print preview that shows the finished PDF;
- six product gaps, all already required by PRD V0.2 and several already routed in `UI-SHELL-BUILD-NOTES.md` §19/§22: time authority (G-9), organisation emergency configuration for the paper pack, related incidents (AC-84), single-person window (G-6, AC-07), governed severity scale (G-1, AC-71), governed checklist templates (G-7).

**Environment:** Linux laptop, Intel Core Ultra 9, 32 GB unified RAM.

**Functional result:** every flow exercised worked. No functional, security or workflow defect was reported. The feedback is UAT direction on visual quality and product completeness, not defects in the Slice 15 build.

| Row | Result |
|---|---|
| T-187 phone capture | pass (functional) |
| T-188 keyboard and zoom | pass (functional) |
| T-189 colour independence | pass (functional) |
| T-190 responsive frame | pass (functional) |
| T-191 operating picture comprehension | pass (functional) |
| T-192 approval comprehension | pass (functional) |

**Verdict:** AC-43 stays open only because the UAT owner has set a new locked visual direction (Storm Blue). The rows above are re-run against that build together with T-211 to T-213.

**Response:** triage in `docs/uat-phase2-triage.md`; built in Stages S16–S20 (0.0.16-dev to 0.0.20-dev) per `UAT2-BUILD-NOTES.md`, tests T-193 to T-213 in `TEST-ACCEPTANCE-NOTES.md` §4. PRD V0.2 and EDD V0.4 unchanged.

### Phase 3 human UAT — NOT ACCEPTED (recorded 2 November 2026, F-6)

**Build:** 0.0.21-dev release candidate (`66b188a`).
**Script:** `docs/uat-phase3-script.md`.
**Outcome:** not accepted. The UAT owner did not return the scored rows below. They returned the *CRAB — Post-UAT Remediation Build Brief*: 22 findings, four of them P0 (people could not be added after setup, so external or temporary people could not be used), plus a final-gate and handover requirement. The brief is triaged in `docs/uat-remediation-triage.md` and every finding is tracked in `docs/remediation-register.md`.

| Row | Result |
|---|---|
| T-187 phone capture | not scored; superseded by the brief (R-17, R-20) |
| T-188 keyboard and zoom | not scored; superseded by the brief (R-17, R-18) |
| T-189 colour independence | not scored; superseded by the brief (R-18, F-16) |
| T-190 responsive frame | not scored; superseded by the brief (R-20, F-15) |
| T-191 operating picture comprehension | not scored; superseded by the brief (R-21) |
| T-192 approval comprehension | not scored; superseded by the brief (R-22) |
| T-211 Storm Blue | not scored |
| T-212 print | not scored |
| T-213 Command Summary in 30 seconds | not scored; superseded by the brief (R-08, R-21) |

The rows are not marked as passed or failed individually, because no such scores were returned. Each is exercised again in Phase 4.

**Response:** post-UAT remediation, chunks C0 to C13 (`docs/remediation-build-plan.md`), 0.0.22-dev to 0.0.34-dev. Every register row now has a result. The final automated gate (`scripts/final-gate.sh`) runs in the `final-gate` workflow.

### Phase 4 human UAT — PENDING

**Build:** 0.0.34-dev release candidate.
**Script:** `docs/uat-phase4-script.md`. About 90 minutes: a fresh install and two browser sessions.
**Automated evidence:** the `final-gate` workflow on `main`.

| Section | Result | Findings |
|---|---|---|
| 1 Setup and sign-in | pending | |
| 2 People | pending | |
| 3 Incident | pending | |
| 4 Compartment | pending | |
| 5 Evidence and indicators | pending | |
| 6 First reports | pending | |
| 7 Communications and notifications | pending | |
| 8 Obligations and closure | pending | |
| 9 Command Summary | pending | |
| 10 Print | pending | |
| 11 Devices (record exactly what was tested) | pending | |

AC-43 is signed off here only when every row passes.
