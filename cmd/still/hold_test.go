package main

import (
	"image"
	"os"
	"path/filepath"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
)

// holdROM is built here, so the test says what it needs: a picture that answers to the
// buttons. Every frame the background is blue, red while fire is down, and the left
// half of the playfield is lit only while the stick is pushed left. A playfield stripe
// on the right is always there, so no frame is uniform (the blank-frame guard).
const holdROM = `
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUPF  = $08
COLUBK  = $09
PF0     = $0D
PF1     = $0E
PF2     = $0F
INPT4   = $3C
SWCHA   = $0280
        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
Frame:  lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ldx #37
Vb:     sta WSYNC
        dex
        bne Vb
        lda #$0E
        sta COLUPF
        ldy #$84            ; blue
        bit INPT4
        bmi Up              ; bit 7 set = fire not pressed
        ldy #$44            ; red
Up:     sty COLUBK
        ldy #0
        lda SWCHA
        and #%01000000      ; bit 6 clear = left pushed
        bne NoLeft
        ldy #$FF
NoLeft: sty PF1
        lda #$F0
        sta PF0
        lda #0
        sta VBLANK
        ldx #192
Pic:    sta WSYNC
        dex
        bne Pic
        lda #2
        sta VBLANK
        ldx #30
Os:     sta WSYNC
        dex
        bne Os
        jmp Frame
        org $FFFC
        .word Start
        .word Start
`

func buildHoldROM(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	asm, bin := filepath.Join(dir, "hold.asm"), filepath.Join(dir, "hold.bin")
	if err := os.WriteFile(asm, []byte(holdROM), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := build.Assemble(asm, bin); err != nil {
		t.Skipf("dasm is not available, nothing can be said about -hold here: %v\n%s", err, out)
	}
	return bin
}

// What the two pixels read: (150, 100) is under PF2, which is never lit, so it is
// background on every frame; (40, 100) is under PF1, lit only while the stick is left.
// (The playfield is not reflected: PF1 also covers x = 96..127, so a background probe
// there reads the playfield whenever the stick is left. The first version of this test
// had it at 120.)
func probe(t *testing.T, img *image.RGBA) (bgRed, pfLit bool) {
	t.Helper()
	bg := img.RGBAAt(150, bandTop+100)
	pf := img.RGBAAt(40, bandTop+100)
	return bg.R > bg.B, pf != bg
}

// The claim: a button named in -hold is down while the ROM runs, from the frame it
// says. Each line of the table is a control for the others -- the same ROM and frame
// with nothing held, with the other button, and with a hold that has not begun yet.
func TestHoldPressesTheButtonItNamesFromTheFrameItNames(t *testing.T) {
	rom := buildHoldROM(t)
	cases := []struct {
		spec             string
		frame            int
		wantRed, wantLit bool
	}{
		{"", 20, false, false},
		{"fire", 20, true, false},
		{"left", 20, false, true},
		{"fire,left", 20, true, true},
		{"fire@30", 20, false, false}, // not yet
		{"fire@30", 40, true, false},
		{"fire@30,left@35", 33, true, false},
	}
	for _, c := range cases {
		holds, err := parseHold(c.spec)
		if err != nil {
			t.Fatalf("-hold %q: %v", c.spec, err)
		}
		img, err := grabFrame(rom, c.frame, holds...)
		if err != nil {
			t.Fatalf("-hold %q frame %d: %v", c.spec, c.frame, err)
		}
		red, lit := probe(t, img)
		if red != c.wantRed || lit != c.wantLit {
			t.Errorf("-hold %q at frame %d: background red %v (want %v), left playfield lit %v (want %v)",
				c.spec, c.frame, red, c.wantRed, lit, c.wantLit)
		}
	}
}

func TestHoldRefusesWhatItDoesNotKnow(t *testing.T) {
	for _, bad := range []string{"jump", "fire@0", "fire@soon", "left@-3"} {
		if _, err := parseHold(bad); err == nil {
			t.Errorf("-hold %q was accepted", bad)
		}
	}
	if h, err := parseHold(" fire , left@60 "); err != nil || len(h) != 2 || h[1].from != 60 || h[0].from != 1 {
		t.Errorf("-hold with spaces: %v %v", h, err)
	}
}
