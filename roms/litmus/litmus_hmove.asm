; litmus_hmove.asm — 横位置 litmus 後半（HMOVE ±1px 微調整の検証）
; player0 を固定の粗位置（DELAY=$80）に RESP0 で置き、HMP0=$81(pokeable) を設定して
; HMOVE をストローブ。read_tia の HmovedPixel が ResetPixel からどれだけ動くかを実測し、
; CLAUDE.md の HMOVE ニブル符号規約（$70=左7…$00=0…$F0=右1…$80=右8、正=左/負=右）を裏取りする。

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUBK  = $09
RESP0   = $10
GRP0    = $1B
HMP0    = $20
HMOVE   = $2A
HMCLR   = $2B

DELAY   = $80           ; 粗調整ループ回数（poke 可。既定 6 → ResetPixel≈72）
HMVAL   = $81           ; HMP0 に書く値（poke 可。上位ニブルのみ有効）

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

        lda #6
        sta DELAY
        lda #0
        sta HMVAL       ; 既定は動き 0
        lda #$0E
        sta COLUP0
        lda #$FF
        sta GRP0
        lda #0
        sta NUSIZ0

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

; --- VBLANK: 35 lines（残り 2 ラインを位置決め＋HMOVE に使う）---
        ldx #35
VBlankLoop:
        sta WSYNC
        dex
        bne VBlankLoop

        ; ---- 粗位置決めライン ----
        sta WSYNC               ; 同期点
        sta HMCLR               ; 動きレジスタ 0
        lda DELAY
        sec
DelayLoop:
        sbc #1
        bcs DelayLoop
        sta RESP0               ; ResetPixel 確定
        lda HMVAL
        sta HMP0                ; 動き量セット（上位ニブル）

        ; ---- HMOVE 適用ライン ----
        sta WSYNC               ; WSYNC 直後に HMOVE（鉄則）
        sta HMOVE               ; HmovedPixel が ResetPixel±移動量へ
        lda #0
        sta VBLANK

; --- Visible: 192 lines ---
        ldx #192
VisibleLoop:
        sta WSYNC
        dex
        bne VisibleLoop

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK
        ldx #30
OverscanLoop:
        sta WSYNC
        dex
        bne OverscanLoop

        jmp MainLoop

        org $FFFC
        .word Reset
        .word Reset
