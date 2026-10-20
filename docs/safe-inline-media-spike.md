# Slice 7 safe-inline media spike

## Decision

CRAB remains download-first. A small passive allowlist may be presented inline only when server-side byte sniffing identifies a supported passive format and, when the client supplied a media type, the declared and detected base media types agree.

Inline candidates in this checkpoint are JPEG, PNG, GIF, WebP, common WAV/MP3/Ogg audio, MP4 and WebM. HTML, SVG, PDF, office/executable content, unknown content and declared/detected mismatches fail closed to attachment download.

All attachment responses use `X-Content-Type-Options: nosniff` and `Cache-Control: no-store`. Download-only responses use `application/octet-stream`. Filenames are reduced to a basename and control characters are removed before `Content-Disposition` is constructed.

## Why this stays small

The frozen design requires safe presentation, not a media-processing estate. CRAB therefore adds no thumbnailer, transcoder, malware scanner, preview service or object store. Uploaded bytes remain immutable and uninterpreted. Inline presentation is merely a response policy over already-authorised, integrity-verified bytes.

## Proof retained in tests

- active HTML declared as an image is download-only;
- SVG/PDF are download-only;
- declared/detected mismatch is download-only;
- a supported passive image with matching type may be inline;
- filename control characters/path components are stripped;
- integrity verification occurs before bytes are copied to the response writer.

This spike can be narrowed later if browser evidence proves any currently permitted passive media class unsafe. Broadening it requires new evidence; it is not automatic.
