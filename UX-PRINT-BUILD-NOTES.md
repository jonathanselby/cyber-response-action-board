# CRAB — UX and Print Build Notes

**Version:** 0.1  
**Date:** 20 October 2026  
**Companion to:** `CRAB-EDD-v0.4-FINAL.md`

These notes protect product quality while the engineering underneath is aggressively simplified.

The key rule is:

> **Generic mechanisms must never force generic behaviour, language or presentation.**

CRAB should feel like a purpose-built incident-response operating product, not a database editor exposing a clever generic schema.

---

## 1. Experience objective

Under pressure, a user should be able to answer quickly:

- What incident am I in?
- Is this live or an exercise?
- What compartment am I working in?
- What is happening now?
- What do we know versus suspect versus not know?
- Who is in charge of what?
- What decisions are waiting?
- What actions are overdue?
- What clocks are running?
- Who has been told what?
- What changed since the last shift?
- Is CRAB itself healthy?
- What happens if digital operation fails?

The interface should not require understanding ledger terms, projection terms or Item envelopes.

---

## 2. Visual register

Use the established Jonno's Garage visual grammar where appropriate:

- white/light background;
- restrained high-whitespace composition;
- square/controlled corners;
- thin structural lines;
- shaded panels without heavy borders;
- clear hierarchy;
- restrained green accent/icon vocabulary;
- no decorative gradients or dashboard theatre;
- colour supports meaning but never carries meaning alone.

This is an incident command product, not a security-operations neon display.

Warnings should be unmistakable without making every screen look alarming.

---

## 3. Global shell

Every operational page should maintain a compact persistent context area with:

- CRAB identity/version as appropriate;
- current incident code/title if authorised;
- LIVE / EXERCISE marking;
- derived operational phase;
- current compartment/visibility context;
- time trust;
- isolate/break-glass status when active;
- high-priority readiness degradation;
- pending approval count relevant to the user.

Do not leak unavailable incidents/compartments through switchers/counts.

---

## 4. Primary incident board

The board is the centre of the product.

It should surface the operating picture rather than mirror database tables.

Suggested information hierarchy:

### Immediate situation

- current situation statement;
- severity + recent change indicator;
- impact/scope summary;
- significant unknowns/hypotheses;
- derived phase.

### Command and ownership

- operational role holders/deputies;
- current Sponsor where appropriate to authorised users;
- critical actions/owners;
- pending decisions/approvals.

### Time pressure

- obligation clocks;
- overdue actions;
- next situation-report/communications commitment;
- time-trust warning if needed.

### Recent change

- recent confirmed decisions;
- material facts/events;
- communications outcomes;
- significant access/role changes where relevant.

### System resilience

- isolate/break-glass status;
- paper-pack age/freshness;
- relevant readiness warnings;
- model lane availability shown as optional assistance, never as core system health.

Avoid an undifferentiated wall of cards.

---

## 5. Semantic actions

Use explicit verbs throughout.

Preferred examples:

- Declare Incident
- Record First Report
- Record Fact
- Record Unknown
- Add Hypothesis
- Update Scope
- Change Severity
- Add Action
- Complete / Skip Action
- Record Decision
- Add Evidence
- Draft Situation Report
- Issue Communication
- Request Incident Access
- Read Into Compartment
- Revoke Access
- Record Enquiry
- Generate Handover
- Stand Down Response
- Start / Confirm PIR
- Close Incident

Avoid `Create Item`, `Edit Item`, `Item Type`, `Item Status` in primary UX.

Internal developer/admin diagnostics may show underlying types where useful.

---

## 6. Record versus governed-change UX

Users should understand whether they are preserving an observation or changing effective state without learning the implementation terms.

Examples:

**Immediate record**

- “Record enquiry”
- “Add progress note”
- “Upload evidence”
- “Record delivery outcome”

The UI should make it clear the record is preserved as supplied.

**Governed change**

- “Propose severity change”
- “Propose stand-down”
- “Submit communication for approval”
- “Request access”

The UI should clearly show:

- current effective value/state;
- proposed new value/version;
- maker;
- required confirmer;
- reason/evidence;
- pending/confirmed/rejected outcome.

