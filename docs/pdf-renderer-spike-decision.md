# CRAB PDF renderer spike decision

## Decision

Use `github.com/signintech/gopdf` for CRAB V0.4 print rendering.

The Slice 11 spike is accepted because the implemented renderer satisfies the frozen print boundary without adding a browser, document service, external font dependency or second application runtime.

## What was proved

- PDF generation is local and pure Go.
- Linux amd64 and arm64 remain CI build gates.
- The Go Regular TTF is embedded from `golang.org/x/image/font/gofont/goregular`; no font file is fetched or mounted at runtime.
- A4 and US Letter use explicit page dimensions.
- Colour and monochrome are renderer modes; headings and exercise status remain textual, so meaning does not depend on colour.
- Text wrapping is width-measured and long unbroken identifiers are split before rendering rather than clipped.
- Page breaks are controlled locally and each generated page receives source/footer metadata and a page number.
- Exercise output is visibly labelled on every page, including continuation pages.
- Fixed print inputs produce deterministic PDF bytes in the renderer acceptance test.
- The renderer consumes composed document models only; it contains no SQL, authorisation or incident-policy logic.
- No network access, headless browser, Chromium, HTML-to-PDF service, object store or print daemon is introduced.

## Source marking

Generated operational PDFs carry:

- source ledger sequence;
- source ledger hash prefix;
- configuration version;
- generation time;
- time-trust status;
- page number.

The application layer obtains the source mark from CRAB. Caller-supplied source provenance is not trusted for the Emergency Response Pack.

## Derived-copy behaviour

Generated PDFs are derived copies, not authoritative state. The authoritative record remains the CRAB ledger and durable supporting data.

The filesystem edge writes a same-directory temporary file, flushes it, and atomically renames it over the current derived copy. A failed primary refresh leaves the previous primary file untouched. An optional secondary copy is refreshed independently after primary success; secondary failure is reported while the last-good secondary remains intact.

Already-generated files are ordinary local PDFs and remain readable when PostgreSQL is unavailable.

## Rejected alternatives

A headless-browser or HTML-to-PDF path was rejected because it adds a large rendering/runtime dependency, complicates constrained-device deployment and creates another service boundary for a capability that can be rendered directly.

External font/assets were rejected because the paper path is specifically required to remain useful when normal infrastructure is unavailable.

A separate print database, queue or service was rejected because print is a derived-output concern and earns none of those mechanisms.

## Gate

If later Bafflewick/Pi visual acceptance shows gopdf cannot meet the frozen readability/print-quality bar, replace the renderer. Do not weaken the print requirement to preserve the library choice.
