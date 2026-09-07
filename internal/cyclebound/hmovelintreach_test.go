package cyclebound

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hmoveKernel builds a minimal ROM whose kernel loop body is `body`, so the lint can be aimed at one
// shape at a time without committing a fixture ROM per shape.
func hmoveKernel(t *testing.T, body string) string {
	t.Helper()
	src := `        processor 6502
VSYNC=$00
VBLANK=$01
WSYNC=$02
RESP0=$10
GRP0=$1B
HMP0=$20
HMOVE=$2A
HMCLR=$2B
        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #$FF
        sta GRP0
Frame:  lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        ldy #191
` + body + `
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp Frame
        org $FFFC
        .word Start
        .word Start
`
	p := filepath.Join(t.TempDir(), "k.asm")
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func hmoveHazards(t *testing.T, body string) int {
	t.Helper()
	r, err := LintDetail(hmoveKernel(t, body))
	if err != nil {
		t.Fatalf("lint: %v", err)
	}
	n := 0
	for _, w := range r.Warnings {
		if strings.Contains(w.Rule, "hmove-hazard") {
			n++
		}
	}
	return n
}

// TestHMOVEHazardLintReachesStraightLinesAndNotBackEdges measures how far the R3 hazard rule can see,
// and answers a question about a technique it was never aimed at.
//
// The rule looks for an HMxx/HMCLR write within 24 CPU cycles **after** an HMOVE strobe. A 2003
// technique writes them the other way round — HMCLR first, HMOVE at cycle 74 — and the worry was
// that the lint would flag it. Measured 2026-09-07, three shapes:
//
//	HMCLR then a late HMOVE, with a WSYNC in the loop      0 warnings   no false positive
//	HMOVE then HMCLR on the next instruction               1 warning    the rule works
//	a loop with NO WSYNC, HMCLR ~8 cycles after HMOVE      0 warnings   ★ not seen
//	via the back edge
//
// ★**So the answer to the original question is "no, it does not misfire", and the reason is worth
// more than the answer.** The scan stops at any branch, jump or WSYNC. Stopping at a WSYNC is sound —
// the wait itself clears the 24-cycle window. Stopping at a **branch** means the rule never follows a
// loop back to its own top, and a kernel loop is exactly where HMOVE lives. The third case above is a
// real hazard that the lint reports nothing about.
//
// That is a bound on the rule, not a bug in it: following back edges is the difference between a
// peephole and the abstract interpreter the rest of this package runs, and `prove_line_budget` already
// does the latter for cycles. It is recorded so that "the timing lint is quiet" is not read as "there
// is no HMOVE hazard here".
//
// Raised by the mailing-list distillation (helper-2), who asked only about the false positive.
func TestHMOVEHazardLintReachesStraightLinesAndNotBackEdges(t *testing.T) {
	// 1. The 2003 order, with a WSYNC in the loop. Must be quiet.
	reversed := `Krow:   sta WSYNC
        sta HMCLR
        lda #$80
        sta HMP0
        ldx #21
Wait:   dex
        bne Wait
        sta HMOVE
        dey
        bpl Krow`
	if n := hmoveHazards(t, reversed); n != 0 {
		t.Errorf("HMCLR-then-late-HMOVE raised %d hazard warning(s); the rule is about writes AFTER "+
			"an HMOVE and this order has none, so flagging it would push authors away from a "+
			"technique that is fine", n)
	}

	// 2. The straight-line hazard. Must fire, or case 1 proves nothing.
	straight := `Krow:   sta WSYNC
        lda #$80
        sta HMP0
        sta HMOVE
        sta HMCLR
        dey
        bpl Krow`
	if n := hmoveHazards(t, straight); n == 0 {
		t.Fatal("an HMCLR on the instruction after HMOVE raised no warning — the rule is not " +
			"firing at all, and the quiet result above is meaningless")
	}

	// 3. The same hazard carried across a back edge, with no WSYNC to clear the window.
	// This is documented as NOT seen; if it ever is, the note above needs rewriting.
	backEdge := `Krow:   sta HMCLR
        lda #$80
        sta HMP0
        sta HMOVE
        dey
        bpl Krow`
	if n := hmoveHazards(t, backEdge); n != 0 {
		t.Errorf("the back-edge hazard now raises %d warning(s). That is an improvement, and it "+
			"means the comment on this test — which says the scan stops at a branch — is out of "+
			"date and should be rewritten rather than left describing a limit that is gone", n)
	}
}
