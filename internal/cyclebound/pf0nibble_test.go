package cyclebound

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pf0ROM builds a playfield kernel whose PF0 comes from RAM one of four ways.
func pf0ROM(t *testing.T, variant string) string {
	t.Helper()
	var body, extraSym, extraTail string
	lines := "        ldy #31\n"
	switch variant {
	case "unpacked": // one byte per line, high nibble already in place
		body = "        lda buf,y\n        sta PF0"
	case "packed": // two lines per byte, parity tested at run time
		body = `        tya
        lsr
        tax
        lda buf,x
        tya
        and #1
        beq Hi
        asl
        asl
        asl
        asl
Hi:     sta PF0`
	case "unrolled_plain": // two lines per iteration, still one byte per line
		lines = "        ldy #15\n"
		extraSym = "buf2    = $B0\n"
		body = "        lda buf,y\n        sta PF0\n        sta WSYNC\n        lda buf2,y\n        sta PF0"
	case "unrolled": // two lines per iteration, packed, odd nibble via a table
		lines = "        ldy #15\n"
		extraTail = "LoTbl:  ds 256, 0\n"
		body = "        lda buf,y\n        sta PF0\n        sta WSYNC\n        lda buf,y\n" +
			"        tax\n        lda LoTbl,x\n        sta PF0"
	default:
		t.Fatalf("unknown variant %q", variant)
	}
	src := `        processor 6502
VSYNC=$00
VBLANK=$01
WSYNC=$02
COLUBK=$09
COLUPF=$08
PF0=$0D
buf     = $90
` + extraSym + `        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #$46
        sta COLUPF
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
` + lines + `Krow:   sta WSYNC
` + body + `
        dey
        bpl Krow
        lda #2
        sta VBLANK
        ldx #160
OS:     sta WSYNC
        dex
        bne OS
        jmp Frame
` + extraTail + `        org $FFFC
        .word Start
        .word Start
`
	p := filepath.Join(t.TempDir(), "p.asm")
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// krowTotal sums every Krow region, so a kernel unrolled across two lines is compared against a
// single-line one by the work per ITERATION rather than per region.
func krowTotal(t *testing.T, asm string) int {
	t.Helper()
	r, err := Prove(asm, 76)
	if err != nil {
		t.Fatalf("prove: %v", err)
	}
	total, found := 0, false
	for _, l := range r.Lines {
		if strings.HasPrefix(l.StartLoc, "Krow") {
			total += l.Worst
			found = true
		}
	}
	if !found {
		t.Fatal("no Krow region in the report — the generated kernel changed shape")
	}
	return total
}

// TestPackingPF0NibblesCostsThreeCyclesOrNineteen prices the option between the two this repository
// already documents.
//
// `design-principles.md` has the extreme — *"not drawing PF0 … frees 12cy per line + 18 bytes of
// RAM"* — and `kernel-micro-idioms.md` has nibble packing as a general idiom. The middle, **packing a
// two-line pair's PF0 into one byte**, had no line anywhere. Ben Larson was doing it in 2002 and
// finding it awkward: *"The fact that I'm **packing both PF0 nibbles into one byte** probably doesn't
// help the situation"* 〔`200210/msg00045`〕.
//
// Measured 2026-09-07 over four kernels, the last two unrolled across two scanlines:
//
//	one line per iteration, one byte per line      22 cy per line
//	one line per iteration, packed, parity tested  41 cy per line          +19
//	two lines per iteration, one byte per line     11 + 22 = 33 per pair
//	two lines per iteration, packed, table         11 + 28 = 39 per pair   +6, so +3 per line
//
// ★**The same idea costs +19 or +3 depending on how it is written** — a factor of six between the
// obvious implementation and the unrolled one, because unrolling removes the parity test entirely:
// the two halves of the pair know which nibble they are.
//
// ★★**The unrolled control is what makes that readable.** The first version of this measurement
// compared a single-line unpacked kernel against an unrolled packed one and made packing look
// *cheaper* than not packing — the saving was the loop overhead halving, and it had nothing to do
// with nibbles. Both unrolled variants are here so the packing is the only difference between them.
//
// ★★★**What it buys and what it costs:** 16 bytes of RAM for a 32-line band, +3 cycles a line, and
// 256 bytes of ROM for the shift table. Against the extreme — abandoning PF0 *gains* 12 cycles a line
// and 18 bytes — so packing is only the right answer when PF0's content is actually needed.
//
// Found by the mailing-list distillation (helper-1).
func TestPackingPF0NibblesCostsThreeCyclesOrNineteen(t *testing.T) {
	unpacked := krowTotal(t, pf0ROM(t, "unpacked"))
	packed := krowTotal(t, pf0ROM(t, "packed"))
	plain2 := krowTotal(t, pf0ROM(t, "unrolled_plain"))
	packed2 := krowTotal(t, pf0ROM(t, "unrolled"))

	if d := packed - unpacked; d != 19 {
		t.Errorf("the naive packing costs %d cycles per line (%d then %d), want 19", d, unpacked, packed)
	}
	if d := packed2 - plain2; d != 6 {
		t.Errorf("the unrolled packing costs %d cycles per two lines (%d then %d), want 6 — three a "+
			"line", d, plain2, packed2)
	}
	// The point of the whole measurement: the two ways of writing it are far apart.
	if (packed - unpacked) <= (packed2-plain2)*2 {
		t.Errorf("the naive packing (+%d per line) is no worse than the unrolled one (+%d per two "+
			"lines). The finding is that the same idea costs six times more written the obvious way; "+
			"if that gap has closed, the note above is wrong", packed-unpacked, packed2-plain2)
	}
	// And the control that makes the second row readable: unrolling alone must be cheaper than not
	// unrolling, independently of any packing.
	if plain2 >= unpacked*2 {
		t.Errorf("unrolling two lines costs %d per pair against %d for two single-line iterations. "+
			"Unrolling is supposed to save the loop overhead; if it does not, the packed-versus-plain "+
			"comparison above is measuring something else", plain2, unpacked*2)
	}
}
