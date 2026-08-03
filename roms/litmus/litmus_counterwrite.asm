; litmus_counterwrite — only the decrement may touch the counter.
;
; `determineBound` reads "entry value v, therefore v iterations". That needs the loop
; body's net effect on the counter to be exactly -1, and the analysis only latched a
; BOOLEAN when it saw a `dex`/`dey` — so a body that also wrote the register was
; indistinguishable from one that did not.
;
; Measured before the fix, on this ROM's DangerRow: proven **22** cycles against a
; machine that spends **2290 across 31 scanlines**, reported as `certified: true` and
; `roll_free: true`. **104x under**, the largest gap found in this package. Two
; increments and one decrement leave the counter RISING, so the loop never terminates
; on it at all; the analysis answered 2 iterations.
;
; This is the other half of the SD-13 repair (`litmus_latchflags`). That one added
; `preservesZN` to guard the window AFTER the decrement, where a compare substitutes
; its own condition for the counter's. The window BEFORE it was left open, and it is
; the worse of the two: a write there changes the COUNT, not merely which flags the
; latch reads.
;
; THE CONTROLS, and each rules out a different wrong repair:
;
;   PlainCtl  `nop nop dex bne`  — refusing every counted loop would "fix" the danger
;   StoreCtl  `dex stx bne`      — a store writes MEMORY, not the counter, and must
;                                  stay bounded; a repair keyed on "anything between
;                                  the decrement and the latch" breaks it, and
;                                  Chopper Command $F39D is a real loop of that shape
;   OtherCtl  `dex` with `iny`   — the OTHER index register moving is not the
;                                  counter moving; refusing it would cost precision
;                                  on any loop that walks two pointers
;
; Without OtherCtl the cheapest repair — "refuse if any index register is written" —
; passes, and it would refuse sound loops that are common in real kernels.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

scratch = $80

        org $F000

Reset:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

Main:
; --- VSYNC: 3 lines ---
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: 37 lines ---
        lda #2
        sta VBLANK
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

; --- Visible ---
; The defect: the body writes the counter ahead of the decrement, so the net step is
; +1 and the loop runs away. Must be REFUSED.
DangerRow:
        sta WSYNC
        ldx #$02
DangerL:
        inx
        inx
        dex
        bne DangerL

; Control 1 — the same body cost with the counter untouched. Must stay EXACT.
PlainCtl:
        sta WSYNC
        ldx #$02
PlainL: nop
        nop
        dex
        bne PlainL

; Control 2 — a store between the decrement and the latch. Writes memory, not the
; counter, and preserves the flags. Must stay EXACT.
StoreCtl:
        sta WSYNC
        ldx #$03
StoreL: lda scratch,x
        dex
        stx scratch
        bne StoreL

; Control 3 — the OTHER index register moves. Y is not the counter, so this loop is
; exactly as boundable as it ever was. Must stay EXACT.
OtherCtl:
        sta WSYNC
        ldy #$00
        ldx #$03
OtherL: iny
        dex
        bne OtherL

        lda #0
        sta COLUBK
        ldx #145
Fill:   sta WSYNC
        dex
        bne Fill

; --- Overscan: 30 lines ---
        sta WSYNC
        lda #2
        sta VBLANK
        ldx #29
OS:     sta WSYNC
        dex
        bne OS

        jmp Main

        org $FFFC
        .word Reset
        .word Reset
