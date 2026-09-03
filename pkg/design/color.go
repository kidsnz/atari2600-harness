// Package design "absorbs" the design rules of design-principles.md into executable
// feasibility verdicts. It turns the prose craft/budget rules into numeric checks and
// serves as the basis for **Claude's design decisions before writing asm** (also reusable
// for the frozen TIA Studio's budget feasibility).
//
// Source: docs/design-principles.md / mining notes reference/atariage/*/notes.ja.md.
// Hardware constants (1 CPU cycle = 3 color clocks = 3 visible px) are from docs/resources.md / CLAUDE.md.
package design

// PixelsPerCycle is the number of visible pixels the beam advances per CPU cycle (1 cycle = 3 color clocks = 3px).
const PixelsPerCycle = 3

// MinColorBandWidthPx returns the minimum horizontal width (px) at which a color is
// "visible" when its color-change write takes writeCycles cycles. Each band of horizontal
// multicolor needs at least this width.
// Examples: STA zp(3cy)→9px / STA abs(4cy)→12px / arbitrary color(LDA#imm+STA≈6cy)→18px.
// [design-principles.md "horizontal multicolor band minimum width = store instruction cycles x 3px" / mining 170018]
func MinColorBandWidthPx(writeCycles int) int {
	return writeCycles * PixelsPerCycle
}

// NarrowBand is a "band too narrow to draw" returned by CheckColorBands.
type NarrowBand struct {
	Index   int // band index (0-based from the left)
	WidthPx int // specified width
	MinPx   int // required minimum width
}

// CheckColorBands judges whether a left-to-right sequence of horizontal color-band widths (px)
// is achievable at a color-change cost of writeCycles cycles per band. The first band is treated
// as the default color at scanline start = no color change needed (0); the minimum width is
// imposed on the second band onward. Returns the list of too-narrow bands (empty slice = feasible).
func CheckColorBands(widthsPx []int, writeCycles int) []NarrowBand {
	min := MinColorBandWidthPx(writeCycles)
	var bad []NarrowBand
	for i, w := range widthsPx {
		if i == 0 {
			continue // first band is the default color = no extra color change needed
		}
		if w < min {
			bad = append(bad, NarrowBand{Index: i, WidthPx: w, MinPx: min})
		}
	}
	return bad
}

// --- Decomposing TIA color registers (common to COLUPx/COLUBK/COLUPF) ------------------
// A TIA color register is D7..D4 = hue(0–15) / D3..D1 = luminance(0–7) / D0 unused.
// Colors are held as "(hue, luminance) register values", not RGB (don't scatter raw hex).
// [design-principles.md "Color (most important)" / mining symbolic-color-names, 118495]

// LuminanceLevels is the practical number of luminance steps (bit0 unused → 8 steps).
const LuminanceLevels = 8

// VividMaxLuminance is the highest luminance step where a "color meant to look vivid" can go.
// Brighter than this, saturation drops and the color washes out (bright blue in particular
// loses its identity).
//
// WHAT THE SOURCE SUPPORTS AND WHAT IS A JUDGEMENT CALL, kept apart on purpose (2026-08-05).
// Thread 132561 establishes the PHENOMENON and nothing narrower: raising luma desaturates on
// real hardware (not an emulator artefact, and the reason is NTSC's I/Q ceiling), bright blues
// hue7-9 stop being distinguishable at the top, and the advice given is "pick mid-to-low
// luminance, and treat max ($xE) as near-white plus a tint". **It never states a threshold.**
// The 5 is this project's cut on that advice, not a measured number, and it is written here
// rather than left to look sourced.
//
// The UNIT is not ambiguous, which was worth checking: Luminance() returns (reg>>1)&7, a step
// on 0..7, and WashoutRisk takes its argument from there — so 5 is step 5 of 8, luma nibble
// $A. The source's own $x0..$xE notation is the nibble; the two scales differ by that factor
// and the code is self-consistent.
// [mining 132561 — phenomenon only; the threshold is ours]
const VividMaxLuminance = 5

// Hue returns the hue of a color register value (D7..D4, 0–15).
func Hue(reg byte) byte { return reg >> 4 }

// Luminance returns the luminance step of a color register value (D3..D1, 0–7). D0 is unused and dropped.
func Luminance(reg byte) byte { return (reg >> 1) & 0x07 }

// HueName maps the documented hue anchors to color names (hue1=yellow / hue4=red / hue8=blue /
// hue12=green, hue15≈hue1). Non-anchors return "" (no name is asserted for intermediate hues). [mining 132561]
var HueName = map[byte]string{
	1:  "yellow",
	4:  "red",
	8:  "blue",
	12: "green",
	15: "yellow", // hue15 ≈ hue1
}

// WashoutRisk returns whether luminance step lum is in the region that makes the color look
// washed out (desaturated). Keep colors meant to look vivid at or below VividMaxLuminance. [mining 132561]
func WashoutRisk(lum byte) bool { return lum > VividMaxLuminance }

// GradientSameHue returns whether the color bands (landscape gradient etc.) all share one hue,
// varying only in luminance. Gradients that mix hues are not allowed (depth comes from
// luminance steps with BG=far / PF=near). [mining 160655]
func GradientSameHue(regs []byte) bool {
	if len(regs) <= 1 {
		return true
	}
	h := Hue(regs[0])
	for _, r := range regs[1:] {
		if Hue(r) != h {
			return false
		}
	}
	return true
}

// SameLuminance reports whether two colours sit on the same luminance step. That is all
// it computes, and the name says so because the previous one — InterlaceColorsSafe — was
// making a judgement the function cannot make and the source does not support.
//
// The context is 2-frame temporal colour mixing. Perceived flicker is proportional to the
// luminance difference, so equal luminance minimises it. But the source harness cites for
// that rule carries its own rebuttal in the same note: seagtgruff points out that telling
// a dark green from a light green NEEDS some luminance difference, and 〔176987:37〕 puts
// it plainly — **完全均一は不可。「輝度差を識別に足りる最小に、hue 差を最大に」**
// ("perfect uniformity is unusable; the craft is the smallest luminance difference that
// still reads, with the largest hue difference").
//
// So equal luminance is the flicker-free end of a trade-off, not the safe choice: a pair
// this function returns true for is exactly a pair the source says the player may not be
// able to tell apart. The threshold — how small a luminance difference still reads — is
// not measured anywhere in this tree (five layers, zero hits), so no function here should
// pretend to know it.
//
// Renamed 2026-09-03 after the Stella distillation read the cited note and found the
// rebuttal on the same page as the rule. 〔176987:24, :37〕
func SameLuminance(a, b byte) bool { return Luminance(a) == Luminance(b) }
