; sfx_demo — 効果音ツールキット 5 種レシピ（technique: sound-effects, U-M2）
; pkg/audio の SFX ヘルパ（PitchSweep/NoiseBurst/Blip/Arpeggio）で生成した
; フレーム表（2バイト/フレーム: AUDC<<4|AUDV, AUDF）を、fire 押下で順に再生する。
;   1. laser   — square 下降スイープ（AUDF 4→22, 12f）
;   2. boom    — ノイズ減衰バースト（AUDC=8, 24f）
;   3. pickup  — 上昇アルペジオ（G5→C6→E6, 12f）
;   4. bounce  — 短ブリップ（AUDC=6, 4f）
;   5. engine  — 低ランブル減衰（AUDC=3, 30f）
; プレイヤーは ch0。再生終了で消音。INPT4 D7 のエッジ検出で次の効果へ。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
AUDC0   = $15
AUDF0   = $17
AUDV0   = $19
INPT4   = $3C

sfxOn   = $80       ; 1=再生中
sfxIdx  = $81       ; 表内オフセット（2/フレーム）
sfxLen  = $82       ; 表バイト長
effNo   = $83       ; 次に鳴らす効果 0-4
prevFi  = $84       ; 前フレームの fire（エッジ検出）
ptr     = $86       ; 表ポインタ（2バイト）
tmp     = $88

        org $F000
Start:  sei
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
        lda #2
        sta VBLANK
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

        ; --- overscan: 入力（エッジ）→ トリガ → SFX tick ---
        lda INPT4
        bmi NoPress         ; D7=1 は非押下
        lda prevFi
        bne Pressed         ; 押しっぱなしは無視
        jsr Trigger
Pressed:
        lda #1
        sta prevFi
        bne Tick
NoPress:
        lda #0
        sta prevFi
Tick:   lda sfxOn
        beq Silent
        ldy sfxIdx
        lda (ptr),y         ; AUDC<<4 | AUDV
        sta tmp
        and #$0F
        sta AUDV0
        lda tmp
        lsr
        lsr
        lsr
        lsr
        sta AUDC0
        iny
        lda (ptr),y
        sta AUDF0
        iny
        sty sfxIdx
        cpy sfxLen
        bcc TickEnd
        lda #0              ; 再生終了
        sta sfxOn
Silent: lda #0
        sta AUDV0
TickEnd:
        ldx #29
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

; Trigger: effNo の効果をセットして effNo を巡回
Trigger:
        ldx effNo
        lda EffLo,x
        sta ptr
        lda EffHi,x
        sta ptr+1
        lda EffLen,x
        sta sfxLen
        lda #0
        sta sfxIdx
        lda #1
        sta sfxOn
        inx
        cpx #5
        bcc TrigEnd
        ldx #0
TrigEnd:
        stx effNo
        rts

EffLo:  byte <SfxLaser,<SfxBoom,<SfxPickup,<SfxBounce,<SfxEngine
EffHi:  byte >SfxLaser,>SfxBoom,>SfxPickup,>SfxBounce,>SfxEngine
EffLen: byte 24,48,24,8,60

SfxLaser: ; 12 frames (2 bytes each: AUDC<<4|AUDV, AUDF)
        byte $4C,$04
        byte $4C,$05
        byte $4C,$07
        byte $4C,$08
        byte $4C,$0A
        byte $4C,$0C
        byte $4C,$0D
        byte $4C,$0F
        byte $4C,$11
        byte $4C,$12
        byte $4C,$14
        byte $4C,$16
SfxBoom: ; 24 frames (2 bytes each: AUDC<<4|AUDV, AUDF)
        byte $8F,$06
        byte $8E,$06
        byte $8D,$06
        byte $8D,$06
        byte $8C,$06
        byte $8B,$06
        byte $8B,$06
        byte $8A,$06
        byte $8A,$06
        byte $89,$06
        byte $88,$06
        byte $88,$06
        byte $87,$06
        byte $86,$06
        byte $86,$06
        byte $85,$06
        byte $85,$06
        byte $84,$06
        byte $83,$06
        byte $83,$06
        byte $82,$06
        byte $81,$06
        byte $81,$06
        byte $80,$06
SfxPickup: ; 12 frames (2 bytes each: AUDC<<4|AUDV, AUDF)
        byte $4A,$13
        byte $4A,$13
        byte $4A,$13
        byte $4A,$13
        byte $4A,$0E
        byte $4A,$0E
        byte $4A,$0E
        byte $4A,$0E
        byte $4A,$0B
        byte $4A,$0B
        byte $4A,$0B
        byte $4A,$0B
SfxBounce: ; 4 frames (2 bytes each: AUDC<<4|AUDV, AUDF)
        byte $6A,$0A
        byte $6A,$0A
        byte $6A,$0A
        byte $6A,$0A
SfxEngine: ; 30 frames (2 bytes each: AUDC<<4|AUDV, AUDF)
        byte $38,$1C
        byte $37,$1C
        byte $37,$1C
        byte $37,$1C
        byte $36,$1C
        byte $36,$1C
        byte $36,$1C
        byte $36,$1C
        byte $35,$1C
        byte $35,$1C
        byte $35,$1C
        byte $35,$1C
        byte $34,$1C
        byte $34,$1C
        byte $34,$1C
        byte $34,$1C
        byte $33,$1C
        byte $33,$1C
        byte $33,$1C
        byte $32,$1C
        byte $32,$1C
        byte $32,$1C
        byte $32,$1C
        byte $31,$1C
        byte $31,$1C
        byte $31,$1C
        byte $31,$1C
        byte $30,$1C
        byte $30,$1C
        byte $30,$1C

        org $FFFC
        .word Start
        .word Start
