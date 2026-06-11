; litmus_mirror — アドレスミラーの実機裏取り（V2-12, woodgrain Memory_Map）
; RAM: $0180 は ゼロページ RAM $80 のミラー（＝スタックが効く理由）。TIA: $0049 は COLUBK($09) のミラー。
; RAM マップ:
;  $90 = $0180 に $5A 書き → $0080 を読む（=$5A 期待: 鏡像成立）
;  $91 = $0080 に $A5 書き → $0180 を読む（=$A5 期待: 逆向きも成立）
; TIA: 可視域の背景を【ミラー $0049 経由】で $84(青) に設定 → read_row 背景が青なら TIA ミラー成立。
; 実機裏取り済（Gopher2600, v0.49.0）: $90=$5A（$0180書き→$0080読み）/ $91=$A5（逆向き）＝RAM $0180 は $0080 のミラー
; （スタックが効く理由）。背景は TIA ミラー $0049 経由で $84 青に＝read_row(100)=$84。回帰固定=scenarios/mirror.json。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK_M = $49          ; COLUBK ($09) の TIA ミラー
RAM_M    = $0180        ; $80 の RAM ミラー（スタックページ）

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
        sta $2C             ; CXCLR

        ; --- RAM ミラー ---
        lda #$5A
        sta RAM_M           ; $0180 へ書く
        lda $80             ; $0080 を読む
        sta $90             ; 期待 $5A
        lda #$A5
        sta $80             ; $0080 へ書く
        lda RAM_M           ; $0180 を読む
        sta $91             ; 期待 $A5
        ; （$90/$91 が $A5 で上書きされないよう別番地）

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
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        ; --- 可視: 背景を TIA ミラー $0049 経由で $84(青) に ---
        lda #$84
        sta COLUBK_M        ; ミラー書き込み
        ldy #192
Vis:    sta WSYNC
        dey
        bne Vis
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

        org $FFFC
        .word Start
        .word Start
