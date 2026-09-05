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
