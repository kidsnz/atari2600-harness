; Monet Frogger (playable) — internal/playfield.GenerateFroggerASM
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
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
CXCLR   = $2C
CXPPMM  = $37
SWCHA   = $0280
FrogY   = $81
PrevU   = $82
PrevD   = $83
Lives   = $84
Score   = $85
OnPad   = $86
PrevY   = $87

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
        lda #$C8
        sta COLUP0
        lda #$1C
        sta COLUP1
        lda #$03
        sta NUSIZ0
        lda #144
        sta FrogY
        sta PrevY
        lda #3
        sta Lives
        lda #0
        sta Score
        lda #$10
        sta PrevU
        lda #$20
        sta PrevD
        sta WSYNC
        ldx #14
PdD:    dex
        bne PdD
        sta RESP0
        sta WSYNC
        ldx #26
FrD:    dex
        bne FrD
        sta RESP1
        lda #$F0
        sta HMP0

NextFrame:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

        ; ===== game logic（前フレームの衝突を使う）=====
        lda CXPPMM
        and #$80
        sta OnPad

        lda SWCHA       ; up hop
        and #$10
        cmp PrevU
        beq UD
        sta PrevU
        cmp #$00
        bne UD
        lda FrogY
        cmp #24
        bcc UD
        sbc #16
        sta FrogY
UD:
        lda SWCHA       ; down hop
        and #$20
        cmp PrevD
        beq DD
        sta PrevD
        cmp #$00
        bne DD
        lda FrogY
        cmp #160
        bcs DD
        clc
        adc #16
        sta FrogY
DD:
        lda FrogY       ; drown? (川レーン FrogY=80 で葉に乗ってない)
        cmp #80
        bne ChkWin
        lda PrevY       ; 川に入った直後の1フレームは猶予（着地位置の衝突がまだ未登録）
        cmp #80
        bne ChkWin
        lda OnPad
        bne ChkWin
        dec Lives
        bne DrownReset  ; まだ残機あり → frog だけ start へ
        lda #3          ; game over → 全リスタート
        sta Lives
        lda #0
        sta Score
DrownReset:
        lda #144
        sta FrogY
        jmp HMlogic
ChkWin:
        lda FrogY       ; win? (FrogY<=16)
        cmp #17
        bcs HMlogic
        inc Score
        lda #144
        sta FrogY
HMlogic:
        ldx #$00        ; frog HMP1 = 入力 or 乗ってたら ride
        lda SWCHA
        and #$80
        bne HL1
        ldx #$E0
HL1:    lda SWCHA
        and #$40
        bne HL2
        ldx #$20
HL2:    cpx #$00
        bne HL3
        lda OnPad
        beq HL3
        ldx #$F0
HL3:    stx HMP1
        lda FrogY       ; 次フレームの猶予判定用に今フレームの FrogY を記録
        sta PrevY

        sta WSYNC
        sta HMOVE
        sta CXCLR
        ldx #35
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; ===== visible: water + frog(可変Y) + pads =====
        ldy #0
VL:     sta WSYNC
        lda BGTab,y
        sta COLUBK
        tya
        sec
        sbc FrogY
        cmp #8
        bcs NF
        tax
        lda FrogGfx,x
        jmp SF
NF:     lda #0
SF:     sta GRP1
        lda GRP0Tab,y
        sta GRP0
        iny
        cpy #192
        bne VL
        lda #0
        sta GRP0
        sta GRP1

        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

FrogGfx
        .byte $24
        .byte $7E
        .byte $FF
        .byte $FF
        .byte $BD
        .byte $7E
        .byte $24
        .byte $42

GRP0Tab
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $3C
        .byte $7E
        .byte $FF
        .byte $FF
        .byte $FF
        .byte $FF
        .byte $7E
        .byte $3C
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00

BGTab
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $1E
        .byte $54
        .byte $54
        .byte $54
        .byte $54
        .byte $56
        .byte $56
        .byte $56
        .byte $56
        .byte $56
        .byte $56
        .byte $56
        .byte $56
        .byte $5A
        .byte $5A
        .byte $5A
        .byte $5A
        .byte $6A
        .byte $6A
        .byte $6A
        .byte $6A
        .byte $6A
        .byte $6A
        .byte $6A
        .byte $6A
        .byte $B8
        .byte $B8
        .byte $B8
        .byte $B8
        .byte $BA
        .byte $BA
        .byte $BA
        .byte $BA
        .byte $C6
        .byte $C6
        .byte $C6
        .byte $C6
        .byte $B4
        .byte $B4
        .byte $B4
        .byte $B4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C6
        .byte $C6
        .byte $C6
        .byte $C6
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $7C
        .byte $7C
        .byte $7C
        .byte $7C
        .byte $88
        .byte $88
        .byte $88
        .byte $88
        .byte $86
        .byte $86
        .byte $86
        .byte $86
        .byte $84
        .byte $84
        .byte $84
        .byte $84
        .byte $8A
        .byte $8A
        .byte $8A
        .byte $8A
        .byte $96
        .byte $96
        .byte $96
        .byte $96
        .byte $9C
        .byte $9C
        .byte $9C
        .byte $9C
        .byte $5C
        .byte $5C
        .byte $5C
        .byte $5C
        .byte $56
        .byte $56
        .byte $56
        .byte $56
        .byte $98
        .byte $98
        .byte $98
        .byte $98
        .byte $5A
        .byte $5A
        .byte $5A
        .byte $5A
        .byte $5A
        .byte $5A
        .byte $5A
        .byte $5A
        .byte $5C
        .byte $5C
        .byte $5C
        .byte $5C
        .byte $64
        .byte $64
        .byte $64
        .byte $64
        .byte $B6
        .byte $B6
        .byte $B6
        .byte $B6
        .byte $64
        .byte $64
        .byte $64
        .byte $64
        .byte $BC
        .byte $BC
        .byte $BC
        .byte $BC
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4
        .byte $C4

        org $FFFC
        .word Start
        .word Start
