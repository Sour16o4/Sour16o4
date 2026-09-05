#!/usr/bin/env bash
# Post-push verification for the profile README (build spec §9: "verify
# rendering on GitHub itself, not just locally — camo and CSP behave
# differently"). Run this once the repo is actually live; it's a no-op
# against a repo that hasn't been pushed yet.
#
# Usage: scripts/verify-github.sh [owner/repo] [branch]
# Defaults to Sour16o4/Sour16o4 main.
#
# Every fetched byte stays in a file, never a shell variable: $(...) strips
# trailing newlines, which silently corrupted the byte-diff in the first
# version of this script (it always reported DIFFERS, even against
# byte-identical content, then printed no diff because `echo` added a
# newline back for the comparison but not for the hash).
set -uo pipefail

REPO="${1:-Sour16o4/Sour16o4}"
BRANCH="${2:-main}"
OWNER="${REPO%%/*}"
PROFILE_URL="https://github.com/${OWNER}"
RAW_BASE="https://raw.githubusercontent.com/${REPO}/${BRANCH}"

WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT

PASS=0
FAIL=0
FLAG=0

pass() { echo "  PASS: $1"; PASS=$((PASS+1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL+1)); }
flag() { echo "  FLAG: $1"; FLAG=$((FLAG+1)); }

# fetch URL OUTFILE -> prints the HTTP status code, writes the body to OUTFILE.
fetch() {
  curl -sL -o "$2" -w "%{http_code}" "$1"
}

echo "== 1. Fetch the rendered profile page =="
PAGE="$WORKDIR/page.html"
PAGE_CODE=$(fetch "$PROFILE_URL" "$PAGE")
if [ "$PAGE_CODE" != "200" ] || [ ! -s "$PAGE" ]; then
  fail "HTTP $PAGE_CODE fetching $PROFILE_URL — is the repo actually pushed and public?"
  echo "Aborting — nothing else can be checked without the live page."
  exit 1
fi

echo
echo "== 2. Where do image srcs resolve? =="
# GitHub emits root-relative URLs for same-repo images
# (/OWNER/REPO/raw/BRANCH/path), not absolute raw.githubusercontent.com ones
# — the first version of this script only matched "https://...", which is
# why it found nothing. Match both src= and srcset=, both relative and
# absolute.
grep -oE '(src|srcset)="[^"]*\.svg"' "$PAGE" \
  | sed -E 's/^(src|srcset)="//; s/"$//' \
  | sort -u > "$WORKDIR/img_srcs.txt"

if [ ! -s "$WORKDIR/img_srcs.txt" ]; then
  fail "no .svg image URLs found in the rendered page at all — README may not be live, or GitHub restructured the markup this script greps for"
else
  cat "$WORKDIR/img_srcs.txt"
  FIRST_PARTY=$(grep -cE "^(https://raw\.githubusercontent\.com/|/${OWNER}/)" "$WORKDIR/img_srcs.txt" || true)
  CAMO=$(grep -c 'camo.githubusercontent.com' "$WORKDIR/img_srcs.txt" || true)
  OTHER=$(( $(wc -l < "$WORKDIR/img_srcs.txt") - FIRST_PARTY - CAMO ))

  if [ "$CAMO" -gt 0 ]; then
    flag "$CAMO URL(s) went through camo.githubusercontent.com — this changes the animation approach (camo may re-encode/flatten SVGs). Same-repo relative paths shouldn't need it."
  fi
  if [ "$OTHER" -gt 0 ]; then
    flag "$OTHER URL(s) matched neither the expected first-party pattern nor camo — inspect manually"
  fi
  if [ "$FIRST_PARTY" -gt 0 ]; then
    pass "$FIRST_PARTY URL(s) resolve as first-party (same-repo raw content), as expected"
  fi
fi

echo
echo "== 3. Byte-diff each served SVG against the local file =="
for f in assets/*.svg; do
  base=$(basename "$f")
  [[ "$base" == _compare-* ]] && continue
  url="${RAW_BASE}/assets/${base}"
  served="$WORKDIR/${base}"
  code=$(fetch "$url" "$served")
  if [ "$code" != "200" ]; then
    fail "$base: HTTP $code fetching $url — not pushed, or pushed to a different branch/path"
    continue
  fi

  if cmp -s "$f" "$served"; then
    pass "$base: byte-identical to local ($(wc -c < "$f") bytes)"
  else
    fail "$base: DIFFERS from local — diff below"
    diff "$f" "$served" | head -20
  fi

  echo "  -- $base: content checks --"
  if grep -q '<style' "$served"; then
    pass "$base: <style> block present"
  else
    fail "$base: <style> block MISSING — stripped by GitHub or camo"
  fi
  if grep -q '@keyframes' "$served"; then
    pass "$base: @keyframes present"
  elif grep -q 'animation' "$served"; then
    flag "$base: has 'animation' but no '@keyframes' string found — check for renaming/minification"
  fi
  if grep -qE '<animate[ >]' "$served"; then
    flag "$base: contains SMIL <animate> — spec says CSS-only; verify this is intentional"
  fi
  echo
done

echo "== 4. <picture> / theme-switch wiring, from the served README =="
README_RAW="$WORKDIR/README.md"
code=$(fetch "${RAW_BASE}/README.md" "$README_RAW")
if [ "$code" != "200" ]; then
  fail "README.md: HTTP $code fetching ${RAW_BASE}/README.md"
else
  PICTURE_COUNT=$(grep -c '<picture>' "$README_RAW" || true)
  SOURCE_COUNT=$(grep -c 'prefers-color-scheme: dark' "$README_RAW" || true)
  IMG_COUNT=$(grep -cE '<img alt=' "$README_RAW" || true)
  echo "  <picture> blocks: $PICTURE_COUNT"
  echo "  dark <source media> entries: $SOURCE_COUNT"
  echo "  <img alt=...> fallbacks: $IMG_COUNT"
  if [ "$PICTURE_COUNT" -eq "$SOURCE_COUNT" ] && [ "$SOURCE_COUNT" -eq "$IMG_COUNT" ] && [ "$PICTURE_COUNT" -gt 0 ]; then
    pass "every <picture> block has one dark source and one light img fallback"
  else
    fail "counts don't line up (picture=$PICTURE_COUNT source=$SOURCE_COUNT img=$IMG_COUNT) — a section's theme switch may be malformed"
  fi
fi

# Confirm the switch actually renders differently per theme in the live page.
# (This only checks the markup is wired; it can't toggle OS theme itself.)
if grep -q 'prefers-color-scheme' "$PAGE"; then
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
