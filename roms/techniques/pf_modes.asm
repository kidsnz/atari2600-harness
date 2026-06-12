; pf_modes — playfield モード（score-mode＋優先度）のクリーンルーム・デモ（technique #8 残り）
; CTRLPF の D1=score / D2=priority を実画面で検証する。
;  上 1/3（行 0-63）  : score mode（D1=1）。同じ PF パターンが左半分=COLUP0 色、右半分=COLUP1 色
;                       で塗られる＝2人のスコアを1つの PF で塗り分ける古典。
;  中 1/3（行 64-127）: 通常優先度。黄色い PF の壁（PF2 bit4）と重なる赤い P0 柱→P0 が壁の手前。
;  下 1/3（行 128-191）: priority（D2=1）。同じ重なりで PF が手前＝P0 は壁の「後ろ」を通る。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
COLUP1  = $07
COLUPF  = $08
COLUBK  = $09
CTRLPF  = $0A
PF0     = $0D
PF1     = $0E
PF2     = $0F
RESP0   = $10
GRP0    = $1B
HMP0    = $20
HMOVE   = $2A

XP0     = 62        ; P0 X（壁 clock 64-67 に重なる）
sent    = $9E

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
        lda #$44
        sta COLUP0          ; 赤
        lda #$86
        sta COLUP1          ; 青
        lda #$D5
        sta sent

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
        sta WSYNC           ; VB 1
        ; --- VB行2: P0 を XP0 へ ---
        lda #XP0
        clc
        adc #XCAL
        sec
Dv:     sbc #15
        bcs Dv
        tay
        lda HMOVE_LUT,y
        sta HMP0
        sta RESP0
        sta WSYNC           ; VB 2
        sta HMOVE
        ldx #34             ; VBLANK 残り（1+1+34+1=37）
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        sta WSYNC           ; VB 37 → 可視開始

        ; --- 行 0-63: score mode ---
        lda #2
        sta CTRLPF          ; D1=1: PF 左半分=COLUP0 / 右半分=COLUP1
        lda #$66
        sta PF1             ; ブロック列（左右同パターン・色だけ変わるのが見どころ）
        ldy #64
R1:     sta WSYNC
        dey
        bne R1
        ; --- 行 64-79: 壁・通常優先度（スプライトはまだ無し）---
        lda #0
        sta CTRLPF          ; D1=0, D2=0: 通常（players が PF の手前）
        sta PF1
        lda #$10
        sta PF2             ; 壁: 左 clock 64-67（repeat で右 144-147 にも）
        lda #$1C
        sta COLUPF          ; 黄
        ldy #16
R2a:    sta WSYNC
        dey
        bne R2a
        ; --- 行 80-127: P0 柱（通常優先度＝P0 が壁の手前）---
        lda #$FF
        sta GRP0
        ldy #48
R2b:    sta WSYNC
        dey
        bne R2b
        ; --- 行 128-175: priority ON（PF が手前＝P0 は壁の後ろ）---
        lda #4
        sta CTRLPF          ; D2=1
        ldy #48
R2c:    sta WSYNC
        dey
        bne R2c
        ; --- 行 176-191: 柱おわり ---
        lda #0
        sta GRP0
        ldy #16
R2d:    sta WSYNC
        dey
        bne R2d

        lda #2
        sta VBLANK
        lda #0
        sta PF2
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame       ; 3+37+(64+16+48+48+16)+30 = 262 を明示所有

XCAL = -5                   ; lda #imm プロローグ。実測で確認すること

HMOVE_TBL:
        byte $80,$70,$60,$50,$40,$30,$20,$10,$00,$F0,$E0,$D0,$C0,$B0,$A0,$90
HMOVE_TBL_END:
HMOVE_LUT = HMOVE_TBL_END - 256

        org $FFFC
        .word Start
        .word Start
