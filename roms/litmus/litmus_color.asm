; litmus_color.asm — per-scanline 色 litmus（gap 1）
; 目的: 1 scanline ごとに COLUBK を変える「縦の色グラデーション」を作り、
;       read_row が per-scanline 色を数値で拾えること＝Monet 的な縦色帯の土台を裏取りする。
;
; 手法: 可視 192 ラインのループで毎ライン `stx COLUBK`（x = 下りカウンタ 191..0）。
;       各ラインが異なる単色背景になる。playfield は未使用＝各 read_row は全幅 160 の単一 run。
; 期待: read_row を複数 scanline で呼ぶと、背景 hex が**ライン毎に違い**かつ**行内は均一**。
;       具体的には COLUBK = (192 − 可視scanline) 付近の値に対応した色。

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

; --- VBLANK: 37 lines ---
        ldx #37
VBlankLoop:
        sta WSYNC
        dex
        bne VBlankLoop
        lda #0
        sta VBLANK      ; 可視へ

; --- Visible: 192 lines。毎ライン COLUBK = x（縦グラデーション）---
        ldx #192
VisibleLoop:
        sta WSYNC
        stx COLUBK      ; 背景色 = 現在の下りカウンタ値（ライン毎に変化）
        dex
        bne VisibleLoop

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK
        lda #0
        sta COLUBK      ; overscan は黒へ戻す
        ldx #30
OverscanLoop:
        sta WSYNC
        dex
        bne OverscanLoop

        jmp MainLoop

        org $FFFC
        .word Reset
        .word Reset
