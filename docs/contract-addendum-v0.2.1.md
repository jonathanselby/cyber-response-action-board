# CRAB — Contract addendum PRD V0.2.1 / EDD V0.4.1

**Date:** 2 November 2026
**Status:** DRAFT for approval. Implementation of the four capabilities below starts only after approval.
**Amends:** `PRD.md` V0.2 and `EDD.md` V0.4. Every other section of both documents is unchanged and still governs.
**Source:** post-UAT remediation brief; decision U-12 in `docs/uat-remediation-triage.md` §0.

> **Four small additions, not four new subsystems.**

This addendum adds exactly four capabilities. Each is defined by what it is, what it is not, and the acceptance criterion that proves it. Anything not written here is out of scope. A proposed implementation that needs more than is written here comes back to the product owner rather than growing.

Not covered here because they need no contract change:
- **User administration and external onboarding.** PRD *Users and authority* and §19 and EDD §9.6 and §13.11 already require them; R-01 and R-02 implement them.
- **External MFA (U-16).** This is a policy within the existing PRD *Operating environment — Identity*.
- **Notification events (U-20).** These sit within PRD §7, §21 and EDD §18. No retry is added.

---

## A1. Incident reopening

**Amends:** PRD §12 (*Confirmed closure makes the incident read-only*), EDD §13.16.

**PRD text added to §12:**

> A closed incident may be reopened by an authorised governed decision with a mandatory reason. Reopening is a new governed event: the original closure, its outputs and all prior history remain immutable and are marked as superseded by the reopening. A reopened incident is no longer read-only and must satisfy the closure prerequisites again before it can be closed again.

**EDD rules:**

- Record type `incident.reopening`, governed (`Propose → Decide`). The maker and confirmer are distinct Majors who are members of the incident. Never covered by a single-person window.
- The confirmed reopening supersedes the current closure and stand-down. Both stay in the ledger. The operational phase is re-derived by the existing rule.
- Prior closure outputs (privileged and shareable) are not edited or deleted. Views and the Brief mark them *superseded by reopening at <time> — <reason>*.
- Access that expired while the incident was closed stays expired. Clocks are not re-armed by reopening.

**Is not:** an edit of closure history, an undo, a merge, or a change to retention or hold records.

**AC-89 — Governed reopening.** A closed incident can be reopened only by a confirmed governed decision with a mandatory reason and a different confirmer. The original closure and its outputs remain immutable and visibly superseded, the incident becomes writable again, and closing again requires the closure prerequisites to be met again.

---

## A2. Observables

**Amends:** PRD §18 (adds to the operating picture), EDD §13.4.

**PRD text added to §18:**

> The response team can record a small set of typed cyber observables for an incident: IP address, domain, URL, hostname, email address and file hash. Each has a value, a status (suspected, confirmed, benign, blocked), a source and a first-seen time with time confidence. Observables are compartment-scoped like other incident records.

**EDD rules:**

- Record type `incident.observable`, immutable `Record(...)`. A status change is a new version.
- Values are validated syntactically per type, stored as text, rendered as text (never as links), and not sent to model lanes beyond existing sensitivity policy.
- *Related incident* is derived at read time: the same normalised value in another incident is shown only when the viewer may read both incidents and the relevant compartments. Otherwise nothing is shown.
- Shown on the Evidence screen. One projection table.

**Is not:** enrichment, reputation lookup, feeds, IOC correlation engine, graph, bulk import, STIX/MISP or any export format, or automated blocking.

**AC-90 — Minimal observables.** Responders can record and version typed observables within an incident and compartment. Values are never rendered as active links. A same-value observable in another incident is shown only to a viewer authorised for both, and no enrichment or external lookup occurs.

---

## A3. Management summary

**Amends:** PRD §1 (common operating picture), adds to the Command Summary. EDD §15.

**PRD text added to §1:**

> The Command Summary includes a fixed management summary over the incidents the viewer is authorised to see: active incidents, active incidents by severity, overdue critical actions, overdue obligations, and median decision-loop time.

**EDD rules:**

- Fixed metrics, computed by a small fixed number of aggregate queries over existing projections, restricted to the viewer's authorised incidents. Exercise incidents are excluded from live figures.
- *Median decision-loop time* = median of (decision time − proposal time) for confirmed governed records in those incidents over the last 7 days. The definition is shown beside the figure.
- No new screen. No new store.

**Is not:** a report builder, BI engine, chart library, trend history, export, filter set or cross-organisation view. It never counts incidents the viewer cannot see.

**AC-91 — Scoped management summary.** The Command Summary shows the five fixed metrics computed only over incidents the viewer may see, excluding exercises, with the decision-loop definition visible. It reveals no count or attribute of an unauthorised incident.

---

## A4. Machine first-report intake

**Amends:** PRD §15 (interfaces) and §17 (first reports), EDD §9 (credentials) and §16 (interfaces).

**PRD text added to §17:**

> External systems such as security tools or scripts can submit a first report through one small, documented, vendor-neutral API endpoint using an intake key. A machine-submitted first report is an ordinary first report: it never becomes an incident without a human decision.

**EDD rules:**

- Endpoint `POST /api/v1/first-reports`. JSON only, existing 64 KiB request cap, no attachments.
- **Intake key**: a machine credential, not a person.
  - It is governed as `security.intake_key` (`Propose → Decide` by two distinct Custodians).
  - It is bound to one accountable named human; EDD §9's no-independent-principal rule is preserved.
  - It authorises **only** first-report creation.
  - The secret is shown once and stored hashed.
  - It has a mandatory expiry, can be revoked immediately, and can be rotated with a bounded overlap (at most 24 hours).
  - It is never subject to TOTP.
- Each report records the key name and accountable human as its source.
- An optional `external_ref` gives idempotency per key: a repeat returns the original report.
- A bounded per-key rate gives `429` beyond the bound. No queue.

**Is not:** a general API IAM framework, OAuth server, service-account model, vendor connector, polling client or outbound webhook. An intake key cannot read anything.

**AC-92 — Scoped machine intake.** A valid intake key can create first reports and nothing else. Expired, revoked, rotated-out and invalid keys are refused identically. A repeated `external_ref` creates no duplicate. A machine report never becomes an incident without human triage.

---

## Definition of done additions

- AC-89 to AC-92 have executable evidence (test rows T-228 to T-232 in the remediation plan).
- `docs/first-report-api.md` documents the endpoint in one page.
- `codebase-to-edd` drift review confirms that none of the four introduced a new store, scheduler, queue or independent principal.
