; litmus_pos.asm — 横位置 litmus test 用 ROM（欠落 B の検証）
; player0 を「WSYNC 同期点から N CPU サイクル後に RESP0 ストローブ」で位置決めする。
; 遅延ユニットは RAM $80 で外部から poke 可能 → 再アセンブル無しで N をスイープできる。
;
; 粗調整ループ: 1 反復 = SBC(2) + BCS(3) = 5 CPU サイクル = 15 カラークロック = 15px。
; よって $80 を 1 増やすと ResetPixel が 15px 動く想定。これをハーネスの read_tia で実測検証。
; HMOVE は使わない（HMCLR で動きレジスタを 0 に）。微調整(±1px)の HMOVE 検証は次段。

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUBK  = $09
RESP0   = $10
HMCLR   = $2B
GRP0    = $1B

DELAY   = $80           ; 粗調整ループ回数（poke で書き換える）

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
        sta DELAY       ; 初期遅延ユニット（後で poke で上書き）
        lda #$0E
        sta COLUP0      ; player0 白
        lda #$FF
        sta GRP0        ; player0 全点灯（8px 幅）
        lda #0
        sta NUSIZ0      ; 標準サイズ・1コピー

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

; --- VBLANK: 37 lines。最後のラインで player0 を位置決め ---
        ldx #37
VBlankLoop:
        sta WSYNC
        dex
        bne VBlankLoop

        ; ---- 位置決めカーネル ----
        ; WSYNC で次ラインの先頭（HBLANK 開始）にビームを合わせる
        sta WSYNC               ; 同期点
        sta HMCLR               ; 動きレジスタ 0（3）
        lda DELAY               ; 遅延ユニット取得（3, zp）
        sec                     ; キャリーセット（2）
DelayLoop:
        sbc #1                  ; 2
        bcs DelayLoop           ; 3（taken,同ページ）→ 1反復5サイクル
        sta RESP0               ; player0 位置確定ストローブ（3）
        lda #0
        sta VBLANK              ; 可視へ

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
