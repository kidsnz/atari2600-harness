package emu

import "testing"

// TestRIOTDataRegistersIgnoreA3AndA4 measures which address lines the RIOT's data
// registers decode, over four input states.
//
// The 2600's RIOT window at $0280 holds four registers in its bottom two address lines.
// The list settled what the upper lines do on real hardware: Eckhard Stolberg, measuring a
// 7800 in 2600 mode — after the person asking said an emulator would not settle it —
// reported $288, SWCHA with A3 set, reading back as the port.
//
// Measured in FOUR input states on purpose. A mirror sampled only while the port reads
// $FF is indistinguishable from a register that always reads $FF, and the first version of
// this measurement did exactly that: it read the ports once at reset, before any input was
// applied, and reported $FF everywhere. The states below make SWCHA take four distinct
// values, so an agreeing mirror is agreeing about something.
//
// ★The negative control is inside the same table. A1 IS decoded — SWCHA and SWCHB are
// different registers — so the assertions require the mirrors to agree AND $0280 to keep
// differing from $0282. Without the second half, a decoder that had stopped discriminating
// entirely would pass.
func TestRIOTDataRegistersIgnoreA3AndA4(t *testing.T) {
	states := []struct {
		label  string
		player int
		action string
	}{
		{"no input", 0, ""},
		{"P0 left", 0, "left"},
		{"P0 down", 0, "down"},
		{"P1 right", 1, "right"},
	}

	seen := map[byte]bool{}
	for _, st := range states {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM("../../roms/litmus/litmus_riot_mirror.bin"); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(2); err != nil {
			t.Fatal(err)
		}
		if st.action != "" {
			if err := e.SetInput(st.player, st.action, true); err != nil {
				t.Fatal(err)
			}
		}
		if err := e.RunFrames(3); err != nil {
			t.Fatal(err)
		}
		r, err := e.CurrentRAM()
		if err != nil {
			t.Fatal(err)
		}
		swcha, a3, a4, a34 := r[0x00], r[0x01], r[0x02], r[0x03]
		swchb, bA3 := r[0x04], r[0x05]
		t.Logf("%-9s SWCHA $0280=$%02X  +A3 $0288=$%02X  +A4 $0290=$%02X  +A3A4 $0298=$%02X   |   SWCHB $0282=$%02X  +A3 $028A=$%02X",
			st.label, swcha, a3, a4, a34, swchb, bA3)
		seen[swcha] = true

		for _, m := range []struct {
			name string
			got  byte
		}{{"$0288 (A3)", a3}, {"$0290 (A4)", a4}, {"$0298 (A3+A4)", a34}} {
			if m.got != swcha {
				t.Errorf("%s: %s reads $%02X, SWCHA reads $%02X — A3/A4 are decoded here",
					st.label, m.name, m.got, swcha)
			}
		}
		if bA3 != swchb {
			t.Errorf("%s: $028A reads $%02X, SWCHB reads $%02X", st.label, bA3, swchb)
		}
		// The control: one address line lower and it IS a different register.
		if swcha == swchb {
			t.Errorf("%s: SWCHA and SWCHB both read $%02X — A1 has stopped discriminating, "+
				"so the mirror agreement above proves nothing", st.label, swcha)
		}
	}

	// And the states have to have moved the port, or every comparison above was made
	// against the same constant.
	if len(seen) < 3 {
		t.Errorf("SWCHA took only %d distinct values across %d states; a mirror measured in "+
			"one state cannot be told from a constant", len(seen), len(states))
	}
}
