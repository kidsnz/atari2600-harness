package emu

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
)

// motionROM builds a kernel that draws one 8x15 player, optionally blinking, optionally drifting
// right by `hm` colour clocks a frame (an HMOVE nibble, $F0 = +1 through $80 = +8).
func motionROM(t *testing.T, blink bool, hm string) string {
	t.Helper()
	move, draw := "", "        lda #$FF"
	if hm != "" {
		move = "        lda #" + hm + "\n        sta HMP0\n        sta WSYNC\n        sta HMOVE"
	}
	if blink {
		draw = "        lda cnt\n        and #1\n        beq On\n        lda #0\n        jmp Set\nOn:     lda #$FF\nSet:"
	}
	clear := "        sta HMCLR\n"
	if hm != "" {
		clear = "" // let the motion accumulate instead of snapping back each frame
	}
	src := `        processor 6502
VSYNC=$00
VBLANK=$01
WSYNC=$02
COLUP0=$06
COLUBK=$09
RESP0=$10
GRP0=$1B
HMP0=$20
HMOVE=$2A
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
        sta WSYNC
        ldx #12
Pa:     dex
        bne Pa
        sta RESP0
Frame:  lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK
` + move + `
        ldx #34
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
` + draw + `
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
` + clear + `        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp Frame
        org $FFFC
        .word Start
        .word Start
`
	dir := t.TempDir()
	asm := filepath.Join(dir, "m.asm")
	if err := os.WriteFile(asm, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "m.bin")
	if out, err := build.Assemble(asm, bin); err != nil {
		t.Fatalf("blink=%v hm=%q did not assemble: %v\n%s", blink, hm, err, out)
	}
	return bin
}

// TestFlickerAreaChargesMotionMoreThanFlicker puts a number on the limitation `FlickerArea`'s own
// documentation states and never quantified: *"What it does NOT distinguish: movement from
// blinking."*
//
// The question is from 2002. Glenn Saunders, on multiplexed objects in motion: *"If you **move at
// certain speeds you will get a comb effect** because of the interlace. If you update the animation
// only 30 frames a second rather than 60 you'll be able to eliminate this… but at the cost of
// somewhat **jerkier animation**"* 〔`200208/msg00128`〕. So motion and flicker genuinely do interact
// on a screen — the metric is not wrong to charge for movement. What it cannot do is say which it is
// charging for, and by how much.
//
// Measured 2026-09-07 on one 8x15 player, everything else held still:
//
//	still                      0
//	blinking on/off          120   = 8 x 15, the whole object
//	drifting 1 px a frame     30   = 2 x 1 x 15
//	drifting 2 px a frame     60   = 2 x 2 x 15
//	drifting 4 px a frame    120   = 2 x 4 x 15   <- the same as a full blink
//	drifting 8 px a frame    240   = 2 x 8 x 15   <- TWICE a full blink
//
// ★**The law is 2·d·h, saturating at 2·w·h once the object clears its own width.** Two edges move,
// each d wide, on every line of the sprite; past d = w the old and new positions are disjoint and
// both are counted whole.
//
// ★★**So the metric charges ordinary motion more than actual flicker.** Four pixels a frame is a
// normal game speed and scores exactly as a sprite switching on and off; eight scores double. A
// `max_flicker_area` ceiling chosen from a flicker budget will fire on a sprite that never flickers,
// and the number will be right — it is the name that is misleading.
//
// ★★★**What to do with it:** compare like with like. A ceiling is meaningful across frames where the
// motion is the same, or on a scene held still. Reading one absolute number and calling it "how much
// this flickers" is the reading this test exists to prevent.
//
// Found by the mailing-list distillation (helper-1).
func TestFlickerAreaChargesMotionMoreThanFlicker(t *testing.T) {
	area := func(bin string) int {
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
		max := 0
		for i := 0; i < 8; i++ {
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
		}
		return max
	}

	still := area(motionROM(t, false, ""))
	blink := area(motionROM(t, true, ""))
	if still != 0 {
		t.Fatalf("a motionless, non-blinking sprite scores %d, want 0 — every number below is "+
			"relative to that zero", still)
	}
	if blink == 0 {
		t.Fatal("a blinking sprite scores 0; the metric is not responding and nothing below means " +
			"anything")
	}

	// The law: 2 * d * h, saturating once the object clears its own width.
	for _, c := range []struct {
		hm   string
		d    int
		want int
	}{
		{"$F0", 1, 30},
		{"$E0", 2, 60},
		{"$C0", 4, 120},
		{"$80", 8, 240},
	} {
		got := area(motionROM(t, false, c.hm))
		if got != c.want {
			t.Errorf("drifting %d px a frame scores %d, want %d (2 x %d x 15). The relation is the "+
				"finding; a single point would not be one", c.d, got, c.want, c.d)
		}
	}

	// And the reading that matters: motion overtakes flicker at four pixels a frame.
	if area(motionROM(t, false, "$C0")) != blink {
		t.Errorf("drifting 4 px a frame no longer scores the same as a full blink (%d). That equality "+
			"is the whole warning — an ordinary speed and an on/off sprite are indistinguishable to "+
			"this number", blink)
	}
	if area(motionROM(t, false, "$80")) <= blink {
		t.Error("drifting 8 px a frame no longer scores MORE than a full blink; past its own width " +
			"the object's two positions are disjoint and should both be counted")
	}
}
