// Package theme holds the colour and type tokens from the build spec (§2, §3).
// One accent hue, no exceptions — never add a second colour here.
package theme

// Tokens holds one full palette (light or dark variant).
type Tokens struct {
	Ground string // page/SVG background
	Card   string // card and panel fill
	CardHi string // card fill, hover/active state
	Bone   string // card borders, structural rules
	Text   string // headings, names
	Body   string // commit messages, body copy
	Mute   string // labels, metadata, timestamps
	Dim    string // least important metadata
	Line   string // section rules, chip borders
	Rail   string // pipeline rails and arrowheads
	Accent string // shadows, fills, packet, bars
	Accent2 string // accent for text — links, hashes, values
	Accent3 string // accent at low emphasis — active track borders
}

// Dark is the primary palette: emerald on near-black (§2).
var Dark = Tokens{
	Ground:  "#0c0c0e",
	Card:    "#141417",
	CardHi:  "#181820",
	Bone:    "#e4e0d8",
	Text:    "#f0ede6",
	Body:    "#cdc9c1",
	Mute:    "#8f8a80",
	Dim:     "#6d6961",
	Line:    "#26262b",
	Rail:    "#33333b",
	Accent:  "#158f65",
	Accent2: "#5cd3a8",
	Accent3: "#164d3c",
}

// Light is derived from the same structure, accent darkened to clear 4.5:1
// on a light ground (§9) — never invert Dark naively.
var Light = Tokens{
	Ground:  "#faf9f6",
	Card:    "#f1f0eb",
	CardHi:  "#e9e7e0",
	Bone:    "#2a2a28",
	Text:    "#141412",
	Body:    "#3a3a36",
	Mute:    "#6b675f",
	Dim:     "#8f8b82",
	Line:    "#d8d5cc",
	Rail:    "#b8b4a9",
	Accent:  "#0f6b4a", // darkened from #158f65 to clear 4.5:1 on #faf9f6
	Accent2: "#0d7a52",
	Accent3: "#cfe8dd",
}

// HeatmapRamp is the five-step contribution heatmap ramp (§2), dark variant.
var HeatmapRampDark = [5]string{"#17171b", "#163d33", "#165a44", "#157655", "#158f65"}

// HeatmapRampLight is the light-mode equivalent, same hue progression.
var HeatmapRampLight = [5]string{"#eceae3", "#bfe0d2", "#8fcdb2", "#4fae87", "#0f6b4a"}

// Type holds the two type families and the rule for which content uses which.
type Type struct {
	Sans string // Space Grotesk — anything a human wrote
	Mono string // JetBrains Mono — anything a machine produced
}

// Fonts is the fallback stack per §3.6 (webfonts don't load in a
// GitHub-served SVG): generic stacks, accept metric drift.
var Fonts = Type{
	Sans: "system-ui, -apple-system, 'Segoe UI', sans-serif",
	Mono: "ui-monospace, 'SF Mono', Menlo, Consolas, monospace",
}

// Ease is the one easing curve governing the page (§6): fast attack, long decay.
const Ease = "cubic-bezier(.32,.72,0,1)"

// PacketEase is the exception for packet travel (§6.1): accelerate out,
// decelerate in. Expressed as SMIL keySplines for travel segments.
const PacketEaseSpline = "0.5 0 0.5 1"
