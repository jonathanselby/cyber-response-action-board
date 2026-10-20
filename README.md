# CRAB — Cyber Response Action Board

CRAB is a single Go application for running a cyber incident from first report through active response, recovery, review and closure while remaining useful when normal corporate systems are unavailable.

> **The model proposes. One person makes. Another confirms. Nothing is erased.**

## Build status

**0.0.3-dev — Slice 2 security boundary.**

Implemented and CI-proven now:

- one `crab` executable with amd64/arm64 build gates;
- file-first deployment configuration and embedded offline web shell;
- PostgreSQL as the single durable live store with separate schema-owner/runtime identities;
- authoritative signed/hash-chained ledger, atomic projections, full verification, integrity-failed read-only mode, deterministic rebuild and portable JSONL export;
- local password authentication with a 600,000-iteration PBKDF2 floor;
- TOTP with deployment-key AES-GCM wrapping of secrets;
- P-256 passkey verification including RP/origin/user-presence/user-verification/counter checks;
- pinned-key RS256 OIDC verification, with isolate mode refusing federation before verifier work;
- short-lived opaque sessions with only SHA-256 token digests stored;
- exactly-two-keyholder break-glass authentication with individually revocable verification material;
- bounded authentication failure limiting and pre-registered-route out-of-band verification;
- one deny-by-default authorisation evaluator for role, incident, compartment, authoritative object scope and separate data-exit permissions;
- ledger-rebuildable Major/Minor/Observer eligibility, Custodian designation, incident membership, restricted compartment membership, output permissions and isolate mode;
- `GENERAL` derived from governed incident membership rather than independently grantable;
- agent operations bound to one named human and unable to exceed that human's authority;
- two-human bootstrap and Custodian-recovery ceremony rules with no automatic incident visibility;
- human-signable ceremony records suitable for the same authoritative ledger path;
- break-glass audit plus alert-edge attempt recording, where alert failure does not cancel valid emergency authentication;
- frozen PRD V0.2 / EDD V0.4 and companion build, test and UX/print notes retained beside source.

Not yet claimed complete:

- secure operator/presentation wiring for `crab setup` and browser login flows; the command continues to refuse rather than expose secrets through an improvised control path;
- Slice 3 incident/domain semantic write kernel and first common operating picture;
- RLS spike, evidence blobs, retrieval, model assistance, communications/notifier implementation, print, backup/restore and later slices.

The checkpoint does not create a universal administrator, universal Major visibility, emergency read-in, per-object ACL estate, alternate store, workflow engine or placeholder incident mutation path.

## Commands

```text
crab version
crab readiness
crab serve
crab verify --ledger <file> --keyring <file>
crab rebuild
crab export-ledger --output <file>
```

`crab verify` is the offline portable verifier. Database-backed commands require `CRAB_DATABASE_URL` and `CRAB_LEDGER_KEYRING_FILE`. `crab serve` remains loopback-only until the authenticated presentation path is wired. `crab setup` continues to fail closed until a secure operator input path is implemented; passwords, recovery proofs and signing private material are not accepted through ordinary command arguments.

## Security boundary

CRAB's application decision remains deliberately small:

```text
ALLOW = authenticated principal
    AND role permits action
    AND active incident membership where required
    AND required compartment membership
    AND object resolves to that authorised scope
    AND requested output/action is separately permitted

otherwise DENY
```

Role determines what a person may do. Incident membership determines where. Compartment membership determines what information they may know. Custodian authority administers identity/access metadata but grants no incident-content visibility by itself.

Break-glass changes authentication only. It does not add a role, incident, compartment or output permission.

## Database boundary

The schema-owner identity performs migrations. The ordinary runtime identity is not table owner, superuser or `BYPASSRLS`. Authoritative ledger rows cannot be updated/deleted by the runtime role. Credential/session state is durable supporting state; access and isolate current state are rebuildable projections from the ledger.

## Governing documents

- `PRD.md` — product contract, V0.2.
- `EDD.md` — frozen architecture, V0.4.
- `BUILD-NOTES.md` — implementation sequencing and build discipline.
- `TEST-ACCEPTANCE-NOTES.md` — behavioural/security acceptance catalogue.
- `UX-PRINT-BUILD-NOTES.md` — web, accessibility and print quality requirements.

The implementation may simplify code. It may not silently weaken the product or security contract.

## Toolchain

CI is pinned to Go 1.27.2. The module baseline is Go 1.25 because the pinned PostgreSQL driver requires Go 1.25 or newer.
