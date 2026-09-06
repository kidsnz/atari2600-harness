package emu

import (
	"os"
	"path/filepath"
	"testing"
)

// buildTIAReadROM reads four TIA addresses through ABSOLUTE addressing, with `hi` as the
// most-significant byte, and stores each result in RAM.
//
// Absolute addressing is the point: with `lda $0E` the one address byte is both the address and
// the last byte on the bus, so the two cannot be told apart. `lda $hi0E` separates them.
func buildTIAReadROM(t *testing.T, hi uint8) string {
	t.Helper()
	prog := []byte{
		0xA9, 0x02, 0x85, 0x00, 0x85, 0x02, 0x85, 0x02, 0x85, 0x02,
		0xA9, 0x00, 0x85, 0x00,
		0xAD, 0x0E, hi, 0x85, 0x90, // LDA $hi0E / STA $90   — nothing decodes $0E
		0xAD, 0x1F, hi, 0x85, 0x91, // LDA $hi1F / STA $91   — nor $1F
		0xAD, 0x08, hi, 0x85, 0x92, // LDA $hi08 / STA $92   — INPT0, a real read register
		0xAD, 0x02, hi, 0x85, 0x93, // LDA $hi02 / STA $93   — CXM0P, a real read register
		0x4C, 0x21, 0xF0,
	}
	rom := make([]byte, 4096)
	copy(rom, prog)
	rom[0x0FFC], rom[0x0FFD] = 0x00, 0xF0
	rom[0x0FFE], rom[0x0FFF] = 0x00, 0xF0
	p := filepath.Join(t.TempDir(), "tiaread.bin")
	if err := os.WriteFile(p, rom, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// TestOnlyTwoTIAPinsAreDrivenAndTheRestCarryTheAddress pins what a TIA read actually returns.
//
// ★The claim comes from the schematics. Kevin Horton, who had them in front of him, wrote to the
// list in 2001: *"When reading, only D6 and D7 are used. period. The chip only has readback buffers
// for D6 and D7, and D0-D5 just hang in the breeze. So, if you are going to do an absolute
// comparison ... you must AND off the lower 6 bits; i.e. LDA tia_register AND #0C0h. Both bits are
// output during a read to ANY TIA address... if nothing happens to be decoded, then 0's are
// returned."* 〔stella-list 200109/msg00291〕. Recovered by the mailing-list distillation
// (helper-2) from the raw archive.
//
// ★★The engine agrees and says so in its own words — `hardware/memory/vcs/tia.go` defines
// `TIADrivenPins = 0b11000000` and explains that the undriven six bits are *"left over from the
// address ... the masking is applied to the most recent byte of the address to be put on the
// address bus. In all cases, this is the most-significant byte."*
//
// ★★★Measured here 2026-09-05, which is what this test is for, because the two statements above
// are a source and a comment and neither is a measurement. Reading `$0E`, `$1F`, `$08` (INPT0) and
// `$02` (CXM0P) through `lda $hi..`:
//
//	hi = $00 → all four read $00
//	hi = $01 → all four read $01
//	hi = $2A → all four read $2A
//
// The value follows the ADDRESS BYTE, not the register, and it does so for the two real read
// registers exactly as for the two nothing decodes. D6 and D7 are zero throughout because nothing
// is colliding and no paddle is connected — which is also Horton's "0's are returned".
//
// ★★★★The consequence for anyone reading a TIA register: **compare only D6 and D7**. A ROM that
// tests `lda CXM0P` against a whole byte is testing the address it happened to use. This is why
// `scripts/check_traps.py`'s message for a write-only TIA read says what it says.
func TestOnlyTwoTIAPinsAreDrivenAndTheRestCarryTheAddress(t *testing.T) {
	// $12 and $3F are deliberately absent: `$120E` and `$3F0E` have A12 set once folded to 13 bits
	// and so are CARTRIDGE reads, not TIA reads at all. A first run of this probe used
	// $00/$40/$80/$C0, every one of which has its low six bits zero — four seeds that cannot tell
	// the two models apart. Both mistakes are why the seeds below are stated with their reason.
	for _, hi := range []uint8{0x00, 0x01, 0x2A} {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(buildTIAReadROM(t, hi)); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(2); err != nil {
			t.Fatal(err)
		}
		get := func(a uint16) uint8 {
			v, err := e.PeekRAM(a)
			if err != nil {
				t.Fatal(err)
			}
			return v
		}
		for _, c := range []struct {
			name string
			at   uint16
		}{
			{"$0E (nothing decodes it)", 0x90},
			{"$1F (nothing decodes it)", 0x91},
			{"$08 INPT0 (a real read register)", 0x92},
			{"$02 CXM0P (a real read register)", 0x93},
		} {
			got := get(c.at)
			if got&0xC0 != 0x00 {
				t.Errorf("hi=$%02X, %s: the driven bits read $%02X, expected zero — nothing is "+
					"colliding and no paddle is plugged in, so D6 and D7 should both be low",
					hi, c.name, got&0xC0)
			}
			if got&0x3F != hi&0x3F {
				t.Errorf("hi=$%02X, %s: the undriven bits read $%02X, expected $%02X (the low six "+
					"bits of the address's high byte). The floating-bus model has changed: it was "+
					"the most-significant address byte on 2026-09-05, and `known-traps.md` and "+
					"`check_traps.py` both describe it that way",
					hi, c.name, got&0x3F, hi&0x3F)
			}
		}
		t.Logf("hi=$%02X: $0E=$%02X $1F=$%02X INPT0=$%02X CXM0P=$%02X",
			hi, get(0x90), get(0x91), get(0x92), get(0x93))
	}

	// ★A witness: if every seed produced the same byte the test above would pass without
	// distinguishing the two models, which is exactly how the first version of this probe failed.
	// Require the results to actually track the seed.
	seen := map[uint8]bool{}
	for _, hi := range []uint8{0x00, 0x01, 0x2A} {
		e, _ := New("NTSC")
		if err := e.LoadROM(buildTIAReadROM(t, hi)); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(2); err != nil {
			t.Fatal(err)
		}
		v, err := e.PeekRAM(0x90)
		if err != nil {
			t.Fatal(err)
		}
		seen[v] = true
	}
	if len(seen) < 3 {
		t.Fatalf("three different address bytes produced only %d distinct results — the reads are "+
			"not following the address, and the assertions above are passing for another reason",
			len(seen))
	}
}
