package emu

import "testing"

// TestSWACNTTruthTable measures what writing the port-A direction register does.
// `fundamentals-audit.md` carried it as ⬜ with the measurement already named — *"Measure what
// writing SWACNT does, which is the truth table … not the default"* — and no ROM here had ever
// written SWACNT, so the register had never been driven.
//
// The mailing list drove it: `poor-man-s-cart-dumper` (2005-08) is a dumper in which the 2600 talks
// serial out of a joystick port at 1200 baud, and a third party reports it working on hardware. That
// is a working report, not a document, which is why this stops being 📖.
//
//	SWACNT | wrote | reads back | what it means
//	  $00  |   —   |    $FF     | the peripheral owns the port (idle stick)
//	  $FF  |  $A5  |    $A5     | every bit driven; the write reads back
//	  $F0  |  $5A  |    $5F     | SPLIT: high nibble from the latch, low from the peripheral
//	  $00  |   —   |    $FF     | handing it back is clean; the latch does not linger
//
// The third row is the one that matters beyond dumping: a port that is half output and half input
// at the same time is what a two-console link needs, and the fourth says a program that drives the
// port can give it back — the direction can be reversed. Neither is in the Programmer's Guide's
// one-line description.
func TestSWACNTTruthTable(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Skip("emulator unavailable")
	}
	if err := e.LoadROM("../../roms/litmus/litmus_swacnt.bin"); err != nil {
		t.Skipf("litmus unavailable: %v", err)
	}
	if err := e.RunFrames(4); err != nil {
		t.Fatal(err)
	}
	at := func(a uint16) uint8 {
		v, err := e.PeekRAM(a)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}

	if got := at(0x80); got != 0xFF {
		t.Errorf("SWACNT=$00 with nothing pressed reads $%02X, want $FF — if this is not the "+
			"peripheral's value the rest of the table is measuring the wrong thing", got)
	}
	if got := at(0x81); got != 0xA5 {
		t.Errorf("SWACNT=$FF after writing $A5 reads $%02X, want $A5 — driving the port as an "+
			"output is what the 2005 cartridge dumper does to send serial at 1200 baud", got)
	}
	// The split is asserted nibble by nibble, because "$5F" alone would also be produced by a port
	// that ignored the write and happened to sit at $5F.
	got := at(0x82)
	if got&0xF0 != 0x50 {
		t.Errorf("SWACNT=$F0 after writing $5A: high nibble is $%X, want $5 (from the output "+
			"latch)", got>>4)
	}
	if got&0x0F != 0x0F {
		t.Errorf("SWACNT=$F0 after writing $5A: low nibble is $%X, want $F (from the peripheral — "+
			"a half-input port is what a two-console link needs)", got&0x0F)
	}
	if last := at(0x83); last != 0xFF {
		t.Errorf("after setting SWACNT back to $00 the port reads $%02X, want $FF — the output "+
			"latch must not survive handing the port back, or the direction cannot be reversed", last)
	}
}
