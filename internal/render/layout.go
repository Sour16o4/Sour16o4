// Package render's shared layout primitives — every section derives its own
// geometry from these rather than hardcoding an absolute position, so
// removing or resizing one section never requires touching another's numbers.
package render

const (
	contentW = 890.0
	// contentPad is the shared left/right content inset for every section —
	// the background rect always stays full-bleed (x=0, width=contentW);
	// this only insets what's drawn on top of it.
	contentPad = 30.0
	topPad     = 40.0
	botPad     = 42.0
)

func usableW() float64 { return contentW - 2*contentPad }
