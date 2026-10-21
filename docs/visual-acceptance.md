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
