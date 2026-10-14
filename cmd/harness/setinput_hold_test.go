package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kidsnz/atari2600-harness/internal/build"
	"github.com/kidsnz/atari2600-harness/internal/emu"
)

// heldROM counts, in RAM $80, the frames on which the fire button of the left joystick
// is down. Built here so the test says what it needs.
const heldROM = `
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
INPT4   = $3C
SWCHA   = $0280
Held    = $80
Left    = $81
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
        bit INPT4
        bmi Up              ; bit 7 set = not pressed
        inc Held
Up:     lda SWCHA
        and #%01000000      ; bit 6 clear = left pushed
        bne NoLeft
        inc Left
NoLeft: ldx #37
Vb:     sta WSYNC
        dex
        bne Vb
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

// The claim is the tool's own description: "pressed=true holds". It did not. The liveness
// probe that runs on a held direction ends by releasing what it pressed, so that a caller
// who goes on measuring is not left holding something -- and the caller here is the one
// who had just asked for it to be held. set_input fire, then ten frames, counted none.
// Found by an agent that needed a held button for a picture check and had to write its own
// program to get one.
func TestSetInputHoldsWhatItSaysItHolds(t *testing.T) {
	dir := t.TempDir()
	asm, bin := filepath.Join(dir, "held.asm"), filepath.Join(dir, "held.bin")
	if err := os.WriteFile(asm, []byte(heldROM), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := build.Assemble(asm, bin); err != nil {
		t.Skipf("dasm is not available, nothing can be said about the hold here: %v\n%s", err, out)
	}
	mu.Lock()
	e, err := emu.New("NTSC")
	if err == nil {
		err = e.LoadROM(bin)
	}
	if err != nil {
		mu.Unlock()
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		e.StepFrame()
	}
	current = e
	mu.Unlock()

	held := func(frames int) int {
		mu.Lock()
		defer mu.Unlock()
		before, _ := e.CurrentRAM()
		for i := 0; i < frames; i++ {
			e.StepFrame()
		}
		after, _ := e.CurrentRAM()
		return int(after[0]) - int(before[0])
	}
	ctx := context.Background()
	if n := held(10); n != 0 {
		t.Fatalf("nothing pressed, and the ROM counted %d frames with fire down", n)
	}
	_, out, err := handleSetInput(ctx, nil, SetInputIn{Action: "fire", Pressed: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.Responds == nil || !*out.Responds {
		t.Errorf("the ROM counts frames with fire down, and the liveness verdict was %v", out.Liveness)
	}
	if n := held(10); n != 10 {
		t.Errorf("after set_input fire pressed=true, fire was down on %d of the next 10 frames, want 10", n)
	}
	if _, _, err := handleSetInput(ctx, nil, SetInputIn{Action: "fire", Pressed: false}); err != nil {
		t.Fatal(err)
	}
	if n := held(10); n != 0 {
		t.Errorf("after the release, fire was still down on %d of 10 frames", n)
	}
}

// Held inputs add up. The liveness probe that a press runs let go of everything when it
// finished, and only the input just asked for was pressed again, so fire pressed while left
// was held let go of left -- reported by two agents who wanted fire and a direction at once
// and tested that through the scenario runner instead.
func TestSetInputKeepsWhatWasAlreadyHeld(t *testing.T) {
	dir := t.TempDir()
	asm, bin := filepath.Join(dir, "held.asm"), filepath.Join(dir, "held.bin")
	if err := os.WriteFile(asm, []byte(heldROM), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := build.Assemble(asm, bin); err != nil {
		t.Skipf("dasm is not available: %v\n%s", err, out)
	}
	ctx := context.Background()
	if _, _, err := handleLoadROM(ctx, nil, LoadROMIn{Path: bin}); err != nil {
		t.Fatal(err)
	}
	count := func(frames int) (fire, left int) {
		mu.Lock()
		defer mu.Unlock()
		before, _ := current.CurrentRAM()
		for i := 0; i < frames; i++ {
			current.StepFrame()
		}
		after, _ := current.CurrentRAM()
		return int(after[0]) - int(before[0]), int(after[1]) - int(before[1])
	}
	count(5)
	for _, a := range []string{"left", "fire"} {
		if _, _, err := handleSetInput(ctx, nil, SetInputIn{Action: a, Pressed: true}); err != nil {
			t.Fatal(err)
		}
	}
	if fire, left := count(10); fire != 10 || left != 10 {
		t.Errorf("left, then fire, pressed: over 10 frames fire was down on %d and left on %d, want 10 and 10", fire, left)
	}
	handleSetInput(ctx, nil, SetInputIn{Action: "fire", Pressed: false})
	if fire, left := count(10); fire != 0 || left != 10 {
		t.Errorf("fire released, left still held: fire on %d, left on %d of 10 frames, want 0 and 10", fire, left)
	}
	if _, _, err := handleLoadROM(ctx, nil, LoadROMIn{Path: bin}); err != nil {
		t.Fatal(err)
	}
	handleSetInput(ctx, nil, SetInputIn{Action: "fire", Pressed: true})
	if _, left := count(10); left != 0 {
		t.Errorf("a new ROM loaded: left, held on the old machine, was down on %d of 10 frames", left)
	}
}
