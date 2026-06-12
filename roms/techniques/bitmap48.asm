; bitmap48 — 48px ビットマップ帯＋窓スクロール（technique: bitmap48, AA2-B2, v1.59.0）
; 系譜: RevEng の Bitmap Minikernel（topic/168603）— スコアカーネルの 6 ポインタを
; ビットマップの列スライスへ向け替え、**窓オフセットで大きな絵の一部を表示**する。
; ロゴ・メッセージ帯・縦スクロールに使う汎用形（自前実装）。
;   ・ビットマップは列優先 6 表（各 48B・下の行から）
;   ・ポインタ = ColK + offset、可視窓 24 行、offset を毎フレーム ±1 でバウンド＝スクロール
;   ・カーネルは score6/text12 と同一の VDEL 6-store 振付（1 行/走査線）
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
NUSIZ1  = $05
COLUP0  = $06
COLUP1  = $07
RESP0   = $10
RESP1   = $11
GRP0    = $1B
GRP1    = $1C
HMP1    = $21
VDELP0  = $25
VDELP1  = $26
HMOVE   = $2A
HMCLR   = $2B
TIM64T  = $0296
INTIM   = $0284

WINDOW  = 24        ; 可視窓の行数
BMH     = 48        ; ビットマップ全高

row     = $80
tmp     = $81
offset  = $82       ; 窓オフセット 0..BMH-WINDOW
odir    = $83       ; 0=順方向/1=逆方向
fcnt    = $84
p0      = $90
p1      = $92
p2      = $94
p3      = $96
p4      = $98
p5      = $9A

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #$1C
        sta COLUP0
        sta COLUP1
        lda #$03
        sta NUSIZ0
        sta NUSIZ1
        lda #1
        sta VDELP0
        sta VDELP1
        lda #>Col0          ; 全列 1 ページ内前提（ORG で保証）
        sta p0+1
        sta p1+1
        sta p2+1
        sta p3+1
        sta p4+1
        sta p5+1

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
        lda #43
        sta TIM64T
        ; --- 窓スクロール（2 フレームに 1px・0..BMH-WINDOW をバウンド） ---
        inc fcnt
        lda fcnt
        and #1
        bne OffDone
        lda odir
        bne ODown
        inc offset
        lda offset
        cmp #BMH-WINDOW
        bcc OffDone
        lda #1
        sta odir
        bne OffDone
ODown:  dec offset
        bne OffDone
        lda #0
        sta odir
OffDone:
        ; --- 6 ポインタ = ColK + offset ---
        lda #<Col0
        clc
        adc offset
        sta p0
        lda #<Col1
        clc
        adc offset
        sta p1
        lda #<Col2
        clc
        adc offset
        sta p2
        lda #<Col3
        clc
        adc offset
        sta p3
        lda #<Col4
        clc
        adc offset
        sta p4
        lda #<Col5
        clc
        adc offset
        sta p5
VBwait: lda INTIM
        bne VBwait
        ; --- 位置決め（score6 と同一: P0=87/P1=95） ---
        sta WSYNC
        ds 13, $EA
        ds 9, $EA
        lda $80
        sta RESP0
        sta RESP1
        lda #$10
        sta HMP1
        sta WSYNC
        sta HMOVE
        ds 12, $EA
        sta HMCLR
        lda #0
        sta VBLANK

        ; ===== ビットマップ窓 24 行（6-store・1 行/走査線） =====
        lda #WINDOW-1
        sta row
Krow:   sta WSYNC
        ldy row
        lda (p0),y
        sta GRP0
        lda (p1),y
        sta GRP1
        lda (p2),y
        sta GRP0
        lda (p3),y
        sta tmp
        lda (p4),y
        tax
        lda (p5),y
        tay
        lda tmp
        sta GRP1
        stx GRP0
        sty GRP1
        sta GRP0
        dec row
        bpl Krow
        lda #0
        sta GRP0
        sta GRP1
        sta GRP0

        ldy #167
Fill:   sta WSYNC
        dey
        bne Fill

        lda #2
        sta VBLANK
        ldx #29
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

; ===== ビットマップ（列優先 6×48B・下の行から・1 ページ内） =====
        org $FE00
Col0:
        byte $00,$00,$00,$00,$00,$00,$00,$00
        byte $00,$00,$00,$00,$6C,$30,$FF,$FF
        byte $3C,$0F,$03,$00,$00,$00,$33,$CC
        byte $33,$CC,$00,$00,$6C,$30,$FF,$FF
        byte $3C,$0F,$03,$00,$00,$00,$00,$00
        byte $00,$00,$00,$00,$00,$00,$00,$00
Col1:
        byte $00,$00,$00,$00,$00,$00,$00,$00
        byte $00,$00,$00,$00,$0D,$03,$FF,$FF
        byte $F3,$FF,$FC,$F0,$00,$00,$33,$CC
        byte $33,$CC,$00,$00,$0D,$03,$FF,$FF
        byte $F3,$FF,$FC,$F0,$00,$00,$00,$00
        byte $00,$00,$00,$00,$00,$00,$00,$00
Col2:
        byte $00,$00,$00,$00,$00,$00,$00,$00
        byte $00,$00,$00,$00,$80,$00,$F0,$F0
        byte $C0,$00,$00,$00,$00,$00,$33,$CC
        byte $33,$CC,$00,$00,$80,$00,$F0,$F0
        byte $C0,$00,$00,$00,$00,$00,$00,$00
        byte $00,$00,$00,$00,$00,$00,$00,$00
Col3:
        byte $00,$00,$00,$00,$00,$00,$00,$00
        byte $00,$00,$00,$00,$1B,$0C,$3F,$3F
        byte $0F,$03,$00,$00,$00,$00,$33,$CC
        byte $33,$CC,$00,$00,$1B,$0C,$3F,$3F
        byte $0F,$03,$00,$00,$00,$00,$00,$00
        byte $00,$00,$00,$00,$00,$00,$00,$00
Col4:
        byte $00,$00,$00,$00,$00,$00,$00,$00
        byte $00,$00,$00,$00,$03,$00,$FF,$FF
        byte $3C,$FF,$FF,$3C,$00,$00,$33,$CC
        byte $33,$CC,$00,$00,$03,$00,$FF,$FF
        byte $3C,$FF,$FF,$3C,$00,$00,$00,$00
        byte $00,$00,$00,$00,$00,$00,$00,$00
Col5:
        byte $00,$00,$00,$00,$00,$00,$00,$00
        byte $00,$00,$00,$00,$60,$C0,$FC,$FC
        byte $F0,$C0,$00,$00,$00,$00,$33,$CC
        byte $33,$CC,$00,$00,$60,$C0,$FC,$FC
        byte $F0,$C0,$00,$00,$00,$00,$00,$00
        byte $00,$00,$00,$00,$00,$00,$00,$00

        org $FFFC
        .word Start
        .word Start
