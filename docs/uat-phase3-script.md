# CRAB — Phase 3 UAT script

**Build:** 0.0.21-dev (release candidate for Phase 3 UAT), branch `uat2/build` → `main`.  
**Purpose:** confirm that every Phase 2 concern has been addressed, and score the human rows T-187 to T-192 and T-211 to T-213.  
**Record results in:** `docs/visual-acceptance.md` (*Phase 3 human UAT*).  
**Time needed:** about 90 minutes.

---

## 0. Before you start

**You need two people, or one person with two browser sessions.** CRAB has exactly the two people created at setup and every governed change needs a second person. Use one normal browser window for person A and a private window (or a second browser) for person B. Section 9 shows how one person can keep working when the other is unavailable.

**Fresh install.** Start from an empty database so setup runs again (setup is where the new password rule and the severity scale are tested).

```text
crab version            # expect 0.0.21-dev
crab setup && crab serve
```

**Reference renders.** Screenshots of every screen at phone, tablet and desktop width in light and dark, plus sample PDFs, are published by CI on the `ci/visual-capture` branch (`shots/` and `pdf/`). They are a comparison aid, not a substitute for your review.

**Throughout:** try the screens in light and dark (operating-system setting), at phone width (narrow the window to about 390 px) and with the keyboard only (Tab, Shift+Tab, Enter, Space).

---

## 1. Setup and password rule — *Phase 2: password needs upper, lower and special symbol*

1. Open CRAB. The sign-in and setup pages show the new CRAB logo.
2. Enter a password of 12+ characters with no capital letter. **Expect:** refused, with a message saying what is missing.
3. Repeat with no lower-case letter, then with no symbol. **Expect:** each refused.
4. Use e.g. `Bafflewick-Alpha-26` for person A and a different one for person B. **Expect:** accepted.
5. The setup form shows the **severity scale** pre-filled *Severity level 1* (least severe) to *Severity level 5* (most severe). Both people confirm it as part of setup.

Tests: T-193, T-202.

## 2. Command Summary — *Phase 2: a summary page users see when they log in and use*

1. Sign in as A. **Expect:** the Command Summary is the home page, with *Needs attention* and *My active incidents*.
2. With nothing happening, it says so plainly rather than showing empty tables.
3. Later in the script (after section 4) come back here as each person. **Expect:** pending approvals, first reports awaiting triage, a stale paper pack and active self-approval windows appear under *Needs attention*, each linking to the item. Nothing about an incident you cannot see is shown or counted.

**T-213 (timed):** after section 5, sign in as B and start a timer. Within 30 seconds B should be able to say what needs attention and open it.

## 3. Storm Blue — *Phase 2: Storm Blue UX overhaul*

Walk every screen: Command Summary, Approvals, First reports, Board, Evidence, Communications, Access, Close-out, Readiness, Organisation, Obligation library, Access admin.

| Check | Expect |
|---|---|
| Palette | Storm Blue for brand and primary actions, Deep Ink text, Slate secondary text, Mist lines, Paper page background with white panels |
| Status colour | Green, amber and red only mean state (OK, warning, danger). Green is no longer the brand colour |
| Text never relies on colour | every badge and status also has words |
| Patterns | each screen reads as one of: summary, operating picture (Board), queue (Approvals, First reports), workspace (Organisation, Access, Readiness) |
| One primary action | each section has at most one obvious primary button |
| Dark mode | readable, same structure, no washed-out text |
| Phone width | one column, nothing cut off, controls easy to tap |

**Known shell limits (recorded, not defects in this build):** the header shows the text wordmark, not the logo, because go-app-shell v0.1.0 has no logo slot; signed-out pages show an empty navigation rail. Both are requested for go-app-shell v0.2.

Tests: T-200, T-211, T-187 to T-190.

## 4. Organisation configuration — *Phase 2: governed severity, emergency configuration and real paper pack, checklist templates*

Open **System → Organisation**.

1. **Severity scale.** A proposes a change (e.g. rename level 5). **Expect:** it waits in Approvals; A cannot confirm it; B confirms it. Declaring an incident and *Propose severity change* on the Board offer only the confirmed levels.
2. **Emergency information.** A fills *Propose emergency information*: contacts, insurer, regulator, legal adviser, escalation route, role cards and a procedure. B confirms.
3. **Checklist templates.** A proposes a template (e.g. *Ransomware: first hour*, 3–4 steps with default roles). B confirms.

Tests: T-202, T-203, T-207.

## 5. Incidents, related incidents and checklists — *Phase 2: related incidents (no access leakage), checklist templates*

1. A declares **INC-A** *Payment outage* at *Severity level 3*; B confirms. Declare **INC-B** *Credential compromise* the same way. Only the declaring person is on an incident at first: on each incident's **Access** page the declaring person uses *Request incident access* with the other person's CRAB ID, and the other person confirms it in Approvals, so both of you are on INC-A and INC-B.
2. On INC-A's Board, *Activate checklist* with the template from section 4. **Expect:** one proposed action per step, owners from the step's default role; Approvals groups them under one heading for B.
3. On INC-A's Board, *Relate or unlink an incident*: relate INC-B with a reason. B confirms. **Expect:** both Boards list the link under *Related incidents*. A link grants no access and merges nothing.
4. **Leakage check:** B declares **INC-C** *Board member phishing* (A confirms the declaration) and does **not** add A to it. **Expect:** INC-A's *Relate* list does not offer INC-C to A; typing its ID is refused with the same message as a made-up ID. B relates INC-A to INC-C. **Expect:** A's Approvals and A's view of INC-A show no trace of INC-C or the reason; the proposal stays pending because nobody else can see both incidents. B's view of INC-A shows it pending.
5. Unlink INC-A and INC-B (propose, confirm). **Expect:** the link disappears from both Boards; it is a new governed version, not a deletion.

