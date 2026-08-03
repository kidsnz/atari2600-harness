; litmus_callinloop — a `jsr` inside a folded loop body is costed at 6 cycles.
;
; `foldLoops` walks the body with `in.nextSite()` and sums `in.nodeCost()`. For a JSR
; that means the RETURN address and six cycles: **the callee's cycles are dropped, once
; per iteration.** `IsBranch()` does not catch it — that is `AddressingMode == Relative
; && Effect == Flow`, and a JSR is Absolute/Subroutine — so nothing refused it.
;
; Worse than the arithmetic: if the callee contains a `sta WSYNC`, the walk steps over
; a REGION BOUNDARY. The machine's interval ends at that strobe; the proof's does not.
; The two numbers then describe different intervals and comparing them means nothing,
; in either direction.
;
; THE LIVE INSTANCE. `roms/techniques/shared_setxpos.asm` region $F054 is exactly this:
;
;       PosLoop:  lda ... ; jsr SetXPos ; dex ; bpl PosLoop
;       SetXPos:  sec ; sta WSYNC ; ...
;
; Measured: **proven 98, machine 36.** It is sound only by accident — the machine's
; interval ends at the callee's first WSYNC after 36 cycles, so the fold's inflated
; number happens to sit above it. Lengthen the callee's pre-WSYNC prefix, or remove the
; WSYNC, and the same structure inverts.
;
; DangerRow is that inversion, made explicit: a callee of twelve `nop`s and no WSYNC.
; Measured before the fix: **proven 48, machine 168 across 3 scanlines — 3.5x under**,
; with `certified: true`.
;
; THE CONTROLS:
;
;   InlineCtl  the same work written inline instead of called. Must stay bounded and
;              EXACT — the repair must refuse CALLS, not loops.
;   StoreCtl   a body full of ordinary memory work. Must stay bounded and EXACT, so the
;              repair cannot be bought by refusing everything that is not a register op.
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
; The defect: the callee's 12 nops are dropped from every iteration's cost.
DangerRow:
        sta WSYNC
        ldx #$04
DangerL:
        jsr Burn
        dex
        bne DangerL

; Control 1 — the same work inline. Must stay bounded and EXACT.
InlineCtl:
        sta WSYNC
        ldx #$02
InlineL:
        nop
        nop
        nop
        dex
        bne InlineL

; Control 2 — ordinary memory work in the body. Must stay bounded and EXACT.
StoreCtl:
        sta WSYNC
        ldx #$03
StoreL: lda scratch,x
        sta scratch
        dex
        bne StoreL

        lda #0
        sta COLUBK
        ldx #140
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

; The callee whose cycles the fold drops. Deliberately WITHOUT a WSYNC, so the region
; boundary is not moved and the two numbers describe the SAME interval — which is what
; makes the comparison meaningful rather than a category error.
Burn:   nop
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        rts

        org $FFFC
        .word Reset
        .word Reset
