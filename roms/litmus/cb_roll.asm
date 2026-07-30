; cb_roll — NEAR-INVISIBILITY demo: a striped background plus ONE heavy line per
; frame that eats a 2nd scanline (262 -> 263 lines).
;
; ★RE-MEASURED 2026-07-30. This header used to say cb_roll and cb_clean render
; "pixel-identically". They do not, and the true number is a better argument than
; the false one: of the 192 visible scanlines, EXACTLY ONE differs — scanline 133,
; where the stolen line duplicates the stripe above it ($060606 where cb_clean has
; $380774). Everything else, including the frame's total visible content, is
; identical. The TV's auto-sync absorbs the one-line slip so the picture does not
; roll; what it leaves behind is a single doubled stripe in 192.
;
; That is the whole point: one row in 192 is not something you catch by eye, while
; the numbers differ unambiguously (263 vs 262 scanlines). VV-2
; (prove_line_budget) flags this heavy line as over-budget statically over ALL
; paths, which is exactly why a static prover is needed. Pinned by
; internal/emu.TestCbRollIsOneRowFromCbClean. Pair with cb_clean. Self-contained.

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

; --- Visible top: 96 striped lines (COLUBK = X = a moving stripe) ---
        ldx #96
Top:    sta WSYNC
        stx COLUBK
        dex
        bne Top

; --- ONE heavy line: ~100cy delay -> eats a 2nd scanline (the over-budget line) ---
        ldy #20
Burn:   dey
        bne Burn
        sta WSYNC

; --- Visible bottom: 95 striped lines ---
        ldx #95
Bot:    stx COLUBK
        sta WSYNC
        dex
        bne Bot

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS

        jmp Main

        org $FFFC
        .word Reset
        .word Reset