Tests: T-205, T-206, T-207, T-191, T-192.

## 6. Time authority — *Phase 2: time authority*

1. **System → Readiness**, *time status*. On a host with NTP synchronised: **expect** pass with the reason.
2. Optional (needs root on a test host): stop time synchronisation (`timedatectl set-ntp false`) and restart CRAB. **Expect:** Readiness warns; new records and PDF footers say **TIME UNVERIFIED**; CRAB keeps working. Re-enable afterwards.

Tests: T-194 to T-196.

## 7. Print — *Phase 2: simpler print preview showing the PDF; print quality*

Print needs a permission. In **System → Access admin → Print and download permissions**, A proposes print and download for B and B proposes it for A; each confirms the other's.

1. On a Board, *Incident Brief → Preview*. **Expect:** the finished PDF opens in a new tab in the browser's own viewer; *Download* saves the same file.
2. **Readiness → Emergency Response Pack → Preview.** **Expect:** your organisation's contacts, insurers, regulators, escalation route, role cards and procedures appear; writable ruled forms for paper operation.
3. Change emergency information in Organisation (A proposes, B confirms). **Expect:** Readiness and the Command Summary say the printed pack is stale until a fresh one is generated.
4. Print the Brief and the Pack on paper in colour and in black-and-white, A4 (and Letter if available). **Expect:** CRAB masthead on every page, clear metadata bar, readable tables, nothing clipped, meaning clear without colour, forms genuinely writable.

Tests: T-197, T-201, T-203, T-204, T-212.

## 8. Approvals comprehension

As B, open Approvals with several items waiting (severity, checklist actions, a relationship, an organisation change). **Expect:** you can tell what each one is, who proposed it, what changes if you confirm, and confirm or reject it without help.

Test: T-192.

## 9. Single-person emergency approval window — *Phase 2: single-person emergency approval window*

Scenario: B will be unreachable overnight; A must keep the response moving.

1. **Organisation → Propose a single-person window**: start now, end 8 hours later (times are UTC), name A by CRAB ID (the form lists the Majors), tick *Severity changes* and *Actions*, give a reason. **Expect:** a window over 24 hours, a start in the past, or naming a non-Major is refused.
2. B confirms the window. **Expect:** the status line shows *Single-person window open*; the Command Summary lists it.
3. As A, propose a severity change on INC-A, then open it in Approvals. **Expect:** a *Single-person window open* notice, a required *Why no second person can decide now* field and *Confirm my own proposal*. Confirm it with a reason.
4. **Expect:** the change is marked **SELF-APPROVAL** on Approvals, the Board, the timeline, the Incident Brief and the Command Summary.
5. As A, propose something not covered (e.g. an obligation clock, or an access change). **Expect:** no self-approve option; it waits for B.
6. In the windows table, *Propose closing it*; the other person confirms. **Expect:** the window shows closed and self-approval is no longer offered.

Never self-approvable, whatever the window says: access, isolate, custodian, organisation configuration, the window itself, closure and retention.

Tests: T-208, T-209, T-210.

---

## 10. Scoring

Record in `docs/visual-acceptance.md` for each row: pass / fail / not run, with a note for every fail.

| Row | Section |
|---|---|
| T-187 phone capture | 3 and First reports at phone width |
| T-188 keyboard and zoom | 3 (keyboard only, 200% zoom) |
| T-189 colour independence | 3 and 7 (dark, monochrome print) |
| T-190 responsive frame | 3 |
| T-191 operating picture comprehension | 5 |
| T-192 approval comprehension | 8 |
| T-211 Storm Blue | 3 |
| T-212 print | 7 |
| T-213 Command Summary in 30 seconds | 2 |

And for each Phase 2 concern, whether it is now resolved:

| Phase 2 concern | Section | Resolved? |
|---|---|---|
| Storm Blue UX overhaul | 3 | |
| Password: upper, lower, special symbol | 1 | |
| Summary / cover page after sign-in | 2 | |
| Simpler print preview showing the PDF | 7 | |
| Time authority | 6 | |
| Organisation emergency configuration and real paper pack | 4, 7 | |
| Related incidents without access leakage | 5 | |
| Single-person emergency approval window | 9 | |
| Governed severity values (Severity level 1–5) | 1, 4 | |
| Governed checklist templates | 4, 5 | |

## 11. Known limits in this build

- **Only the two setup people.** There is no way yet to add further accounts or role eligibility after setup. Routed to the product track (PRD access administration), not part of this response.
- **Header logo and signed-out navigation rail** — go-app-shell v0.1.0 limits, requested for v0.2.
- **Phone header** — at phone width the status text in the header is shortened with an ellipsis (go-app-shell header); the same states are shown in full on the Board and the Command Summary.
- **Time check** reads the host's synchronisation state (Linux `adjtimex`); it does not query time servers itself.
