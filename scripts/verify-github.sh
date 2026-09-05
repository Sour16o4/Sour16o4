#!/usr/bin/env bash
# Post-push verification for the profile README (build spec §9: "verify
# rendering on GitHub itself, not just locally — camo and CSP behave
# differently"). Run this once the repo is actually live; it's a no-op
# against a repo that hasn't been pushed yet.
#
# Usage: scripts/verify-github.sh [owner/repo] [branch]
# Defaults to Sour16o4/Sour16o4 main.
set -uo pipefail

REPO="${1:-Sour16o4/Sour16o4}"
BRANCH="${2:-main}"
PROFILE_URL="https://github.com/${REPO%%/*}"
RAW_BASE="https://raw.githubusercontent.com/${REPO}/${BRANCH}"

PASS=0
FAIL=0
FLAG=0

pass() { echo "  PASS: $1"; PASS=$((PASS+1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL+1)); }
flag() { echo "  FLAG: $1"; FLAG=$((FLAG+1)); }

echo "== 1. Fetch the rendered profile page =="
PAGE_TMP=$(mktemp)
PAGE_CODE=$(curl -sL -o "$PAGE_TMP" -w "%{http_code}" "$PROFILE_URL")
PAGE=$(cat "$PAGE_TMP")
rm -f "$PAGE_TMP"
if [ "$PAGE_CODE" != "200" ] || [ -z "$PAGE" ]; then
  fail "HTTP $PAGE_CODE fetching $PROFILE_URL — is the repo actually pushed and public?"
  echo "Aborting — nothing else can be checked without the live page."
  exit 1
fi

echo
echo "== 2. Where do image srcs resolve? =="
# Every <img> and <source srcset> inside the rendered README body.
IMG_SRCS=$(echo "$PAGE" | grep -oE '(src|srcset)="[^"]*"' | grep -oE 'https://[^"]*\.svg' | sort -u)
if [ -z "$IMG_SRCS" ]; then
  fail "no .svg image URLs found in the rendered page at all — README may not be live, or GitHub restructured the markup this script greps for"
else
  echo "$IMG_SRCS"
  RAW_COUNT=$(echo "$IMG_SRCS" | grep -c 'raw.githubusercontent.com' || true)
  CAMO_COUNT=$(echo "$IMG_SRCS" | grep -c 'camo.githubusercontent.com' || true)
  if [ "$CAMO_COUNT" -gt 0 ]; then
    flag "$CAMO_COUNT URL(s) went through camo.githubusercontent.com — this changes the animation approach (camo may re-encode/flatten SVGs). Inspect which ones and why; same-repo relative paths shouldn't need it."
  fi
  if [ "$RAW_COUNT" -gt 0 ]; then
    pass "$RAW_COUNT URL(s) resolve to raw.githubusercontent.com as expected"
  fi
fi

echo
echo "== 3. Byte-diff each served SVG against the local file =="
for f in assets/*.svg; do
  base=$(basename "$f")
  [[ "$base" == _compare-* ]] && continue
  url="${RAW_BASE}/assets/${base}"
  tmp=$(mktemp)
  code=$(curl -sL -o "$tmp" -w "%{http_code}" "$url")
  served=$(cat "$tmp")
  rm -f "$tmp"
  if [ "$code" != "200" ]; then
    fail "$base: HTTP $code fetching $url — not pushed, or pushed to a different branch/path"
    continue
  fi
  local_hash=$(sha256sum "$f" | cut -d' ' -f1)
  served_hash=$(echo -n "$served" | sha256sum | cut -d' ' -f1)
  if [ "$local_hash" = "$served_hash" ]; then
    pass "$base: byte-identical to local"
  else
    fail "$base: DIFFERS from local — diff below"
    diff <(cat "$f") <(echo "$served") | head -20
  fi

  echo
  echo "  -- $base: content checks --"
  if echo "$served" | grep -q '<style'; then
    pass "$base: <style> block present"
  else
    fail "$base: <style> block MISSING — stripped by GitHub or camo"
  fi
  if echo "$served" | grep -q '@keyframes'; then
    pass "$base: @keyframes present"
  else
    if echo "$served" | grep -q 'animation'; then
      flag "$base: has 'animation' but no '@keyframes' string found — check for renaming/minification"
    fi
  fi
  if echo "$served" | grep -qE '<animate[ >]'; then
    flag "$base: contains SMIL <animate> — spec says CSS-only; verify this is intentional (older sections?)"
  fi
done

echo
echo "== 4. <picture> / theme-switch wiring, from the served README =="
README_RAW=$(curl -sL "${RAW_BASE}/README.md")
PICTURE_COUNT=$(echo "$README_RAW" | grep -c '<picture>' || true)
SOURCE_COUNT=$(echo "$README_RAW" | grep -c 'prefers-color-scheme: dark' || true)
IMG_COUNT=$(echo "$README_RAW" | grep -cE '<img alt=' || true)
echo "  <picture> blocks: $PICTURE_COUNT"
echo "  dark <source media> entries: $SOURCE_COUNT"
echo "  <img alt=...> fallbacks: $IMG_COUNT"
if [ "$PICTURE_COUNT" -eq "$SOURCE_COUNT" ] && [ "$SOURCE_COUNT" -eq "$IMG_COUNT" ] && [ "$PICTURE_COUNT" -gt 0 ]; then
  pass "every <picture> block has one dark source and one light img fallback"
else
  fail "counts don't line up (picture=$PICTURE_COUNT source=$SOURCE_COUNT img=$IMG_COUNT) — a section's theme switch may be malformed"
fi

# Confirm the switch actually renders differently per theme in the live page.
# (This only checks the markup is wired; it can't toggle OS theme itself.)
if echo "$PAGE" | grep -q 'prefers-color-scheme'; then
  pass "the live page's DOM still carries prefers-color-scheme media, i.e. GitHub didn't flatten <picture> into a single <img>"
else
  flag "no prefers-color-scheme found in the rendered page — GitHub may have flattened <picture> down to one <img>; check which source it kept"
fi

echo
echo "== Summary =========================================="
echo "PASS: $PASS   FAIL: $FAIL   FLAG: $FLAG"
if [ "$FAIL" -gt 0 ] || [ "$FLAG" -gt 0 ]; then
  echo "Do not treat the animation approach as confirmed until FAIL/FLAG are resolved."
fi
