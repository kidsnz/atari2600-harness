package ingest

import "testing"

// TestPaletteAliases pins how much of the TIA palette this project can actually distinguish.
//
// The artwork for this project is drawn in Photoshop and ingested here. **An artist can pick two
// swatches that look different, get two different TIA codes, and see one colour on the screen** —
// and nothing in the chain says so, because by the time `Nearest` runs the two have already
// collapsed into one entry. That is the failure this test exists to keep visible.
//
// Measured on the Stella NTSC table: 128 codes, **91 distinct colours**, 28 alias groups, **37 codes
// no image can ever ask for**. Four codes are one colour ($26 $28 $F6 $F8); the adjacent greys
// $08/$0A and $0C/$0E are pairs, which matters most — a luminance ramp that looks smooth in the
// swatch has two steps that do not exist.
//
// **This is a property of this table, and the control below is what establishes that.** The engine's
// own palette, run through the same two functions, gives **126 distinct colours and 2 alias groups**
// against Stella's 91 and 28 — so the collapse is not the TIA's colour generation, it is the
// measured Stella table, whose author wrote in 2001 that the colours there "seem to be idealized a
// bit". Three layers — code, an emulator's RGB, a real television — and this is the middle one of
// one emulator. Found by the mailing-list distillation (helper-2).
func TestPaletteAliases(t *testing.T) {
	q := NewStellaNTSCQuantizer()

	if got := len(q.codes); got != 128 {
		t.Fatalf("table has %d codes, want 128", got)
	}
	if got := q.DistinctColours(); got != 91 {
		t.Errorf("the palette produces %d distinct colours, want 91. If this rose, the table "+
			"changed and every count below is stale; if it fell, more of the palette just became "+
			"unreachable and nobody was told", got)
	}

	groups := q.Aliases()
	if len(groups) != 28 {
		t.Errorf("%d alias groups, want 28", len(groups))
	}
	lost := 0
	for _, g := range groups {
		lost += len(g) - 1
	}
	if lost != 37 {
		t.Errorf("%d codes are unreachable through this quantiser, want 37", lost)
	}
	if lost+q.DistinctColours() != len(q.codes) {
		t.Errorf("the arithmetic does not close: %d unreachable + %d distinct != %d codes",
			lost, q.DistinctColours(), len(q.codes))
	}

	// The widest group and the grey pairs are named, because they are the ones an artist meets.
	widest := 0
	for _, g := range groups {
		if len(g) > widest {
			widest = len(g)
		}
	}
	if widest != 4 {
		t.Errorf("widest alias group holds %d codes, want 4 ($26 $28 $F6 $F8 are one colour)", widest)
	}
	has := func(want ...uint8) bool {
		for _, g := range groups {
			if len(g) != len(want) {
				continue
			}
			ok := true
			for i := range want {
				if g[i] != want[i] {
					ok = false
					break
				}
			}
			if ok {
				return true
			}
		}
		return false
	}
	if !has(0x08, 0x0A) || !has(0x0C, 0x0E) {
		t.Errorf("the adjacent grey pairs $08/$0A and $0C/$0E should be aliases — they are the "+
			"case that turns a smooth-looking luminance ramp into one with missing steps: %v", groups)
	}

	// Control, and the reason the caveat above is a measurement rather than a hedge: the engine's
	// own palette must stay far less collapsed than the measured one. If these ever converge, the
	// aliasing has become a statement about the TIA rather than about Stella's table, and every doc
	// comment here needs re-reading.
	e := NewNTSCQuantizer()
	if e.DistinctColours() != 126 || len(e.Aliases()) != 2 {
		t.Errorf("the engine's palette gives %d distinct colours in %d alias groups, want 126 and 2 "+
			"— this is the control that separates 'the table collapses' from 'the TIA collapses'",
			e.DistinctColours(), len(e.Aliases()))
	}
	if e.DistinctColours() <= q.DistinctColours() {
		t.Errorf("the engine's palette (%d colours) is no more distinct than the measured Stella "+
			"table (%d) — the claim that the collapse belongs to the table has stopped holding",
			e.DistinctColours(), q.DistinctColours())
	}
}
