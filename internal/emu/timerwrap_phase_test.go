package emu

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
)

// waitROM sets the timer and then waits for it with WAIT, after PAD cycles of NOPs that
// move where in the wait the wrap falls.
const waitROM = `
        processor 6502
VSYNC   = $00
WSYNC   = $02
INTIM   = $0284
TIM64T  = $0296
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
        lda #TICKS
        sta TIM64T
        repeat PAD
        nop
        repend
WAIT
        ldx #200
Rest:   sta WSYNC
        dex
        bne Rest
        jmp Frame
        org $FFFC
        .word Start
        .word Start
`

func watchWait(t *testing.T, ticks, wait string, pad int) *TimerWrapHit {
	t.Helper()
	dir := t.TempDir()
	src := strings.NewReplacer("TICKS", ticks, "WAIT", wait).Replace(waitROM)
	src = fmt.Sprintf("PAD = %d\n", pad) + src
	asm, bin := filepath.Join(dir, "w.asm"), filepath.Join(dir, "w.bin")
	if err := os.WriteFile(asm, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := exec.LookPath("dasm"); err != nil {
		t.Skip("dasm is not available")
	}
	if out, err := build.Assemble(asm, bin); err != nil {
		t.Fatalf("the test ROM does not assemble: %v\n%s", err, out)
	}
	e, err := New("NTSC")
	if err != nil {
		t.Fatal(err)
	}
	if err := e.LoadROM(bin); err != nil {
		t.Fatal(err)
	}
	if err := e.RunFrames(2); err != nil {
		t.Fatal(err)
	}
	hit, err := e.WatchTimerWrap(3)
	if err != nil {
		t.Fatal(err)
	}
	return hit
}

// The claim: whether a wait is flagged does not depend on where in it the wrap falls, and
// what is flagged is a wait that missed the zero or a read long after it -- not the read
// that ends a wait for a negative INTIM, which is the wait that cannot miss the zero.
// Found by two agents building test tasks: one wrong answer of theirs (`lda INTIM / bpl`)
// was flagged under four input patterns and passed under a fifth.
func TestTimerWrapVerdictDoesNotDependOnPhase(t *testing.T) {
	waits := []struct {
		name, ticks, wait string
		flagged           bool
	}{
		{"`lda INTIM / bpl`, the wait for a negative value", "3", "Wt:     lda INTIM\n        bpl Wt", false},
		{"`sta WSYNC / lda INTIM / bpl`, the same polled once a line", "3", "Wt:     sta WSYNC\n        lda INTIM\n        bpl Wt", false},
		{"`lda INTIM / bne`, the wait for zero, which a 64-cycle tick cannot step over", "3", "Wt:     lda INTIM\n        bne Wt", false},
		{"INTIM read once as the time left, long after the timer ran out", "1",
			"        ldx #6\nBurn:   sta WSYNC\n        dex\n        bne Burn\n        lda INTIM", true},
	}
	for _, w := range waits {
		for pad := 0; pad <= 40; pad += 1 {
			hit := watchWait(t, w.ticks, w.wait, pad)
			if (hit != nil) != w.flagged {
				t.Errorf("%s, %d NOPs in front: flagged %v, want %v", w.name, pad, hit != nil, w.flagged)
				break
			}
		}
	}
}
