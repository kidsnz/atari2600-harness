; cb_jsr.asm — VV-14 2A self-test ROM (interprocedural JSR/RTS).
; One visible line calls a small WSYNC-free subroutine (Work) and returns. The
; static prover must FOLLOW the call, charge the callee's cycles + JSR/RTS, and
; bound the region (before 2A this was reported "JSR in region — unbounded").
; The region's worst case is well under one scanline, so it certifies at budget
; 76; a tight budget must flip it to a violation (proving the callee cost is
; actually counted, not ignored). Self-contained (no include).

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
ClearMem:
        sta $00,x
        dex
        bne ClearMem

Main:
; --- VSYNC: 3 lines ---
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: 37 lines ---
        ldx #37
VBlank:
        sta WSYNC
        dex
        bne VBlank
        lda #0
        sta VBLANK

; --- Visible top: 95 normal lines ---
        ldx #95
Top:
        sta WSYNC
        dex
        bne Top

; --- the line under test: call a subroutine, then continue ---
        sta WSYNC           ; opens the region
        jsr Work            ; 6 — prover must follow into Work and back
        nop

; --- Visible bottom: 95 normal lines ---
        ldx #95
Bottom:
        sta WSYNC
        dex
        bne Bottom

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK
        ldx #30
Overscan:
        sta WSYNC
        dex
        bne Overscan

        jmp Main

; --- WSYNC-free subroutine: ~15 cycles of straight-line work ---
Work:
        lda #5              ; 2
        clc                 ; 2
        adc #3              ; 2
        sta COLUBK          ; 3
        rts                 ; 6

; --- vectors ---
        org $FFFC
        .word Reset
        .word Reset
