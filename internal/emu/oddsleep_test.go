package emu

import "testing"

// TestSpendingAnOddNumberOfCyclesHasThreeAnswersAndTwoAreTraps settles a question the mailing list
// asked in 2004 and left hanging, and corrects the answer the list itself was converging on.
//
// This repository pads with `ds N, $EA`, which spends 2N cycles and therefore cannot make an ODD
// delay. Sprite positioning asks for odd delays routinely, and the obvious reach is DASM's `SLEEP`
// macro. What that macro emits is not a matter of opinion — `machines/atari2600/macro.h` of
// dasm 2.20.14.1 says:
//
//	IF .CYCLES & 1
//	    IFNCONST NO_ILLEGAL_OPCODES
//	        nop 0
//	    ELSE
//	        bit VSYNC
//	    ENDIF
//
// and assembling it confirms the bytes: SLEEP 2 -> EA, SLEEP 3 -> 04 00, SLEEP 4 -> EA EA,
// SLEEP 5 -> 04 00 EA; with -DNO_ILLEGAL_OPCODES=1 the 04 becomes 24.
//
// Dennis Debro (200401): "If you're using the SLEEP macro in macro.h and have not turned off
// illegal opcodes, then when your SLEEP value is >= 3 you're using `nop 0`." Eckhard Stolberg
// (200403): "I think the SLEEP macro uses undocumented opcodes in it's default state. StellaX
// doesn't support these. There should be a compiler switch..." Kirk Israel asked in that same
// thread, "though if it's a multiple of 2, isn't SLEEP 'safe' re: undocumented opcodes?" — and
// nobody answered him. **Yes: even is safe.**
//
// ★But the switch they were recommending does not make odd safe, and no one in either thread says
// so. `bit $00` is a legal opcode that READS THE SAME ADDRESS as `nop $00`. $00 has A6 and A7 low,
// so on a 3F/X07 cart it is a bankswitch hotspot — the exact pattern `check_traps.py` warns about,
// and the engine's own tigervision mapper states the condition (`addr&0x10c0 == 0x0000`).
// `NO_ILLEGAL_OPCODES` fixes the opcode and leaves the trap. **Both branches of odd SLEEP are
// unsafe on a bankswitched cart, and both are invisible to a source-level grep** because the bytes
// only exist after macro expansion.
//
// The third option is the one this test measures: Jim Nitchals, 199704, "if you need to delay for
// 7 cycles, a PHP/PLP is a code-compact way to do it." Measured here at 7 cycles in two bytes,
// restoring every flag and touching no address outside the stack.
func TestSpendingAnOddNumberOfCyclesHasThreeAnswersAndTwoAreTraps(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_oddsleep.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(3); err != nil {
		t.Fatal(err)
	}
	r, err := e.CurrentRAM()
	if err != nil {
		t.Fatal(err)
	}

	// ★The overhead of the measurement is measured, not assumed: $8F is an EMPTY interval, and
	// every figure is read against it. A hard-coded constant here would be asserting this
	// harness's own timing rather than the instruction's.
	base := int(r[0x0F])
	if base < 100 || base > 160 {
		t.Fatalf("the empty measurement reads %d, which is outside anything the loop could take — "+
			"the timer setup itself has moved and every difference below is meaningless", base)
	}
	cy := func(addr int) int { return int(r[addr]) - base }

	for _, tc := range []struct {
		what string
		addr int
		want int
		why  string
	}{
		{"nop $00 (odd SLEEP, default)", 0x00, 3,
			"the illegal opcode DASM emits for an odd SLEEP"},
		{"bit $00 (odd SLEEP, NO_ILLEGAL_OPCODES)", 0x01, 3,
			"legal, same cost — and the same address, which is the point"},
		{"php", 0x02, 3, ""},
		{"plp", 0x03, 4, ""},
		{"php/plp", 0x04, 7,
			"the odd delay that is legal AND touches no address: two bytes, seven cycles"},
		{"ds 2,$EA", 0x07, 4, "the even baseline this repository already uses"},
	} {
		if got := cy(tc.addr); got != tc.want {
			msg := "%s takes %d cycles, want %d"
			if tc.why != "" {
				msg += " — " + tc.why
			}
			t.Errorf(msg, tc.what, got, tc.want)
		}
	}

	// --- PLP has to put back everything, or it is not a transparent delay -------------------
	if got := r[0x05]; got != 0xB5 {
		t.Errorf("a status byte of $B5 round-tripped through plp/php comes back as $%02X — plp does "+
			"not restore every flag, so php/plp is NOT a transparent delay and the recommendation "+
			"above is wrong", got)
	}
	if r[0x06] != 1 {
		t.Error("php/plp changed A, X or Y — a delay that clobbers a register is not a delay")
	}

	// --- and the reason php/plp cannot be used EVERYWHERE ------------------------------------
	// `docs/techniques/missiles-bullets.md` points SP at the ENAM mirror so that PHP *writes a TIA
	// register*: "[$011D]=ENAM0, [$011E]=ENAM1, [$011F]=ENABL". Inside a kernel doing that, seven
	// cycles of php/plp does not spend seven cycles — it fires the missile. Measured, not asserted.
	tia := e.ReadTIARegisters()
	if r[0x08] != 1 {
		t.Fatal("the ROM never reached the stack-into-TIA section, so the two results below are " +
			"leftover zeros rather than measurements")
	}
	if !tia.Missile0.Enabled || !tia.Missile1.Enabled {
		t.Errorf("with SP at $1E, two pushes left ENAM0=%v ENAM1=%v, want both true — either the "+
			"stack no longer reaches TIA space, or missiles-bullets.md's `[$011D]=ENAM0` is wrong. "+
			"Whichever it is, the PHP-writes-a-missile technique and the PHP-as-delay idiom stop "+
			"being incompatible, and that is worth knowing",
			tia.Missile0.Enabled, tia.Missile1.Enabled)
	}
	if got := r[0x09]; got != 0x1C {
		t.Errorf("after two pushes from SP=$1E the stack pointer is $%02X, want $1C — pushes must "+
			"DESCEND for the doc's `SP $1E->$1D` ladder to land one register per PHP", got)
	}
}
