; litmus_jsrentry — a subroutine between the counter's load and the loop.
;
; `determineBound` reads the counter's entry value from the header's predecessors. It
; computed that value with `State.transfer`, which models what an INSTRUCTION does to
; the machine — and for a JSR that is only the push, leaving X and Y untouched.
;
; The state that actually flows along the edge is `absSuccessors`', and it resets a
; JSR's return point to Top precisely because the callee's effect is not modelled.
; **Two functions in the same package, disagreeing about the same edge.** The scan was
; reading the counter from BEFORE the call and treating it as the value at the header.
;
; Measured on DangerRow before the fix: `ldx #$02 / jsr SetBig` where `SetBig` does
; `ldx #$50`. The scan saw X=2 and answered **36** cycles; the machine spent **738
; across 10 scanlines**. 20.5x under, with `certified: true` and `roll_free: true`.
;
; The repair takes the state from the edge rather than from `transfer`, which DELETES
; the divergence instead of adding a rule — the same argument `successors` makes about
; having one notion of a successor. A JSR predecessor now yields X.Top and the existing
; "unknown entry value" refusal fires.
;
; THE CONTROLS:
;
;   SafeCtl   the same `jsr` between the load and the loop, but the callee provably
;             does NOT touch X. Today this is refused too, because the analysis has no
;             callee summary and Top is the honest answer for an unmodelled call.
;             It is here so that the refusal is a MEASURED consequence rather than an
;             assumption: if a callee-effect summary is ever added, this row is the
;             one that should become bounded, and the test says so.
;   PlainCtl  no call at all. Must stay bounded and EXACT — the repair must key on the
;             edge state, not on the mere presence of a subroutine anywhere.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

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
; The defect: the callee replaces the counter, and the pre-call value is 2.
DangerRow:
        sta WSYNC
        ldx #$02
        jsr SetBig
DangerL:
        nop
        nop
        dex
        bne DangerL

; Control 1 — the callee leaves X alone. Refused today for want of a callee summary;
; the test asserts that, so the refusal is measured rather than assumed.
SafeCtl:
        sta WSYNC
        ldx #$03
        jsr NoTouch
SafeL:  nop
        nop
        dex
        bne SafeL

; Control 2 — no call. Must stay bounded and EXACT.
PlainCtl:
        sta WSYNC
        ldx #$03
PlainL: nop
        nop
        dex
        bne PlainL

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

; The callee that rewrites the counter. $50 = 80 iterations, against the 2 the
; pre-call state would suggest.
SetBig: ldx #$50
        rts

; The callee that does not. It writes memory instead, so a future callee-effect
; summary would have something true to say about it.
NoTouch:
        lda #$00
        sta $80
        rts

        org $FFFC
        .word Reset
        .word Reset
