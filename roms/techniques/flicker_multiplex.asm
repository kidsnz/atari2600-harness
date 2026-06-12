; flicker_multiplex — flicker 多重化のクリーンルーム・デモ（technique #10, docs/techniques/flicker-multiplexing.md）
; ハード上限「players は 2」を時間方向に破る: 4 オブジェクトをフレームパリティで 2 個ずつ交互に描く
; ＝各オブジェクトは 30Hz 表示（点滅）。同一ラインに何体重なっても破綻しない、Pac-Man のゴーストの正体。
; （縦ゾーン #1 と違い、オブジェクトの Y がどこにあっても・重なっても良いのが強み）
; kernel は #3 の縦判定 ×2（P0/P1, ~49cy/行）。色はオブジェクト固有＝点滅で4色が交互に見える。
; 完全形（毎フレーム Y ソート→2-of-N 動的割当→ゾーン内再配置）は doc 参照（documented）。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
COLUP1  = $07
COLUBK  = $09
RESP0   = $10
RESP1   = $11
GRP0    = $1B
GRP1    = $1C
HMP0    = $20
HMP1    = $21
HMOVE   = $2A

SPRITE_H = 8

; オブジェクト: 0=Y可変X40 / 1=Y可変X120 / 2=X可変Y60 / 3=X可変Y120
oy0     = $80       ; obj0 Y（10..150 を +1 ピンポン）
oy1     = $81       ; obj1 Y（150..10 を -1 ピンポン）
ox2     = $82       ; obj2 X（10..140 を +1 ピンポン）
ox3     = $83       ; obj3 X（140..10 を -1 ピンポン）
d0      = $84
d1      = $85
d2      = $86
d3      = $87
frameCt = $88
sy0     = $89       ; 今フレームのスロット0（P0）Y
sy1     = $8A       ; 今フレームのスロット1（P1）Y
px0     = $8B       ; 今フレームのスロット0 X
px1     = $8C       ; 今フレームのスロット1 X
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
        lda #10
        sta oy0
        sta ox2
        lda #150
        sta oy1
        lda #140
        sta ox3
        lda #1
        sta d1
        sta d3
        lda #$E6
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
        ; --- VB行1: obj0/obj1 の移動 ---
        lda d0
        bne U0u
        inc oy0
        lda oy0
        cmp #150
        bcc U0e
        lda #1
        sta d0
U0e:    jmp U1
U0u:    dec oy0
        lda oy0
        cmp #11
        bcs U1
        lda #0
        sta d0
U1:     lda d1
        bne U1u
        inc oy1
        lda oy1
        cmp #150
        bcc U1e
        lda #1
        sta d1
U1e:    jmp UEnd1
U1u:    dec oy1
        lda oy1
        cmp #11
        bcs UEnd1
        lda #0
        sta d1
UEnd1:  sta WSYNC           ; VB 1
        ; --- VB行2: obj2/obj3 の移動 ---
        lda d2
        bne U2u
        inc ox2
        lda ox2
        cmp #140
        bcc U2e
        lda #1
        sta d2
U2e:    jmp U3
U2u:    dec ox2
        lda ox2
        cmp #11
        bcs U3
        lda #0
        sta d2
U3:     lda d3
        bne U3u
        inc ox3
        lda ox3
        cmp #140
        bcc U3e
        lda #1
        sta d3
U3e:    jmp UEnd2
U3u:    dec ox3
        lda ox3
        cmp #11
        bcs UEnd2
        lda #0
        sta d3
UEnd2:  sta WSYNC           ; VB 2
        ; --- VB行3: パリティで 2 個選んでスロットへ（30Hz flicker の心臓部）---
        inc frameCt
        lda frameCt
        and #1
        bne SelOdd
        lda oy0             ; 偶: obj0→P0 / obj1→P1
        sta sy0
        lda oy1
        sta sy1
        lda #40
        sta px0
        lda #120
        sta px1
        lda #$1E            ; obj0=黄
        sta COLUP0
        lda #$56            ; obj1=桃
        sta COLUP1
        jmp SelEnd
SelOdd: lda #60             ; 奇: obj2→P0 / obj3→P1
        sta sy0
        lda #120
        sta sy1
        lda ox2
        sta px0
        lda ox3
        sta px1
        lda #$9A            ; obj2=青
        sta COLUP0
        lda #$CA            ; obj3=緑
        sta COLUP1
SelEnd: sta WSYNC           ; VB 3
        ; --- VB行4-5: スロットを配置 → 共有 HMOVE 1発 ---
        lda px0
        clc
        adc #XCAL
        sec
Dv0:    sbc #15
        bcs Dv0
        tay
        lda HMOVE_LUT,y
        sta HMP0
        sta RESP0
        sta WSYNC           ; VB 4
        lda px1
        clc
        adc #XCAL
        sec
Dv1:    sbc #15
        bcs Dv1
        tay
        lda HMOVE_LUT,y
        sta HMP1
        sta RESP1
        sta WSYNC           ; VB 5
        sta HMOVE
        ldx #31             ; VBLANK 残り（5+31+1=37）
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        sta WSYNC           ; VB 37 → 可視開始

        ; --- 可視 192 行: 縦判定 ×2（任意 Y・重なり OK）---
        ldy #0
Vis:    sta WSYNC
        tya                 ; P0 スロット
        sec
        sbc sy0
        cmp #SPRITE_H
        bcc VD0
        lda #0
        beq VS0
VD0:    tax
        lda Art,x
VS0:    sta GRP0            ; ~21
        tya                 ; P1 スロット
        sec
        sbc sy1
        cmp #SPRITE_H
        bcc VD1
        lda #0
        beq VS1
VD1:    tax
        lda Art,x
VS1:    sta GRP1            ; ~42
        iny
        cpy #192
        bne Vis             ; ~49

        lda #2
        sta VBLANK
        lda #0
        sta GRP0
        sta GRP1
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame       ; 3+37+192+30 = 262 を明示所有

XCAL = -8                   ; lda zp プロローグ（sprite_anim と同型）。実測で確認すること

Art:    ; 8×8 の丸（自作・全オブジェクト共通、色で区別）
        byte %00111100
        byte %01111110
        byte %11111111
        byte %11111111
        byte %11111111
        byte %11111111
        byte %01111110
        byte %00111100

HMOVE_TBL:
        byte $80,$70,$60,$50,$40,$30,$20,$10,$00,$F0,$E0,$D0,$C0,$B0,$A0,$90
HMOVE_TBL_END:
HMOVE_LUT = HMOVE_TBL_END - 256

        org $FFFC
        .word Start
        .word Start
