package emu

import "testing"

// TestWhichFloatingBusModelThisEngineImplements answers a question the repository has been relying
// on without ever asking: when a ROM reads a write-only TIA register, WHERE does the returned byte
// come from — and is that the same place other emulators put it?
//
// Only some data pins are driven on such a read; the rest float. Three models are in circulation
// and the engine's own source names two of them (`Gopher2600/hardware/memory/memory.go`):
//
//	if mem.env.Prefs.RandomPins.Get().(bool) {
//	    data |= uint8(mem.env.Random.Rewindable(0xff)) & (^mem.DataBusDriven)   // random
//	} else {
//	    // this pattern is good for replicating what we see on the pluscart
//	    // ... a different bit pattern can be seen on the Harmony
//	    data |= mem.LastCPUData & ^mem.DataBusDriven                            // last bus byte
//	}
//
// and B. Watson describes the third in 200508: "In both emulators, the disconnected bits will read
// as the address being read from, so bits 0-6 will be $2B, which has bit 0 set." **That is what
// Stella and z26 did in 2005** — the ADDRESS, not the bus byte.
//
// ★The measured answer: this engine is the LAST BUS BYTE, and it is therefore NOT what those two
// emulators did. That is a real difference, not a detail. Eckhard Stolberg, 200110, names three
// commercial ROMs that depend on the answer: "If you don't emulate the undefined bits in the TIA
// read-registers correctly, the ball won't bounce off of the paddles properly in Video Pinball";
// Dodge'em shows "a reversed score display" if bit 0 comes back 1; Berzerk enables a missile it
// should not. **We cannot check those three here** — the original ROMs are not in this tree
// (`reference/` has 319 .bin files and the only near match is a hacked Berzerk), and fetching them
// is not on the table. So this test pins the model, not the consequences.
//
// ★★How the two models are made to disagree is the whole design, and the first attempt at it did
// not disagree — see the ROM's header. Reading the same register through `$002B` and `$012B` gave
// bit 0 set both times, which looks exactly like the address model and is not: for a direct read
// the last bus byte IS the address's low byte, and those two addresses share it. What separates
// them is an addressing mode whose last fetched byte is not the address:
//
//	lda $2B     zero page   -> last bus byte $2B (odd)
//	lda $00,X   X=$2B       -> last bus byte $00 (even), same read register
func TestWhichFloatingBusModelThisEngineImplements(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_floatbits.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(3); err != nil {
		t.Fatal(err)
	}
	r, err := e.CurrentRAM()
	if err != nil {
		t.Fatal(err)
	}

	const (
		viaAddr  = 0x00 // $80: bit0 of `lda $2B`   — last bus byte $2B
		viaZeroX = 0x01 // $81: bit0 of `lda $00,X` — last bus byte $00
		differ   = 0x02
	)

	if r[viaAddr] != 1 {
		t.Errorf("`lda $2B` returned an even byte (bit0=%d); with the last bus byte $2B under the "+
			"last-bus-byte model, or the address $2B under the address model, bit 0 should be set "+
			"either way. Something more basic than the model has changed", r[viaAddr])
	}
	if r[viaZeroX] != 0 {
		t.Errorf("`lda $00,X` (X=$2B) returned an odd byte (bit0=%d), want 0. Under the LAST BUS "+
			"BYTE model the floating bits follow the operand $00 and bit 0 is clear; under the "+
			"ADDRESS model they follow $2B and bit 0 is set. Getting 1 here means this engine has "+
			"moved to the address model — the one Stella and z26 used in 2005 — and every claim "+
			"about reading a write-only TIA register needs re-checking against it", r[viaZeroX])
	}
	if r[differ] != 1 {
		t.Errorf("the two reads AGREED, so this test discriminated nothing. It only means something " +
			"when they disagree: agreement is what the first version of this experiment produced, " +
			"by accidentally putting the same byte on the bus both times")
	}

	// --- negative control: switch the model and the answer must move ------------------------
	// If the two reads differ for some reason OTHER than the floating bits, flipping the engine's
	// RandomPins preference would change nothing. Turning it on selects the third model.
	//
	// ★It is asserted rather than logged because the third model turns out to be REPRODUCIBLE:
	// the engine draws from `Random.Rewindable`, so three runs give the same bytes. Measured:
	// default gives ($80=1, $81=0) on every run and RandomPins gives ($80=0, $81=1) on every run.
	// "Random pins" therefore does not mean "unpredictable" here — it means a different fixed
	// pattern — and that is worth pinning, because a test that expected noise would be wrong.
	{
		e2, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		e2.VCS.Env.Prefs.RandomPins.Set(true)
		if err := e2.LoadROM("../../roms/litmus/litmus_floatbits.bin"); err != nil {
			t.Fatal(err)
		}
		if err := e2.RunFrames(3); err != nil {
			t.Fatal(err)
		}
		r2, err := e2.CurrentRAM()
		if err != nil {
			t.Fatal(err)
		}
		if r2[viaAddr] == r[viaAddr] && r2[viaZeroX] == r[viaZeroX] {
			t.Errorf("RandomPins produced the same bytes as the default model ($80=%d $81=%d) — "+
				"the preference is not reaching the read path, so the model this test claims to "+
				"identify is not actually selectable and the identification means nothing",
				r2[viaAddr], r2[viaZeroX])
		}
	}

	// --- the read half has to be a real bus cycle, or none of the above is about the bus -----
	// `$2B` is HMCLR, a strobe. The ROM sets HMP0 to $70 and then performs `lsr HMCLR`, whose
	// read-modify-write must drive the strobe and clear the horizontal-motion registers. The
	// engine stores HM biased by 8 (the oracle undoes it as `Hmove ^ 8`), so cleared reads as 8.
	if got := e.VCS.TIA.Video.Player0.Hmove; got != 8 {
		t.Errorf("after `lsr HMCLR` the player-0 motion register reads %d, want 8 (= HMP0 $00). "+
			"The strobe did not fire, so the instruction never performed a real write — and if the "+
			"write half is not real, the read half this whole test rests on is suspect too", got)
	}
}
