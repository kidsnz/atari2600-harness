package emu

import "testing"

// The line profiler is the ∃ oracle the ∀ prover is graded against, so it has to
// be trustworthy about two things it used to be silent on.
//
// First, it keyed its rows on the strobe PC alone. Two banks that store WSYNC at
// one address would merge into a single row, and a containment check against the
// static proof would then pass while testing half of what it claims. No ROM in the
// corpus executes the same address in two banks, so the old keying happened to be
// correct — by ROM layout, not by the instrument being right.
//
// Second, an interval that straddles a frame boundary has no valid cycle count
// from the beam coordinates, so it is not aggregated. That is correct, but it was
// dropped silently, which makes a table that is merely incomplete look complete.
func TestProfileTagsTheBankAndReportsWhatItDropped(t *testing.T) {
	cases := []struct {
		rom        string
		wantBanked bool
	}{
		{"../../roms/litmus/litmus_bank.bin", true},
		{"../../roms/techniques/banked_game.bin", true},
		{"../../roms/litmus/litmus_lastline.bin", false},
	}
	for _, c := range cases {
		e, err := New("NTSC")
		if err != nil {
			t.Skip("emulator unavailable")
		}
		if err := e.LoadROM(c.rom); err != nil {
			t.Skipf("ROM unavailable (%s): %v", c.rom, err)
		}
		rows, dropped, err := e.ProfileLineWorst(6, nil)
		if err != nil {
			t.Fatalf("%s: %v", c.rom, err)
		}
		if len(rows) == 0 {
			t.Fatalf("%s: no intervals measured, so this proves nothing", c.rom)
		}

		tagged := 0
		for _, r := range rows {
			if r.BankValid {
				tagged++
			}
		}
		switch {
		case c.wantBanked && tagged != len(rows):
			t.Errorf("%s is bank-switched but only %d of %d rows carry a bank; an untagged row can "+
				"merge with another bank's row at the same address", c.rom, tagged, len(rows))
		case !c.wantBanked && tagged != 0:
			t.Errorf("%s is a flat image, so no row should carry a bank (that would make bank 0 and "+
				"'not a banked cartridge' the same answer); %d do", c.rom, tagged)
		}

		// Six frames means five frame boundaries, and each one closes an interval the
		// coordinate arithmetic cannot cost. The count must be reported, and it must
		// be the boundaries rather than a number that drifts with the ROM.
		if dropped != 5 {
			t.Errorf("%s: %d cross-frame intervals dropped over 6 frames; expected one per boundary "+
				"(5). A count that does not track the boundaries means intervals are being lost for "+
				"some other reason, silently", c.rom, dropped)
		}
	}
}
