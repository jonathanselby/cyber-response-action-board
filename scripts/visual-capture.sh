#!/usr/bin/env bash
# Renders every CRAB screen with seeded Bafflewick data and screenshots it at
# phone, tablet and desktop widths in light and dark (T-211 review aid).
# Requires CRAB_TEST_POSTGRES_ADMIN_URL and a Chrome/Chromium binary.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"
OUT=${1:-visual-capture}
CHROME=${CHROME:-$(command -v google-chrome || command -v chromium || command -v chromium-browser || true)}
if [ -z "$CHROME" ]; then
  echo "Chrome or Chromium is required" >&2
  exit 1
fi
rm -rf "$OUT"
mkdir -p "$OUT/shots"
OUT=$(cd "$OUT" && pwd)
CRAB_VISUAL_CAPTURE_DIR="$OUT/site" go test -count=1 -run '^TestVisualCapture$' ./cmd/crab
cp -r "$OUT/site/pdf" "$OUT/pdf"

PORT=8765
python3 -m http.server "$PORT" --bind 127.0.0.1 --directory "$OUT/site" >/dev/null 2>&1 &
SERVER=$!
trap 'kill $SERVER 2>/dev/null || true' EXIT
sleep 1

shoot() { # name width height scheme
  local flags=(--headless=new --disable-gpu --no-sandbox --hide-scrollbars --window-size="$2,$3" --screenshot="$OUT/shots/$1-$2-$4.png")
  if [ "$4" = dark ]; then flags+=(--force-dark-mode --blink-settings=preferredColorScheme=0); else flags+=(--blink-settings=preferredColorScheme=1); fi
  "$CHROME" "${flags[@]}" "http://127.0.0.1:$PORT/pages/$1.html" >/dev/null 2>&1 || true
}

for page in "$OUT"/site/pages/*.html; do
  name=$(basename "$page" .html)
  shoot "$name" 1366 1600 light
  shoot "$name" 1366 1600 dark
  shoot "$name" 390 1600 light
  shoot "$name" 768 1400 light
done
ls "$OUT/shots" | wc -l
echo "Screenshots in $OUT/shots; PDFs in $OUT/pdf"
