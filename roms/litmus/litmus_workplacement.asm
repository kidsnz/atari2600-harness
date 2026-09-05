; litmus_workplacement — WHERE in the frame you put non-kernel work, and what it costs you.
;
; `prove_line_budget` proves the worst case over all paths WITHIN A SCANLINE. Nothing here says
; anything about the frame: a 262-line NTSC frame has **37 VBLANK lines before the picture and 30
; overscan lines after it** — 67 lines of non-kernel time — and which of the two you use has never
; been written down as making any difference.
;
; The list found that it does. Andrew Davie, 200103/msg00056, on a game losing vertical sync:
;
;	the amount of time required to move and draw all the cubes is > the available cycles that I
;	can provide … Cubes are moved every 2nd frame, so they are also drawn every 2nd frame.
;	**I moved the routine from the overscan to the vertical bl[ank]**
;
; Two different fixes in one report — *do the work half as often* (spend less) and *do the same work
; somewhere else* (spend it elsewhere) — and only the first is a budget question.
;
; **What this ROM measures.** A player moves one pixel right per frame. The move is computed either
; in VBLANK (before the picture) or in overscan (after it), selected by RAM $80, and the position
; actually DRAWN is recorded. If the phase matters, the two differ by exactly one frame of motion:
; overscan computes a value the beam has already passed, so it shows up on the NEXT frame.
;
; That is worth a fixture because the consequence is invisible in a still: both versions animate
; smoothly, at the same speed, and one of them is a frame behind the input that caused it. On a
; game that reads a stick and moves an object, that is the difference between a control that feels
; attached and one that does not — and it costs nothing to fix, because the work is the same work.
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
COLUP0  = $06
GRP0    = $1B
RESP0   = $10
HMP0    = $20
HMOVE   = $2A
HMCLR   = $2B
phase    = $80      ; 0 = compute in VBLANK, 1 = compute in overscan (the test pokes this)
xpos     = $81      ; the value the kernel will use
drawn    = $82      ; the value the kernel actually used this frame
frames   = $83

        org $F000
Start:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #$0E
        sta COLUP0
        lda #0
        sta COLUBK
        sta phase           ; default: VBLANK

NextFrame:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

        ; ---- VBLANK phase ----
        lda phase
        bne VBSkip          ; phase=1 -> the work happens in overscan instead
        inc xpos            ; the "work": advance the object
VBSkip:
        ; the kernel commits to whatever xpos holds RIGHT NOW
        lda xpos
        sta drawn

        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; ---- picture: draw one line of the object so the frame is not blank ----
        ldx #192
Vis:    sta WSYNC
        cpx #100
        bne NoDraw
        lda #$FF
        sta GRP0
        jmp VisNext
NoDraw: lda #0
        sta GRP0
VisNext:
        dex
        bne Vis

        lda #2
        sta VBLANK

        ; ---- overscan phase ----
        lda phase
        beq OSSkip          ; phase=0 -> the work already happened in VBLANK
        inc xpos            ; same work, later in the frame
OSSkip:
        inc frames
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

        org $FFFC
        .word Start
        .word Start
