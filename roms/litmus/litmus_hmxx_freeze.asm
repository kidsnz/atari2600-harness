; litmus_hmxx_freeze — HMOVE 後 24cy 以内の HMxx 書換の実測（U-M11）
; 文書（Stella PG）: 「HMOVE 後 24 CPU サイクルは HMxx に書くな（予測不能な移動）」。
; 実測計画: P0 は HMP0=$80（右8＝移動ティックが窓いっぱい続く）を毎フレーム HMOVE 適用。
;   frame 10-19: HMOVE の +13cy 後（禁止窓内）に HMP0=$00 へ書換（移動の中断を試みる）
;   frame 30-39: HMOVE の +33cy 後（窓外＝移動完了後）に同じ書換
; 各期間の 1 フレームあたり移動量を位置で測る（窓外=+8 のはず / 窓内=異常があれば差が出る）。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
RESP0   = $10
GRP0    = $1B
HMP0    = $20
HMOVE   = $2A
HMCLR   = $2B

fc      = $80

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        sta WSYNC
        ds 13, $EA          ; SLEEP 26
        sta RESP0
        lda #$FF
        sta GRP0

NextFrame:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK
        inc fc
        ; --- 毎フレーム: HMP0=$F0 をセットして HMOVE ---
        lda #$80            ; 右 8px（移動ティックが HBLANK 窓いっぱい続く）
        sta HMP0
        ; 期間 C（frame 50-59）は HMOVE 直後 +3cy で書換するため先に判定して別経路
        lda fc
        cmp #50
        bcc FzNorm
        cmp #60
        bcs FzNorm
        lda #$00
        sta WSYNC
        sta HMOVE           ; 完了 cy3
        sta HMP0            ; 完了 cy6（HMOVE 直後・移動進行中）
        jmp FzDone
FzNorm: sta WSYNC
        sta HMOVE           ; 3cy（完了 cy3）
        ; --- 期間別の書換 ---
        lda fc              ; 5cy（cy8）
        cmp #10             ; cy10
        bcc FzNone
        cmp #20
        bcs FzLate
        lda #$00            ; 窓内書換: HMOVE+~13cy で HMP0=0（移動中断を試みる）
        sta HMP0            ; 完了 ~cy15（24cy 窓内）
        jmp FzDone
FzLate: cmp #30
        bcc FzNone
        cmp #40
        bcs FzC
        ds 8, $EA           ; +16cy → ~cy28（窓外）
        lda #$00
        sta HMP0            ; 完了 ~cy33
        jmp FzDone
FzC:    cmp #50
        bcc FzNone
        cmp #60
        bcs FzNone
        jmp FzDone          ; 期間 C は下の即時書換パスで処理（このパスには来ない）
FzNone:
FzDone: ds 10, $EA          ; 残り移動の完了を待つ（行内）
        sta HMCLR
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        ldx #192
Vis:    sta WSYNC
        dex
        bne Vis
        lda #2
        sta VBLANK
        ldx #28
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

        org $FFFC
        .word Start
        .word Start
