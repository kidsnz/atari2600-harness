package emu

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
)

// dutyROM builds a ROM that shows one 8x16 player for `on` frames out of every `period`, so two
// duty ratios can be compared without committing a fixture for each.
func dutyROM(t *testing.T, on, period int) string {
	t.Helper()
	decide := "        lda cnt\n        cmp #" + strconv.Itoa(on) +
		"\n        bcc On\n        lda #0\n        jmp Set\nOn:     lda #$FF\nSet:"
	src := `        processor 6502
VSYNC=$00
VBLANK=$01
WSYNC=$02
COLUP0=$06
COLUBK=$09
RESP0=$10
GRP0=$1B
HMCLR=$2B
cnt     = $80
        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        sta HMCLR
        lda #$0E
        sta COLUP0
        lda #0
        sta COLUBK
Frame:  lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK
        sta WSYNC
        ldx #10
Pa:     dex
        bne Pa
        sta RESP0
        ldx #36
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
` + decide + `
        ldy #16
Draw:   sta WSYNC
        sta GRP0
        dey
        bne Draw
        lda #0
        sta GRP0
        ldx #176
Vis:    sta WSYNC
        dex
        bne Vis
        lda #2
        sta VBLANK
        inc cnt
        lda cnt
        cmp #` + strconv.Itoa(period) + `
        bcc NoWrap
        lda #0
        sta cnt
NoWrap:
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp Frame
        org $FFFC
        .word Start
        .word Start
`
	dir := t.TempDir()
	asm := filepath.Join(dir, "d.asm")
	if err := os.WriteFile(asm, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "d.bin")
	if out, err := build.Assemble(asm, bin); err != nil {
		t.Fatalf("duty %d/%d did not assemble: %v\n%s", on, period, err, out)
	}
	return bin
}

// TestMaxFlickerAreaCannotSeeDutyRatio measures a limit of the number scenarios gate on, against a
// 2003 claim that a LONGER flicker cycle looks CALMER.
//
// Andrew Davie, proposing a three-frame Fuji over a two-frame one: *"**three frames**, first frame
// with the outer bars, second with the left two, third with the right two. It gives you an
// **11-pixel Fuji**… This one is **less 'flickery' in my opinion, because any of the bars is
// displayed two out of every 3 frames**"* 〔`200309/msg00154`〕. The period went up and the flicker
// went down, because what the eye tracks is the **duty ratio** — 2/3 rather than 1/2.
//
// `FlickerArea` compares two consecutive frames, and `max_flicker_area` gates on the worst pair.
// Measured 2026-09-07 on two ROMs identical but for their duty:
//
//	shown 1 frame of every 2    max 120   mean 120.0   0 of 12 pairs unchanged
//	shown 2 frames of every 3   max 120   mean  80.0   4 of 12 pairs unchanged
//
// ★**The maxima are equal.** A gate written as `max_flicker_area` ranks Davie's calmer design exactly
// level with the one he was replacing, because the worst pair is the same in both — the difference is
// entirely in **how often** that pair occurs. The mean carries it: 80/120 is 2/3, the duty ratio
// itself.
//
// This is not an argument for changing the gate. A ceiling on the worst pair is the right shape for
// "no single transition may be too violent", and the archive's own threshold is about area
// 〔`200108/msg00315`〕. It is an argument for knowing what the number cannot say: **a design that
// flickers less often, but just as hard when it does, is invisible to it.** Choose the duty by eye,
// then let the gate hold the worst case.
//
// Found by the mailing-list distillation (helper-2).
func TestMaxFlickerAreaCannotSeeDutyRatio(t *testing.T) {
	measure := func(bin string) (max int, mean float64, unchanged int) {
		e, err := New("NTSC")
		if err != nil {
			t.Fatal(err)
		}
		if err := e.LoadROM(bin); err != nil {
			t.Fatal(err)
		}
		if err := e.RunFrames(6); err != nil {
			t.Fatal(err)
		}
		sum := 0
		const pairs = 12
		for i := 0; i < pairs; i++ {
			if _, err := e.StepFrame(); err != nil {
				t.Fatal(err)
			}
			a, err := e.FlickerArea()
			if err != nil {
				t.Fatal(err)
			}
			if a > max {
				max = a
			}
			if a == 0 {
				unchanged++
			}
			sum += a
		}
		return max, float64(sum) / pairs, unchanged
	}

	max2, mean2, still2 := measure(dutyROM(t, 1, 2))
	max3, mean3, still3 := measure(dutyROM(t, 2, 3))

	if max2 == 0 || max3 == 0 {
		t.Fatal("one of the ROMs never flickers — the comparison below is between nothing and nothing")
	}
	if max2 != max3 {
		t.Errorf("the worst frame pair differs between the two duties (%d and %d). They are the same "+
			"sprite appearing and disappearing, so the worst transition should be identical; if it "+
			"is not, the ROMs are no longer differing only in duty", max2, max3)
	}
	if !(mean3 < mean2) {
		t.Errorf("mean flicker area is %.1f at duty 2/3 and %.1f at duty 1/2 — the longer cycle is "+
			"supposed to change less often, and if it does not the whole point of Davie's three-frame "+
			"design is absent from this measurement", mean3, mean2)
	}
	if still2 != 0 {
		t.Errorf("%d of the 1/2-duty pairs were unchanged, want 0 — that duty alternates every "+
			"frame by construction", still2)
	}
	if still3 == 0 {
		t.Error("no 2/3-duty pair was unchanged. The frames where the sprite stays on are what make " +
			"the mean fall, and without them this test is comparing two identical things")
	}
	// The relation worth remembering, stated as a check rather than a comment: the mean tracks the
	// duty, so mean/max recovers how often the picture changes.
	if r := mean3 / float64(max3); r < 0.6 || r > 0.72 {
		t.Errorf("mean/max at duty 2/3 is %.3f, want about 0.667 (two of every three pairs differ). "+
			"That ratio is the duty the eye responds to, and it is the quantity max_flicker_area "+
			"discards", r)
	}
}
