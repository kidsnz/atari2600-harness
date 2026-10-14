package emu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
)

// rmwROM counts frames in RAM $82. FIRST is the line under test: with `inc Count` the
// byte is incremented without ever having been written; with a store in front of it the
// same program is clean. Nothing else differs.
const rmwROM = `
        processor 6502
VSYNC   = $00
WSYNC   = $02
Count   = $82
        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
Frame:  lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        FIRST
        ldx #255
Wait:   sta WSYNC
        dex
        bne Wait
        sta WSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        jmp Frame
        org $FFFC
        .word Start
        .word Start
`

// The detector passed `inc Count` on a byte nothing had written: INC reads the byte and
// writes it back, and only pure reads and pure writes were looked at. Found by an agent
// building a task whose contract says "no RAM byte is read before the program has
// written it", when a wrong answer made for that clause came back clean.
func TestUninitReadSeesAnIncrementOfAByteNothingWrote(t *testing.T) {
	watch := func(first string) *UninitReadHit {
		dir := t.TempDir()
		asm, bin := filepath.Join(dir, "rmw.asm"), filepath.Join(dir, "rmw.bin")
		if err := os.WriteFile(asm, []byte(strings.Replace(rmwROM, "FIRST", first, 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := build.Assemble(asm, bin); err != nil {
			t.Skipf("dasm is not available, nothing can be said here: %v\n%s", err, out)
		}
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(bin); err != nil {
			t.Fatal(err)
		}
		hit, err := e.WatchUninitRead(3)
		if err != nil {
			t.Fatal(err)
		}
		return hit
	}
	for _, op := range []string{"inc Count", "dec Count", "asl Count", "lsr Count", "rol Count", "ror Count"} {
		if hit := watch(op); hit == nil {
			t.Errorf("`%s` on a byte nothing wrote was not flagged", op)
		} else if hit.Addr != 0x0082 {
			t.Errorf("`%s`: flagged $%04X, want $0082", op, hit.Addr)
		}
	}
	// the twins: the byte is written before it is modified, and modified again every frame
	// after that -- a modify is also a write, so the second frame's must not be flagged
	if hit := watch("sta Count\n        inc Count"); hit != nil {
		t.Errorf("a store followed by `inc Count` was flagged at $%04X, pc $%04X", hit.Addr, hit.PC)
	}
	if hit := watch("nop"); hit != nil {
		t.Errorf("a program that never touches the byte was flagged at $%04X", hit.Addr)
	}
}
