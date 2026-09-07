package emu

import (
	"testing"
)

// TestTheSWACNTDelayIsNotModelled turns a comment in the vendored engine into a measurement, and
// names the trap that follows from it.
//
// The Stella Programmer's Guide, quoted on the list in 2001 by Nicolás Olhaberry: *"a delay of **400
// microseconds** is necessary between **writing** to this port and reading the TIA input ports"*
// 〔`200111/msg00192`〕. Chad Schell, who had a working 38.4 kbps serial link through the port, bounds
// it usefully: *"If you **only read** the port, and thus don't change it's configuration, the 400 uS
// delay **does not apply**"* 〔`200111/msg00194`〕. So the constraint is about **changing the
// direction**, not about reading — which matters, because reading is what most ROMs do.
//
// The engine says it does not implement this (`peripherals/controllers/keypad.go`):
//
//	"We're not emulating this here because as far as I can tell there is no need to.  More over,
//	 I'm not sure what's supposed to happen if the 400ms is not adhered to.
//	 !!TODO: Consider adding 400ms delay for SWACNT settings to take effect."
//
// Measured 2026-09-07 rather than taken from that comment (`roms/litmus/litmus_swacnt_delay.asm`):
// writing `$F0` to SWACNT and reading SWCHA on the **very next instruction** gives `$0F`, and reading
// after **608 cycles** — comfortably past the 477 that 400 µs works out to — gives `$0F` as well.
//
// ★**So the emulator cannot tell a compliant ROM from one that violates the guide**, and a ROM that
// changes SWACNT and reads immediately is green here whatever a console would do.
//
// ★★**What a console DOES do is not recorded anywhere this project can reach.** The guide states the
// requirement without a consequence, the engine's author writes *"I'm not sure what's supposed to
// happen"*, and the list thread bounds when it applies without saying what breaks. That is the honest
// end of this measurement: the leniency is established, the hardware behaviour is not.
//
// Found by the mailing-list distillation (helper-1), who matched the engine's open TODO to the thread
// that answers half of it.
func TestTheSWACNTDelayIsNotModelled(t *testing.T) {
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM("../../roms/litmus/litmus_swacnt_delay.bin"); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(6); err != nil {
		t.Fatal(err)
	}
	immediate, _ := e.PeekRAM(0x80)
	delayed, _ := e.PeekRAM(0x81)
	written, _ := e.PeekRAM(0x82)

	if written != 0xF0 {
		t.Fatalf("the litmus recorded $%02X as the direction it wrote, want $F0 — the write did not "+
			"happen and the two reads below are not about a direction change at all", written)
	}
	if immediate != delayed {
		t.Errorf("SWCHA reads $%02X immediately after the SWACNT write and $%02X after 608 cycles. "+
			"They used to agree, which is what made the 400 microsecond delay invisible here. If "+
			"they now differ the engine has started modelling it, and known-traps.md's row saying "+
			"it does not is out of date", immediate, delayed)
	}
	// Two-sided: a reading of $00 or $FF would mean the port is not responding at all, and then
	// "the two agree" would be true for a reason that has nothing to do with the delay.
	if immediate == 0x00 || immediate == 0xFF {
		t.Errorf("SWCHA reads $%02X at both timings. That is the port saying nothing rather than "+
			"the direction change being instant, so the agreement above proves nothing", immediate)
	}
}
