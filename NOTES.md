# Build notes (addenda to the original spec)

Lessons learned while shipping this that the original build spec didn't
cover — read before adding a new section.

## Never animate an element that exists only inside `<clipPath>` or `<defs>`

An element defined only inside `<clipPath>` (or only referenced via `<use>`
from `<defs>` in some engines) is not part of the normal paint tree — it's
geometry, not a rendered thing. CSS animations applied to it can update its
*initial* computed style correctly (so a static/first-paint screenshot looks
right) but then never advance again once the page is actually running,
because it never joins the browser's ongoing animation tick.

This shipped broken twice before being caught, both times because local
testing only checked the static/reduced-motion frame and never re-verified
*dynamic* playback afterward:

- **Arch traffic particles** — originally animated via `<animate>` inside a
  `<clipPath>`-adjacent structure. Fixed by moving to a directly-rendered
  `<g>` animated with CSS `transform`.
- **Header typing line** — animated `width` on a `<rect>` that existed only
  inside a `<clipPath>`, referenced via `clip-path: url(#id)`. It revealed
  the first couple of characters on first paint, then froze — confirmed on
  the live page and reproduced in isolation (a real-time interval test held
  `getComputedStyle(rect).width` at `0px` across 3.2 seconds of sampling,
  never updating). Fixed by animating `clip-path: inset(0 Rpx 0 0)` directly
  on the (normally rendered) wrapping `<g>` instead — confirmed frame-by-frame
  that this actually progresses.

**Rule:** if something needs to move, it must be a directly rendered element
animated via a normal CSS property (`transform`, `opacity`, `clip-path`,
`fill`, `stroke`, geometry attributes on a real shape). Never point an
animation at something that only exists to be referenced by `url(#id)`.

**Testing corollary:** checking the reduced-motion static frame is not
sufficient to confirm an animation works. Verify actual progression over
real time — screenshots at increasing `--virtual-time-budget` values (or,
more reliably, a `setInterval`-based sample loop reading computed style or
comparing rendered pixels) — before calling a section done. `getComputedStyle`
readouts for SVG-specific properties (`width` on geometry, `clip-path`) have
also been seen lying about the true animated state in headless Chromium;
when in doubt, compare rendered pixels, not the computed style string.

## Never use `animation-fill-mode: both` (or `backwards`) on a one-shot entrance animation

This is a second, distinct bug class from the clip-path one above — it hit
every section that had an "appear on load" entrance (header, now, shipped,
commits, contributions' cell wave, chips), and it's specifically about how
GitHub serves these SVGs: as `<img src="...">`, not as a top-level document.

`fill-mode: both` (or `backwards`) means the element renders as if the
animation's *first keyframe* is already applied during any delay before the
animation starts — that's the intended behaviour, so a delayed fade-in
doesn't pop from fully-visible to hidden right as it begins. The bug: if the
animation then never reliably progresses in the `<img>`-embedding context —
confirmed by isolating it down to a minimal case (a single `<rect>`,
`animation: fadeIn .6s ease .3s both`, embedded via `<img>`) that stayed
invisible across 100ms, 800ms, and 2000ms samples — the element is stuck
showing that hidden first-keyframe state *forever*, not just during the
0.3s delay. This is what "empty dark bands, no text" actually was: whole
cards, the header's avatar/name/subtitle, and heatmap cells rendering as
fully transparent, not empty rects.

Critically, this does **not** affect infinite/looping animations with no
delay-before-visible (pipeline packet, arch traffic, ticker scroll, the
typing line's `clip-path` cycle): their base CSS state and their 0% keyframe
are the same *visible* appearance, so even if progression is unreliable in
this embedding context, they're stuck-but-visible, not stuck-invisible. The
failure mode is specific to "starts hidden, animates to visible."

**Rule:** for any one-shot "reveal on load" animation, use
`animation-fill-mode: forwards`, never `both` or `backwards`. `forwards`
only affects the state *after* the animation completes (identical to
`both` there) — it does not force the hidden keyframe onto the element
before/if the animation actually runs, so the fallback is your own visible
base CSS rule instead of the keyframe's hidden one. Confirmed via the same
minimal isolated case: swapping `both` → `forwards` made the element fully
visible at 100ms, before the delay would even have elapsed.

**Testing corollary, extended:** navigating directly to a `.svg` file
(`file:///path/to/file.svg`) is not the same test as embedding it via
`<img src="...">` — GitHub only ever does the latter. Every verification in
this file before this entry was done by direct navigation and missed this
bug entirely, even though the clip-path fixes above were re-tested
carefully. Test through a real `<img>` embedding, not just direct navigation.