---

## 7. Facts, unknowns and hypotheses

These must be visually distinguishable without relying on colour alone.

Use differences such as:

- icon/label;
- heading language;
- badge text;
- border/shape treatment;
- explicit confidence/source fields where appropriate.

A hypothesis must never look like a confirmed fact merely because it appears in the same timeline.

The common operating picture should show important unknowns prominently; absence of knowledge is operational information.

---

## 8. Operational phase

Although phase is derived, it should look first-class.

Possible labels:

- Active Response
- Recovery
- Stood Down / Review
- Closure Ready
- Closed

Do not expose the implementation fact that the persisted incident state is only live/closed.

The product should communicate the operating reality, not storage economy.

---

## 9. Action/checklist UX

Actions need a purpose-built surface.

Show:

- description;
- owner;
- status;
- priority;
- due time/overdue state;
- source checklist/decision/runbook;
- evidence/notes;
- latest update.

Checklist views should feel like checklists even though they create ordinary Actions underneath.

Skipping requires a reason.

Do not expose a workflow diagram.

---

## 10. Obligation/clocks UX

Clocks deserve strong visual priority when active.

Distinguish:

- provisional versus firm;
- upcoming versus overdue;
- trusted versus time-unverified;
- original deadline versus amended/extended deadline;
- owner/recipient;
- trigger/source.

Do not imply legal applicability. Where a trigger/applicability remains unresolved, state that clearly rather than showing false precision.

---

## 11. Access and compartment UX

Security can be strong without making the product bureaucratic.

For normal incident content, a visibility selector can present configured compartments directly:

```text
Visibility
  General incident
  Legal
  Board
  Forensics
  <other configured compartment>
```

The product translates this into the underlying compartment model.

For access administration, keep two different questions clear:

- **Incident access:** should this person be part of this incident?
- **Compartment read-in:** should this person know this restricted body of information?

Custodian screens should show enough metadata to administer the grant without opening protected content.

Expiry and revocation should be visible and easy to operate for external participants.

---

## 12. Approval UX

Approval should show the exact version being confirmed.

The confirmer needs:

- change summary;
- old/effective state;
- proposed state;
- maker;
- reason/evidence/source;
- compartment/classification;
- consequences where meaningful;
- confirm/reject controls.

Never show a generic “Approve Item #123” as the primary decision surface.

---

## 13. Timeline UX

Timeline is a projection, but the UI should permit useful filtering by semantic type without fragmenting the history.

Potential filters:

- All
- Facts / Unknowns
- Decisions
- Actions
- Evidence
- Communications
- Access / Roles
- Obligations
- Recovery / Closure

Display event-time semantics clearly where multiple times matter.

Avoid a decorative graphical timeline if a dense chronological table/list is more useful under pressure.

---

## 14. Evidence and media UX

Evidence should have a dedicated register and detail surface.

Show relevant metadata without implying forensic certification.

For safe inline media:

- inline only when allow-listed by server policy;
- clear download action where permission allows;
- show filename/type/size/hash/source/collector;
- label integrity failure conspicuously;
- do not auto-open active files;
- do not auto-send evidence to models.

Phone capture flow should be short:

1. choose/capture media;
2. description/source as needed;
3. compartment/visibility;
4. upload progress;
5. clear success/failure.

Do not reproduce a desktop evidence form on a small phone screen.

---

## 15. Communications UX

Differentiate:

- draft;
- pending approval;
- approved;
- issued/attempted;
- delivered/failed/suppressed;
- acknowledgement/response.

The user should be able to see exactly what version was approved and what version was issued.

Incoming enquiries should look like assigned response work, not a chat inbox.

Situation-report cadence should be visible on the board/handover.

---

## 16. Handover UX

Handover should be a purpose-built summary, not a filtered database dump.

Target sections:

- Situation now
- Impact/scope
- Roles/deputies
- Critical open actions
- Pending decisions
- Active clocks
- Unknowns/questions
- Significant recent change
- Next objectives
- Communications commitments

Allow generation of an immutable handover record where useful, but the live view remains derived/current.

---

## 17. Exercise UX

