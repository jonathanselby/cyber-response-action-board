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

### Phase 3 human UAT — PENDING

**Build:** 0.0.21-dev release candidate (Phase 2 response complete).  
**Script:** `docs/uat-phase3-script.md` — every Phase 2 concern mapped to a section and its tests.  
**Reference renders:** `ci/visual-capture` branch (`shots/`, `pdf/`), produced by `.github/workflows/visual-capture.yml` from seeded Bafflewick data.  
**Automated rows:** T-193 to T-210 green in CI.

| Row | Result |
|---|---|
| T-187 phone capture | pending |
| T-188 keyboard and zoom | pending |
| T-189 colour independence | pending |
| T-190 responsive frame | pending |
| T-191 operating picture comprehension | pending |
| T-192 approval comprehension | pending |
| T-211 Storm Blue | pending |
| T-212 print | pending |
| T-213 Command Summary in 30 seconds | pending |

AC-43 is signed off here only when every row passes.
