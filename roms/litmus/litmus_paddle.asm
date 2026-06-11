; litmus_paddle — パドル読みの実機裏取り（V2-4b: INPT0 充電タイミング + VBLANK D7 ダンプ）
; 標準手順: VBLANK 中 D7=1 でコンデンサ放電 → 可視開始で D7=0（充電開始）→ 毎ライン INPT0 D7 を
; ポーリングし、立った時点のライン番号を $90 に記録（パドル位置に比例して増える）。
; $90 = 充電完了ライン数（毎フレーム更新）。$91 = 未完了なら $FF（=パドル最大でも 192 行で届かない時の検出）。
; 実機裏取り済（Gopher2600, v0.54.0, scenario 入力タイムライン value= 経由）: 転送カーブ（単調・RC充電型）
;  value 0.0→0行 / 0.1→5行 / 0.25→69行 / 0.5→176行 / 0.6以上→$FF（192行内に届かず）。
;  回帰固定=scenarios/paddle.json。set_input action=paddle / scenario inputs value= は本件で追加。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
INPT0   = $38

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
        sta $2C

NextFrame:
        lda #2
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #$82            ; blank + D7=1（パドルコンデンサをダンプ＝放電）
        sta VBLANK
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK          ; 可視開始＝D7=0 → 充電開始

        ; --- 可視 192 行: INPT0 D7 が立つまでのライン数を数える ---
        ldy #0              ; ライン番号
        lda #$FF
        sta $91             ; 未完了マーカー
Scan:   sta WSYNC
        bit INPT0           ; N ← INPT0 D7
        bmi Done            ; 立った
        iny
        cpy #192
        bne Scan
        ; 192 行で立たず → $90 に $FF
        lda #$FF
        sta $90
        jmp Over
Done:   sty $90             ; 充電完了ライン数
        lda #0
        sta $91
        ; 残り可視を消化
Rest:   sta WSYNC
        iny
        cpy #192
        bne Rest

Over:   lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

        org $FFFC
        .word Start
        .word Start