Exercise mode must be impossible to miss.

Use persistent textual `EXERCISE` marking on:

- shell/header;
- incident board;
- communications;
- PDFs;
- exports.

Do not rely on colour alone.

Exercise controls should otherwise be the same as live controls so rehearsal creates real familiarity.

---

## 18. Readiness UX

Readiness should answer one question quickly: **can we operate?**

Prefer a compact grouped report with:

- PASS / WARN / FAIL text/icon;
- concise reason;
- last successful evidence/date where useful;
- direct corrective navigation for things CRAB can configure;
- explicit operator instruction for deployment issues.

Model availability should not cause an overall FAIL when deterministic core operation is healthy.

---

## 19. Recovery/PIR/closure UX

Do not collapse these because the backend lifecycle is simple.

Recovery surface:

- restoration priorities;
- verification;
- residual risk;
- open recovery actions.

Stand-down:

- explicit consequential decision;
- reason/evidence;
- confirm/reject.

PIR:

- pre-populate from ledger;
- make human lessons clearly editable/proposable;
- distinguish generated summary from confirmed PIR.

Closure:

- show prerequisites and blockers;
- clearly state that closure makes the incident read-only;
- offer separate privileged/shareable closure artefacts.

---

## 20. Retention/hold UX

After closure show:

- governing retention policy/source;
- retention date/period;
- active hold and reason/authority;
- eligibility status;
- disposition/handoff decision history.

Do not show an ordinary Delete Incident button anywhere.

---

## 21. Mobile scope

V0.4 is not a native mobile application.

Phone browser is an explicit acceptance surface for:

- first-report intake;
- photo/audio/video/file capture;
- concise metadata entry;
- upload result.

Full incident command remains optimised for normal laptop/desktop browser sizes but should remain responsive rather than deliberately unusable on tablets/phones.

---

## 22. Accessibility

At minimum:

- keyboard operation for core workflows;
- visible focus;
- semantic headings/labels;
- status text/icons beyond colour;
- readable contrast;
- practical touch targets;
- table alternatives/responsive treatment where necessary;
- screen text that distinguishes operational role from system permission;
- predictable denied/hidden behaviour.

Accessibility defects in core incident flows block acceptance.

---

## 23. Print system

Print has its own design grammar and templates.

Do not simply reuse screen CSS.

Common print tokens:

- A4 and Letter page geometry;
- safe margins;
- header/footer positions;
- page numbering;
- typography scale;
- table rules;
- status/classification markings;
- source ledger/config version;
- stale/current indicator;
- monochrome treatment.

### Incident Brief

Target one to two pages where possible.

Optimise for someone walking into a room and needing the current picture quickly.

### Emergency Response Pack

Optimise for utility when CRAB itself cannot be reached.

Blank forms must be genuinely usable by hand.

### Evidence register / timeline / PIR / closure

Each deserves a purpose-built print layout rather than one universal table renderer.

---

## 24. Visual acceptance evidence

Retain representative screenshots/PDF fixtures for:

- common operating picture;
- approval view;
- access administration;
- first-report desktop;
- first-report phone;
- evidence register;
- safe inline media;
- obligation clocks;
- handover;
- readiness;
- recovery/PIR/closure;
- Incident Brief colour;
- Incident Brief monochrome;
- Emergency Response Pack;
- A4 and Letter examples;
- blank forms.

Review should record actual defects and fixes rather than a statement that the product is “polished”.

---

## 25. UX stop signs

Stop and correct course if any of these emerge:

- primary navigation organised around generic Item types rather than incident work;
- generic create/edit form used for semantically different critical operations;
- users need to understand role × incident × compartment algebra to use ordinary screens;
- restricted content requires multi-screen bureaucracy for common classification;
- status relies on colour;
- operational phase disappears because it is derived;
- safe evidence becomes unnecessarily download-only without testing passive inline media;
- every feature becomes a card/dashboard widget;
- phone capture mirrors a desktop form;
- print output resembles a database dump;
- Node/SPA complexity is introduced to solve a visual problem that plain HTML/CSS/small JS can solve;
- implementation simplicity is used to justify lower product quality.
