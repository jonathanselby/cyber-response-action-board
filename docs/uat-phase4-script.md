# CRAB — Phase 4 UAT script

**Build:** 0.0.34-dev release candidate (post-UAT remediation C0–C13 complete), on `main`.
**Purpose:** human acceptance of the remediated product. The engineering is proven by the final automated gate (`scripts/final-gate.sh`, `final-gate` workflow). This UAT is about the experience: can real people run an incident with it?
**Record results in:** `docs/visual-acceptance.md` (*Phase 4 human UAT*), one row per section below.
**Time needed:** about 90 minutes.

> **Prove the engineering so the human UAT can be about the experience.**

---

## 0. Before you start

**Two browser sessions.** Person A in a normal window and person B in a private window or a second browser. From section 2 you also add people, so most governed changes have a natural second person.

**Fresh install, no helper machinery.**

```text
crab version            # expect 0.0.34-dev
crab setup && crab serve
```

Open the address `crab serve` prints. For a phone on the same network, `hostname -I` gives the laptop's address.

**Optional: notifications.** To try section 7 with real delivery, give `crab serve` a notification deployment file (`docs/notifications.md`). A file sink and a test webhook are enough. An SMS or email channel lets you receive notices on your own phone.

**Throughout:** try light and dark, phone width (about 390 px), and the keyboard only (Tab, Shift+Tab, Enter, Space). Record exactly which browsers and devices you used. Nothing is claimed beyond what you test: for example, *Safari/iOS — not tested; no certification claimed*.

**Findings:** each finding is either **fix now** (broken behaviour, security, accessibility, layout, visible inconsistency, confusing interaction, real device defect) or **do not add** (new features, redesign). Fixes go through the chunk loop at the smallest size.

---

## 1. Setup and sign-in

1. Run setup with two people (A and B). Passwords need upper and lower case and a symbol.
2. Sign in as A. **Expect:** the Command Summary, with *Sign out* and *Your account* in the navigation.
3. Sign out from the *Sign out* page. **Expect:** signed out; the back button does not bring the session back.

Findings covered: R-20, F-13.

## 2. People — add, external, temporary (R-01 to R-03)

1. As A, open **Access admin → People** and add *Carol* (Minor, internal) and *Jane* (Minor, **external**, Forensic Co, account ends in 7 days).
2. As B, confirm both in **Approvals**. **Expect:** each card names the person, role, EXTERNAL marking and account end in plain words, with *If you confirm* and *If you reject*.
3. Issue each an enrolment code (A proposes; B confirms and sees the code once).
4. In a third browser session, enrol Carol and Jane. **Expect:** Jane must set up an authenticator app; Carol may.
5. Suspend Carol, then reactivate her. **Expect:** while suspended she cannot sign in, and an open session stops on its next request.

Findings covered: R-01, R-02, R-03, F-1, F-9, U-17.

## 3. Declare and run an incident (R-04, R-21, R-22)

1. As A, declare a **live** incident. B confirms it.
2. Add Carol and Jane as members. Give Jane access that **expires in 3 hours**.
3. As A, record facts, unknowns, a decision and actions, one owned by Carol. Assign an operational role.
4. Open the **Board**. **Expect:** a *Now* block of at most about seven items at the top, and each section showing the most urgent five first with *All N…* below.
5. As B, open Approvals and read each card before deciding. **Expect:** you can tell what will happen on confirm and on reject without opening anything else.
6. Hand over, as if at a shift change.

Findings covered: R-04, R-21, R-22.

## 4. Restricted compartment and what others can see

1. Create a restricted compartment (for example FORENSICS). Read in A and Carol only.
2. Record an indicator and a fact in FORENSICS.
3. As Jane (not read in), look at the Board, Evidence, Approvals and Communications. **Expect:** nothing from FORENSICS, not even counts or names that give it away.

Findings covered: R-07, F-9, the access abuse suite (R-16).

## 5. Evidence and indicators (R-07)

1. Upload a file as evidence; view it on its own page.
2. Record indicators of several types (IP, domain, hash) and change one's status. **Expect:** values shown as plain text, never as links; the history is kept.

## 6. First reports and machine intake (R-06)

1. Record a first report by hand from **First reports** (phone width too).
2. Issue a machine intake key (A proposes, B confirms and sees the secret once). Submit one report with `curl` as in `docs/first-report-api.md`, then submit the same one again. **Expect:** one report, attributed to the key; the repeat creates nothing.

## 7. Communications and notifications (R-05)

1. Draft a communication, propose it, and have B confirm it.
2. Grant A **Send issued communications** (Access admin; B confirms). Send it. **Expect:** a delivery table with each channel's outcome in words. If a channel fails, it says *Failed — not retried automatically*, the Board shows it, and *Resend to failed* appears.
3. On **Your account**, set your notification route (if a per-person channel is configured). **Expect:** only a masked hint is shown afterwards.
4. Make Carol owner of a new action, propose something B must decide, and wait up to a minute. **Expect:** short notices that say only what kind of item it is and the incident ID, with no titles. Someone without a route appears as *no route* under **Notices sent**.

## 8. Obligations, time and closure (R-09, F-12)

1. Look at an obligation clock on the Board.
2. Try to close the incident with an overdue active clock. **Expect:** closure is blocked and the reason is named. Waiving the clock (with a reason) lets closure go ahead.
3. Close the incident, then reopen it (governed). **Expect:** the history is intact and the reopening is recorded.

## 9. Management summary and Command Summary (R-08)

1. As A, open the Command Summary. **Expect:** within 30 seconds you can say what needs you, which incidents are active and their severity, and what the management summary figures mean (definitions shown).

## 10. Print and paper

1. Print the Incident Brief and the Emergency Response Pack (A4 or Letter; colour and monochrome).
2. **Expect:** no clipping, useful page breaks, visible source, version and time markings, and blank forms that can genuinely be written on.

## 11. Devices and accessibility (R-17, R-18, R-20)

Repeat a short path (sign in, Board, Approvals, record a fact) on:

- the primary desktop browser;
- a second desktop browser;
- a real Android phone;
- tablet and phone widths;
- keyboard only, dark mode and 200% zoom.

**Expect:** no sideways scrolling, a visible focus outline, no cut-off header text, and status never shown by colour alone.

## 12. Scoring

Record one row per section in `docs/visual-acceptance.md`, *Phase 4 human UAT*:

| Section | Result | Findings |
|---|---|---|
| 1 Setup and sign-in | pass / fail | |
| 2 People | | |
| 3 Incident | | |
| 4 Compartment | | |
| 5 Evidence and indicators | | |
| 6 First reports | | |
| 7 Communications and notifications | | |
| 8 Obligations and closure | | |
| 9 Command Summary | | |
| 10 Print | | |
| 11 Devices (list what was tested) | | |

When every row passes, the build is closed (`docs/remediation-build-plan.md` §8): the version is promoted, release evidence is refreshed, the final CI runs, and the release is tagged.

## 13. Known limits in this build

- **Header logo and signed-out navigation:** they follow go-app-shell v0.1.0 (U-14); v0.2 has not been supplied.
- **Wrong but synchronised clock:** a clock that is wrong yet synchronised cannot be detected (R-19); time trust is shown on every page.
- **Raspberry Pi measurements:** the Pi run is handed over as `scripts/final-acceptance.sh`; no Pi figures are claimed.
- **Escalation notice wording:** it currently reads *"an incident you are on on INC-7"*. Note it if it reads badly.
- **Notification settings:** escalation notices, their shared destinations and the decision window are set in the notification deployment file, not in the app.
