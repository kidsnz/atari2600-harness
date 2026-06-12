; two_line_vdel — 2LK の VDEL 偶奇技（technique #4 の verified refinement）
; 2-line kernel の代償「縦2行刻み」を VDEL で破る: VDELP0=1 にすると GRP0 書込は影に入り、
; **GRP1 書込の瞬間に表示へコピー**される。本 kernel は A 行=GRP0 / B 行=GRP1 の構造なので、
; VDELP0 を立てるだけで P0 の絵が 1 走査線下へずれる＝Y の偶奇を VDELP0 ビットで選べる。
; P0 の Y は走査線単位（8..170 を ±1 ピンポン）。毎フレーム VDELP0 = y0&1。kernel は無改造。
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
VDELP0  = $25

SPRITE_H = 8        ; ペア単位（=16走査線ぶん）
Y_MIN    = 8        ; P0 は走査線単位（VDEL 偶奇で 1px 刻み）
Y_MAX    = 170
Y_MINP   = 4        ; P1 はペア単位のまま（コントロール）
Y_MAXP   = 84
X0       = 60
X1       = 100

y0       = $80      ; P0 上端（走査線 0-191。kernel 用ペア値は y0p に前計算）
y1       = $81      ; P1 上端（ペア）
y0p      = $84      ; y0>>1（kernel の比較用）
d0       = $82      ; P0: 0=下/1=上
d1       = $83      ; P1: 0=下/1=上
sent     = $9E

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
        lda #$1E
        sta COLUP0          ; 黄
        lda #$5C
        sta COLUP1          ; 桃
        lda #Y_MIN
        sta y0
        lda #Y_MAXP
        sta y1
        lda #1
        sta d1              ; P1 は上向きから（逆相）
        lda #$C5
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
        ; --- VB行1: 両スプライトの縦移動（ペア単位のピンポン）---
        lda d0
        bne M0u
        inc y0
        lda y0
        cmp #Y_MAX
        bcc M0d
        lda #1
        sta d0
M0d:    jmp M1
M0u:    dec y0
        lda y0
        cmp #Y_MIN+1
        bcs M1
        lda #0
        sta d0
M1:     lda d1
        bne M1u
        inc y1
        lda y1
        cmp #Y_MAXP
        bcc M1d
        lda #1
        sta d1
M1d:    jmp MvEnd
M1u:    dec y1
        lda y1
        cmp #Y_MINP+1
        bcs MvEnd
        lda #0
        sta d1
MvEnd:  lda y0
        lsr                 ; ペア値（kernel 比較用）。carry = 偶奇
        sta y0p
        lda #0
        adc #0              ; A = y0&1（carry から）
        sta VDELP0          ; ★偶奇ビット → VDEL ON なら P0 が 1 走査線下がる
        sta WSYNC           ; VB 1
        ; --- VB行2-4: P0/P1 を粗+微でセット → 共有 HMOVE を 1 発 ---
        ; HMOVE を行毎に打つと前行の HMxx が二重適用される（実測 +3px のバグで発見）。
        ; 両者の HMxx/RESPx を先に揃え、最後の WSYNC 直後に 1 回だけ HMOVE するのが正攻法。
        lda #X0
        clc
        adc #XCAL
        sec
Dv0:    sbc #15
        bcs Dv0
        tay
        lda HMOVE_LUT,y
        sta HMP0
        sta RESP0
        sta WSYNC           ; VB 2
        lda #X1
        clc
        adc #XCAL
        sec
Dv1:    sbc #15
        bcs Dv1
        tay
        lda HMOVE_LUT,y
        sta HMP1
        sta RESP1
        sta WSYNC           ; VB 3
        sta HMOVE           ; 1 発で P0/P1 両方に適用
        ldx #33             ; VBLANK 残り（1+1+1+33+1=37）
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        sta WSYNC           ; VB 37 → 可視開始

        ; --- 可視 192 行 = 96 ペア × 2 行（A 行: P0+背景 / B 行: P1）---
        ldy #0
Pair:   sta WSYNC           ; --- A 行 ---
        tya                 ; 2
        sec                 ; 4
        sbc y0p             ; 7
        cmp #SPRITE_H       ; 9
        bcc PD0             ; 11/12
        lda #0              ; 13
        beq PS0             ; 16
PD0:    tax                 ; 14
        lda ArtA,x          ; 18
PS0:    sta GRP0            ; ~21
        tya                 ; 背景グラデ（8 ペア毎に1段）
        lsr
        lsr
        lsr
        ora #$90            ; y/8 ≤ 11 < 16 なので加算と等価（adc だと直前の carry で縞が揺れる）
        sta COLUBK          ; ~35
        sta WSYNC           ; --- B 行 ---
        tya                 ; 2
        sec                 ; 4
        sbc y1              ; 7
        cmp #SPRITE_H       ; 9
        bcc PD1             ; 11/12
        lda #0              ; 13
        beq PS1             ; 16
PD1:    tax                 ; 14
        lda ArtB,x          ; 18
PS1:    sta GRP1            ; ~21
        iny
        cpy #96
        bne Pair            ; ~28

        lda #2
        sta VBLANK
        lda #0
        sta GRP0
        sta GRP1
        sta COLUBK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame       ; 3+37+192+30 = 262 を明示所有

XCAL = -5                   ; lda #imm プロローグ（vertical_pos と同型）。実測で確認すること

ArtA:   ; 8 ペア行 = 16 走査線の菱形（自作）
        byte %00011000
        byte %00111100
        byte %01111110
        byte %11111111
        byte %11111111
        byte %01111110
        byte %00111100
        byte %00011000
ArtB:   ; 8 ペア行の枠（自作）
        byte %11111111
        byte %10000001
        byte %10111101
        byte %10100101
        byte %10100101
        byte %10111101
        byte %10000001
        byte %11111111

HMOVE_TBL:
        byte $80,$70,$60,$50,$40,$30,$20,$10,$00,$F0,$E0,$D0,$C0,$B0,$A0,$90
HMOVE_TBL_END:
HMOVE_LUT = HMOVE_TBL_END - 256

        org $FFFC
        .word Start
        .word Start
