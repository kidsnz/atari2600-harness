; smoke.asm — ハーネス配管検証用の最小決定的 ROM
; 目的: Gopher2600 を自前 Go から headless 駆動できることを数値で確認する。
;   - RAM $80 に sentinel 値 $42 を置く（read_ram の検証点）
;   - NTSC 262 ライン/フレームをきっちり生成する（VSYNC3/VBLANK37/可視192/Overscan30）
;   - COLUBK を既知値に（TIA read の検証点）
; include は使わず TIA/RIOT レジスタを自前 equ で定義（自己完結）。

        processor 6502

; --- TIA write registers ---
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
        sta $00,x       ; ゼロページ（RAM/TIA ミラー）をクリア
        dex
        bne ClearMem    ; x=$FF→$01 までクリア（$00 は据え置き、無害）

        lda #$42
        sta $80         ; sentinel: read_ram($80) == $42 を期待

MainLoop:
; --- VSYNC: 3 lines ---
        lda #2
        sta VBLANK      ; 消灯
        sta VSYNC       ; #2 で VSYNC on
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC       ; VSYNC off

; --- VBLANK: 37 lines ---
        ldx #37
VBlankLoop:
        sta WSYNC
        dex
        bne VBlankLoop

        lda #0
        sta VBLANK      ; 可視領域へ：消灯解除

; --- Visible: 192 lines ---
        lda #$1E        ; 既知の背景色（黄系）
        sta COLUBK
        ldx #192
VisibleLoop:
        sta WSYNC
        dex
        bne VisibleLoop

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK      ; 消灯
        ldx #30
OverscanLoop:
        sta WSYNC
        dex
        bne OverscanLoop

        jmp MainLoop

; --- vectors ---
        org $FFFC
        .word Reset     ; RESET
        .word Reset     ; IRQ/BRK
