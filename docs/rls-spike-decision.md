# Slice 6 RLS spike decision

**Decision:** do not retain PostgreSQL row-level security in CRAB V0.4.

Application authorisation remains the primary security boundary. The Slice 6 spike tested PostgreSQL RLS only as an independent defence against an accidentally broad application query, exactly as required by the frozen EDD.

## What the spike proved

Using the real non-owner runtime role and the real pgx pool pattern, a disposable RLS table demonstrated that:

- a transaction-local incident and compartment context can restrict a deliberately unscoped `SELECT`;
- GENERAL plus one authorised restricted compartment can be expressed with a small policy;
- a wrong incident or wrong compartment is not returned by PostgreSQL;
- `SET LOCAL`/transaction-local configuration clears on both commit and rollback even when the same pooled connection is reused;
- the ordinary runtime role remains neither table owner, superuser nor `BYPASSRLS`.

So PostgreSQL RLS itself is capable of providing the intended read-side backstop.

## Why it is not retained

The same empirical spike also proved that an unscoped runtime write is denied. That matters because CRAB deliberately uses the same constrained, non-owner runtime database identity for normal application work and deterministic projection writes/rebuilds.

Retaining RLS on the real incident projections would therefore require at least one additional mechanism:

- pin every scoped multi-query read to a transaction and establish transaction-local security context before any repository query; and
- introduce a separate privileged projection/rebuild path, a second database role/pool, or a bypass flag/policy so deterministic projection writes can rebuild all incidents and compartments.

That would add a new privilege channel and recovery distinction solely to support a defence-in-depth layer. It would also make backup/restore and projection rebuild harder to reason about, while duplicating incident/compartment filtering already enforced by the single application authorisation boundary and scope-explicit repositories.

Under the frozen EDD gate, that recovery and privilege ambiguity costs more than the additional protection. RLS is therefore removed rather than partially retained.

## Security consequence

This is not a relaxation of the access model. The retained controls are:

- deny-by-default application authorisation;
- role + incident membership + compartment membership + action/output checks;
- authoritative object-to-scope resolution for direct-object requests;
- scoped Sponsor -> Custodian and Controller -> Custodian access governance;
- query-time expiry/revocation;
- scoped common-operating-picture filtering;
- negative tests for cross-incident, cross-compartment and identifier-substitution access;
- runtime PostgreSQL role remains non-owner, non-superuser and without `BYPASSRLS`.

If the persistence shape later changes so read queries and projection/rebuild writes naturally use separate database identities, RLS may be re-spiked. V0.4 does not introduce that architecture merely to make RLS fit.
