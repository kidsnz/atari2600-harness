package emu

import "testing"

// TestSwitchDrawCostsTheSameOnBothPaths measures a kernel whose drawing and skipping lines cost the
// same number of cycles, and states what that costs in ROM.
//
// The problem is on record: `fundamentals-audit.md`, measured 2026-09-03 — the classic skipDraw
// idiom is **20 cycles on the lines that draw and 17 on the lines that skip**, so a kernel budgeted
// at a constant loses three cycles on exactly the tightest lines. `skipdraw_test.go` pins that.
//
// The answer is on the mailing list, 2005-02 (Thomas Jentzsch, *"NOT skipdraw"*), with a follow-up
// thread that names it **SwitchDraw**. What the list never wrote down is the price: the first post
// says only *"it has some disadvantages"*, and the two replies are a correction and a further saving.
// (The first post's code is also broken — its wait loop branches to itself — so a reader who finds
// only that message copies something that hangs. The second post is the corrected one.)
//
// `litmus_switchdraw` removes the branch by making the TABLE absorb the range test: `sprDraw` walks
// the whole byte range, so a 256-entry table with the art at 0..H-1 and zero elsewhere turns "am I in
// range" into an array index. Every line is then
//
//	lda #H-1 / DCP sprDraw / ldx sprDraw / lda Art256,x / sta GRP0
//
// **17 cycles, drawing or not.** Three better than the old draw path, equal to the old skip path,
// and — the point — *the same number twice*.
//
// **The price: a 256-byte table instead of 8.** 248 bytes, 6% of a 4K cartridge, for one sprite, and
// it does not share between sprites with different art. Three cycles a line bought with 248 bytes.
// Whether that trade is right depends on which resource is binding, which is a budget question and
// not a technique question — this test states the number and does not take a side.
func TestSwitchDrawCostsTheSameOnBothPaths(t *testing.T) {
	const (
		staZP    = 0x85 // STA zp
		wsync    = 0x02
		grp0     = 0x1B
		expected = 17
	)
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM("../../roms/litmus/litmus_switchdraw.bin"); err != nil {
		t.Skipf("litmus unavailable: %v", err)
	}
	if err := e.RunFrames(8); err != nil {
		t.Fatal(err)
	}

	// With no branch there is nothing to watch for the path, so the paths are told apart by what
	// lands in GRP0: a non-zero pattern means this line drew, zero means it skipped.
	drew, skipped := map[int]int{}, map[int]int{}
	armed, cycles := false, 0
	for step := 0; step < 40000; step++ {
		pc := e.PC()
		op, err := e.VCS.Mem.Read(pc)
		if err != nil {
			t.Fatalf("read opcode at $%04X: %v", pc, err)
		}
		operand, err := e.VCS.Mem.Read(pc + 1)
		if err != nil {
			t.Fatalf("read operand at $%04X: %v", pc+1, err)
		}
		isGRP0 := op == staZP && operand == grp0
		var a uint8
		if isGRP0 {
			a = e.A()
		}
		tr, err := e.TraceClocks(1)
		if err != nil || len(tr) == 0 {
			break
		}
		switch {
		case op == staZP && operand == wsync:
			armed, cycles = true, 0
			continue
		case !armed:
			continue
		}
		cycles += tr[0].Cycles
		if isGRP0 {
			if a != 0 {
				drew[cycles]++
			} else {
				skipped[cycles]++
			}
			armed = false
		}
	}

	if len(drew) == 0 || len(skipped) == 0 {
		t.Fatalf("only one kind of line was exercised (drew=%v skipped=%v) — the fixture must both "+
			"draw and skip for this measurement to mean anything", drew, skipped)
	}
	if len(drew) != 1 || len(skipped) != 1 {
		t.Errorf("a branchless kernel must take ONE cycle count on each kind of line; got drew=%v "+
			"skipped=%v", drew, skipped)
	}
	for c := range drew {
		if c != expected {
			t.Errorf("drawing line is %d cycles from WSYNC to GRP0, want %d", c, expected)
		}
	}
	for c := range skipped {
		if c != expected {
			t.Errorf("skipping line is %d cycles from WSYNC to GRP0, want %d", c, expected)
		}
	}

	// **Negative control, and the strongest available kind: the same measurement code on the OTHER
	// fixture.** If this loop cannot tell a 20 from a 17, the agreement above is a property of the
	// measurement rather than of the kernel. Run it on the branching ROM and it must disagree.
	if e2, err := New("NTSC"); err == nil {
		if err := e2.LoadROM("../../roms/techniques/vertical_pos_dcp.bin"); err == nil {
			if err := e2.RunFrames(8); err != nil {
				t.Fatal(err)
			}
			d2, s2 := map[int]int{}, map[int]int{}
			armed, cycles := false, 0
			for step := 0; step < 40000; step++ {
				pc := e2.PC()
				op, err := e2.VCS.Mem.Read(pc)
				if err != nil {
					break
				}
				operand, err := e2.VCS.Mem.Read(pc + 1)
				if err != nil {
					break
				}
				isGRP0 := op == staZP && operand == grp0
				var a uint8
				if isGRP0 {
					a = e2.A()
				}
				tr, err := e2.TraceClocks(1)
				if err != nil || len(tr) == 0 {
					break
				}
				if op == staZP && operand == wsync {
					armed, cycles = true, 0
					continue
				}
				if !armed {
					continue
				}
				cycles += tr[0].Cycles
				if isGRP0 {
					if a != 0 {
						d2[cycles]++
					} else {
						s2[cycles]++
					}
					armed = false
				}
			}
			same := true
			for c := range d2 {
				if _, ok := s2[c]; !ok {
					same = false
				}
			}
			if len(d2) > 0 && len(s2) > 0 && same {
				t.Errorf("the control failed: the branching kernel measured the same on both paths "+
					"(drew=%v skipped=%v). It is 20 and 17 — if this code cannot see that, the "+
					"agreement measured above proves nothing", d2, s2)
			}
		}
	}

	// The whole claim in one assertion: the two paths agree. This is what the branching version
	// cannot do — `skipdraw_test.go` pins that one at 20 and 17.
	var d, s int
	for c := range drew {
		d = c
	}
	for c := range skipped {
		s = c
	}
	if d != s {
		t.Errorf("the paths differ by %d cycles (draw %d, skip %d) — a branchless kernel that still "+
			"differs is not branchless, and the technique's entire value is that the number is the "+
			"same twice", d-s, d, s)
	}
}
