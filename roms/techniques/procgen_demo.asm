; procgen_demo — 乱数/手続き生成の定石（technique: procedural, U-M7）
; litmus_lfsr（v0.46.0: Galois 8bit `lsr/eor #$8E`・周期 255・非ゼロ検証済み）の authoring 側:
;   ・既知シード（$5A）から 30 フレーム毎に 1 ステップ → 敵出現 X に写像（X = (v&$7F)+16）
;   ・スポーンマーカー（P0）を PosObj で配置、スポーン数をカウント
; 検証: 参照列（Go/手計算: $2D,$98,$4C,$26,$13,$87,$CD,$E8）と RAM・実 X が一致＝
; 「シードが同じなら世界が再現される」を数値で固定。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
RESP0   = $10
GRP0    = $1B
HMP0    = $20
HMOVE   = $2A
HMCLR   = $2B
TIM64T  = $0296
INTIM   = $0284

lfsr    = $80       ; 乱数状態（非ゼロ維持・周期 255）
fc      = $81       ; フレームカウンタ（30 で 1 スポーン）
spawns  = $82       ; スポーン数
markX   = $83       ; マーカー X

SEED    = $5A

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #SEED
        sta lfsr
        lda #80
        sta markX
        lda #$36
        sta COLUP0

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
        lda #48
        sta TIM64T
        ; --- 30 フレーム毎に LFSR ステップ → スポーン ---
        inc fc
        lda fc
        cmp #30
        bcc NoSpawn
        lda #0
        sta fc
        lda lfsr            ; Galois 右シフト（litmus_lfsr の形そのまま）
        lsr
        bcc NoTap
        eor #$8E
NoTap:  sta lfsr
        and #$7F            ; X = (v & $7F) + 16 → 16..143
        clc
        adc #16
        sta markX
        inc spawns
NoSpawn:
VBwait: lda INTIM
        bne VBwait
        lda markX
        ldx #0
        jsr PosObj
        sta WSYNC
        lda #0
        sta VBLANK

        ; ===== 可視 186 行（マーカーは行 90-105） =====
        ldx #0
KLine:  sta WSYNC
        txa
        sec
        sbc #90
        cmp #16
        bcc KOn
        lda #0
        beq KStore
KOn:    lda #$3C
KStore: sta GRP0
        inx
        cpx #186
        bne KLine

        lda #0
        sta GRP0
        lda #2
        sta VBLANK
        ldx #29
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

; ===== PosObj（共通形・実測校正） =====
        align 16
PosObj: clc
        adc #3              ; 校正: 実X = A−3（read_tia 裏取り）
        sec
        sta WSYNC
PDiv:   sbc #15
        bcs PDiv
        eor #7
        asl
        asl
        asl
        asl
        sta HMP0,x
        sta RESP0,x
        sta WSYNC
        sta HMOVE
        ds 12, $EA
        sta HMCLR
        rts

        org $FFFC
        .word Start
        .word Start
