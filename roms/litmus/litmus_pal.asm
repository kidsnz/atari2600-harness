; litmus_pal.asm — PAL フレーム構成の検証（hardening-roadmap F-3）
; 目的: harness が PAL（312 ライン）を正しく駆動・計測できることを数値で確認する。
;   - PAL 312 = VSYNC 3 / VBLANK 45 / 可視 228 / Overscan 36
;   - RAM $80 に sentinel $5A、COLUBK を既知値に
; 検証: scenario(tv_spec=PAL) で ntsc_frame_lines（=StepFrame の実ライン数）== 312。
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

        lda #$5A
        sta $80         ; sentinel: read_ram($80) == $5A

MainLoop:
; --- VSYNC: 3 lines ---
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: 45 lines (PAL) ---
        ldx #45
VBlankLoop:
        sta WSYNC
        dex
        bne VBlankLoop
        lda #0
        sta VBLANK

; --- Visible: 228 lines (PAL) ---
        lda #$54        ; 既知の背景色
        sta COLUBK
        ldx #228
VisibleLoop:
        sta WSYNC
        dex
        bne VisibleLoop

; --- Overscan: 36 lines (PAL) ---
        lda #2
        sta VBLANK
        ldx #36
OverscanLoop:
        sta WSYNC
        dex
        bne OverscanLoop

        jmp MainLoop

        org $FFFC
        .word Reset
        .word Reset
