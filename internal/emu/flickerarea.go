package emu

import "fmt"

// FlickerArea counts the pixels whose DRAWING OBJECT changes between two consecutive frames.
//
// ★Why not a pixel diff. `cmd/still` has one — `diffPixels` — and its own comment says why it
// cannot answer this question: "READ THIS AS COLOUR PLUS GEOMETRY, NEVER AS GEOMETRY … COLUPF
// follows the drum envelope in EVERY build, glitched or not, so the clean control still reports
// 6136 differing pixels between its two frames … a sanity check on the capture, not a measure of
// an effect". Six thousand pixels is twenty per cent of the picture, from colour alone. The same
// comment names the right instrument: "the per-element attribution in `emu.DecomposeRow` is what
// answers the first question".
//
// So this compares ELEMENTS — BG/PF/P0/P1/M0/M1/BL — not colours. A playfield whose colour sweeps
// every frame contributes nothing; a player that appears on odd frames and vanishes on even ones
// contributes its own area twice over. Measured on a static picture (`litmus_pal`) the answer is
// exactly **0**, which is the number `diffPixels` cannot produce.
//
// ★★What it is FOR. The archive is specific that flicker is judged by area and not by object
// count: "an area as large as an Arkanoid wall is going to be hard on the eyes even at 30 Hz
// flicker, and positively headache-inducing beyond that" 〔stella-list `200108/msg00315`, Erik
// Mooney, 2001-08-20〕. That threshold is a phrase, not a number, and this function does not invent
// one — it gives the author a quantity so they can set their own ceiling once and have a machine
// keep it. Designed by the mailing-list distillation (helper-3), who found the `diffPixels` trap
// first and routed around it; cost and behaviour measured here.
//
// ★★★What it does NOT distinguish: movement from blinking — and, measured 2026-09-07, it charges
// motion MORE. On one 8x15 player with everything else held still
// (`internal/emu/flickermotion_test.go`):
//
//	still                      0
//	blinking on/off          120   = 8 x 15, the whole object
//	drifting 1 px a frame     30   = 2 x 1 x 15
//	drifting 4 px a frame    120   = the same as a full blink
//	drifting 8 px a frame    240   = TWICE a full blink
//
// The law is 2·d·h, saturating at 2·w·h once the object clears its own width: two edges move, each d
// wide, on every line. So four pixels a frame — an ordinary speed — is indistinguishable from a
// sprite switching on and off, and eight scores double. A ceiling chosen from a flicker budget will
// fire on something that never flickers, and the number will be RIGHT; it is the name that misleads.
// Compare like with like: across frames whose motion matches, or on a scene held still. A sprite that moves ten pixels left
// changes the element at twenty columns and reads as flicker. `read_motion` is the instrument for
// that axis. This one answers "how much of the picture is not the same thing two frames running".
//
// ★★★★CALIBRATED against the author's own naming, on 134 built ROMs of the first work. Ten of
// them have "flick" in the filename because that is what they are, and **all ten read non-zero** —
// no false negatives against a judgement made by the person who wrote them. 115 read exactly zero.
// The flickering variants land between **2.6% and 8.0%** of the visible picture
// (`transistor-flick12` 808 px, `full-flick12s` 1234, `full-flick` 1414, `full-flick12` 2468, of
// 30,720).
//
// ★★★★★Nine ROMs read non-zero without "flick" in the name, and they are the limitation above
// showing up in real work rather than being argued: `meterv-noise` and `meterv-peak` are meters
// whose bars change height, and seven `probe*` ROMs change something deliberately every frame.
// Movement and blinking are the same measurement here. That is the number's boundary, demonstrated.
func (e *Emu) FlickerArea() (int, error) {
	if e.elemBuf == nil {
		e.EnableElementCapture()
		// One frame to fill the buffer before the first snapshot means the caller does not have
		// to know that enabling capture is not retroactive.
		if err := e.RunFrames(1); err != nil {
			return 0, err
		}
	}
	before, err := e.elementMap()
	if err != nil {
		return 0, err
	}
	if err := e.RunFrames(1); err != nil {
		return 0, err
	}
	after, err := e.elementMap()
	if err != nil {
		return 0, err
	}
	if len(before) != len(after) {
		// The visible window changed between the two frames, so the rows do not correspond and a
		// per-pixel comparison would be comparing different parts of the picture.
		return 0, fmt.Errorf("flicker_area: the visible window changed between the two frames "+
			"(%d rows then %d), so the rows do not correspond — get a stable frame first "+
			"(frame_lines_stable) before asking what flickers", len(before), len(after))
	}
	area := 0
	for y := range before {
		for x := range before[y] {
			if before[y][x] != after[y][x] {
				area++
			}
		}
	}
	return area, nil
}

// elementMap snapshots the drawing object of every visible pixel of the current frame.
func (e *Emu) elementMap() ([][]uint8, error) {
	vt := e.cap.frameInfo.VisibleTop
	vb := e.cap.frameInfo.VisibleBottom
	out := make([][]uint8, 0, vb-vt+1)
	for y := vt; y <= vb; y++ {
		runs, w, err := e.DecomposeRow(y)
		if err != nil {
			return nil, err
		}
		row := make([]uint8, w)
		for _, r := range runs {
			id := elemID(r.Element)
			for i := 0; i < r.Len && r.Clock+i < w; i++ {
				row[r.Clock+i] = id
			}
		}
		out = append(out, row)
	}
	return out, nil
}

func elemID(name string) uint8 {
	for i, n := range elemNames {
		if n == name {
			return uint8(i + 1)
		}
	}
	return 0
}
