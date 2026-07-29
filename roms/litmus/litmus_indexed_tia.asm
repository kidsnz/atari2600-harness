; litmus_indexed_tia.asm — an INDEXED store to a TIA register.
;
; Exists to pin down which register a write actually reaches. `sta COLUP0,x`
; with x=3 writes COLUBK, not COLUP0 — the register is the EFFECTIVE address, and
; any tool that reports the base operand instead is naming the wrong register.
; That is not a corner case: indexed TIA stores are how multiplexed kernels set
; several objects from one loop, which is exactly where a write→beam timeline is
; worth having.
;
; Observable ground truth: the background turns green ($C4). If a tool claims the
; write went to COLUP0, the screen disagrees with it.
;
; Self-contained (no vcs.h); NTSC 3/37/192/30 = 262.

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
COLUP1  = $07
COLUPF  = $08
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
        ; --- VSYNC 3 ---
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

        ; --- VBLANK 37 ---
        lda #2
        sta VBLANK

        ; THE POINT OF THIS ROM: base is COLUP0 ($06), the index is 3, so the
        ; hardware writes COLUBK ($09). Anything reporting COLUP0 is wrong.
        ldx #3
        lda #$C4                ; green
        sta COLUP0,x            ; -> COLUBK

        ldx #36
vb:     sta WSYNC
        dex
        bne vb

        ; --- visible 192 ---
        lda #0
        sta VBLANK
        ldx #192
vis:    sta WSYNC
        dex
        bne vis

        ; --- overscan 30 ---
        lda #2
        sta VBLANK
        ldx #30
os:     sta WSYNC
        dex
        bne os

        jmp Frame

        org $FFFA
        .word Reset             ; NMI
        .word Reset             ; RESET
        .word Reset             ; IRQ
