# CRAB — final acceptance progress

## Automated acceptance completed

The final acceptance work is deliberately split into bounded, independently gated components.

1. **Bafflewick end-to-end regression gate — PASS.** Eight mandatory operational phases are executable in CI; threshold is 8/8.
2. **Codebase-to-EDD drift review — PASS.** The implementation remains materially aligned with frozen EDD V0.4; no architecture reopen is required. The stale `architecture.txt` checkpoint text was corrected.
3. **Visual/accessibility automated preflight — PASS once its dedicated CI check is green.** The pass covers responsive/mobile contracts, visible keyboard focus, practical touch targets, no remote core CSS assets, A4/Letter rendering, monochrome/exercise semantics and print authorisation/provenance checks.

## Remaining empirical release evidence

Two items cannot be truthfully manufactured by ordinary CI:

- **Raspberry Pi 5 / 8 GB physical resource run** — execute `scripts/final-acceptance.sh` as documented in `docs/pi-final-acceptance.md` and retain its Markdown evidence file.
- **Human rendered visual review** — inspect the representative browser and printed outputs in `docs/visual-acceptance.md`, record any real defects/fixes, and sign off only after they pass.

No stable release tag should be created until both are complete.

## Release rule

A failure in either remaining gate reopens the failed behaviour only. It is not permission to replace the frozen architecture or restart the product build.
