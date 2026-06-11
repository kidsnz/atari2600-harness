; litmus_input — 入力ポートの実機裏取り（V2-4: SWCHA ジョイスティック＋INPT4 fire＋VBLANK D6 ラッチ）
; 毎フレーム overscan で SWCHA→$80, INPT4→$81 に記録（read_ram で数値確認）。
; VBLANK は常に D6=1（ラッチ有効: blank=$42 / visible=$40）→ fire を押して離しても INPT4 D7 は 0 のまま＝ラッチ立証。
; 期待値（Stella PG §12 / vcs_reference）: 無入力 SWCHA=$FF。P0 left 押下=D6→0 ($BF)。INPT4 D7: 1=離/0=押。
; 実機裏取り済（Gopher2600, v0.42.0, set_input 経由）:
;  無入力: $80(SWCHA)=$FF, $81(INPT4)=$BC（D7=1 離・下位はバスノイズ→N フラグでのみ判定すべき根拠）。
;  P0 left 押下: SWCHA=$BF（D6→0）。fire 押下: INPT4=$3C（D7→0）。
;  ★ラッチ: fire を離して3フレーム後も INPT4=$3C のまま（VBLANK D6 ラッチ動作を立証）。
;  方向は非ラッチ: left 解放で SWCHA は即 $FF（対照）。回帰固定=scenarios/input.json（入力タイムライン）。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
INPT4   = $3C
SWCHA   = $0280

        org $F000
Start:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

NextFrame:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #$42            ; blank ON + INPT4/5 ラッチ有効 (D6)
        sta VBLANK
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #$40            ; blank OFF・ラッチは維持
        sta VBLANK

        ldy #192
Vis:    sta WSYNC
        dey
        bne Vis

        lda #$42
        sta VBLANK
        ; --- 入力サンプリング（overscan 先頭）---
        lda SWCHA
        sta $80
        lda INPT4
        sta $81
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

        org $FFFC
        .word Start
        .word Start
