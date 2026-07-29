; litmus_stack_trick.asm — writing a TIA register with PHA.
;
; The stack pointer is aimed at a TIA register and a PUSH becomes the store.
; Page 1 mirrors the same addresses the console decodes, so with SP = $09 a PHA
; writes $0109, which is COLUBK. Commercial kernels use this to get a register
; written in fewer cycles than a `sta` would take, and Combat's missile enable is
; the famous one (SP at $1D, `PHP` landing the Z flag in ENAM0).
;
; It exists here because an analysis that does not track SP cannot see this write
; at all: the instruction is one implied-addressing byte with no operand, so a
; naive read of the opcode says it touches no memory. The write is real, it goes
; to a TIA register, and it changes the picture.
;
; Observable ground truth: the background turns green ($C4). No JSR anywhere, so
; nothing else needs the stack while it is pointed away.
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
clr:    sta $00,x
        dex
        bne clr

Frame:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

        lda #2
        sta VBLANK

        ; THE POINT: aim the stack at COLUBK and push the colour into it.
        ldx #COLUBK             ; $09
        txs                     ; SP = $09
        lda #$C4                ; green
        pha                     ; writes $0109 -> COLUBK, SP becomes $08
        ldx #$FF
        txs                     ; put the stack back before anything needs it

        ldx #36
vb:     sta WSYNC
        dex
        bne vb

        lda #0
        sta VBLANK
        ldx #192
vis:    sta WSYNC
        dex
        bne vis

        lda #2
        sta VBLANK
        ldx #30
os:     sta WSYNC
        dex
        bne os

        jmp Frame

        org $FFFA
        .word Reset
        .word Reset
        .word Reset
