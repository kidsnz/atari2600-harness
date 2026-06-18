; multicolor48 — 48px multicolor graphic with per-row color (technique: multicolor48, AA2-209137)
; 系譜: AtariAge topic/209137（SeaGtGruff の 76cy 多色 48px カーネル）を score6/bitmap48 の
; 検証済み 6-store 振付に載せた完全形:
;   ・48px = NUSIZ $03（3 copies close）× P0/P1 ＋ P1 を +8px ＋ VDEL ダブルバッファ。
;   ・**各行で COLUP0/COLUP1 を ColorTab から書き換え**＝48px 画像が縦方向に多色化。
;   ・色書き換えは HBLANK 内（最初の GRP0 ストア前）で済ませ、6-store 本体は score6 と同一振付。
;   ・1 行 = 1 走査線、行内予算 76cy 未満（色 10cy ＋ 6-store）。
;   ・描く絵 = レインボーのハート（縦に色が変わる＝多色が一目で分かる）。
;   ・各行の実効色を RAM ミラー（colhi/collo）へ退避＝シナリオが「≥2 色」を数値検証できる。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
NUSIZ1  = $05
COLUP0  = $06
COLUP1  = $07
COLUBK  = $09
RESP0   = $10
RESP1   = $11
GRP0    = $1B
GRP1    = $1C
HMP1    = $21
VDELP0  = $25
VDELP1  = $26
HMOVE   = $2A
HMCLR   = $2B

HEIGHT  = 16        ; 画像の行数

row     = $80
tmp     = $81
colfst  = $82       ; 先頭行（row=HEIGHT-1）の色ミラー
collst  = $83       ; 末尾行（row=0）の色ミラー
fcnt    = $84
p0      = $90       ; 列ポインタ ×6（hi は初期化で固定・1 ページ内）
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
        sta COLUBK
        lda #$03            ; 3 copies close
        sta NUSIZ0
        sta NUSIZ1
        lda #1
        sta VDELP0
        sta VDELP1
        lda #>Col0          ; 列テーブルは 1 ページ内（ORG で保証）
        sta p0+1
        sta p1+1
        sta p2+1
        sta p3+1
        sta p4+1
        sta p5+1
        lda #<Col0
        sta p0
        lda #<Col1
        sta p1
        lda #<Col2
        sta p2
        lda #<Col3
        sta p3
        lda #<Col4
        sta p4
        lda #<Col5
        sta p5

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
        ; 影クリア（決定化）
        lda #0
        sta GRP0
        sta GRP1
        sta GRP0
        inc fcnt
        ldx #34
VB:     sta WSYNC
        dex
        bne VB
        ; --- 位置決め（score6 と同一: P0=87 / P1=95） ---
        sta WSYNC
        ds 13, $EA          ; SLEEP 26
        ds 9, $EA           ; SLEEP 18
        lda $80             ; +3 = 21cy
        sta RESP0
        sta RESP1
        lda #$10
        sta HMP1
        sta WSYNC
        sta HMOVE
        ds 12, $EA          ; SLEEP 24
        sta HMCLR
        lda #0
        sta VBLANK

        ; ===== 多色 48px カーネル（HEIGHT 行・色＋6-store・1 行/走査線） =====
        lda #HEIGHT-1
        sta row
Krow:   sta WSYNC          ; @lines 2 — each data row spans 2 visible scanlines (loop-back ~79cy>76); verified stable 262
        ldy row             ; 3
        lda ColorTab,y      ; 7   行毎の色
        sta COLUP0          ; 10
        sta COLUP1          ; 13
        lda (p0),y          ; 18
        sta GRP0            ; 21  B0→P0新
        lda (p1),y          ; 23
        sta GRP1            ; 26  B1→P1新, B0→P0影(表示)
        lda (p2),y          ; 31
        sta GRP0            ; 34  B2→P0新, B1→P1影(表示)
        lda (p3),y          ; 39
        sta tmp             ; 42
        lda (p4),y          ; 47
        tax                 ; 49
        lda (p5),y          ; 54
        tay                 ; 56
        lda tmp             ; 59
        sta GRP1            ; 62  B3→P1新, B2→P0影
        stx GRP0            ; 65  B4→P0新, B3→P1影
        sty GRP1            ; 68  B5→P1新, B4→P0影
        sta GRP0            ; 71  junk,    B5→P1影
        dec row             ; 76
        bpl Krow            ; (行内完結後の次行 WSYNC で揃う)

        ; 消灯（影まで）
        lda #0
        sta GRP0
        sta GRP1
        sta GRP0
        ; 色ミラー記録（検証用: 先頭行=HEIGHT-1, 末尾行=0）
        lda ColorTab+HEIGHT-1
        sta colfst
        lda ColorTab+0
        sta collst

        ldy #161
Fill:   sta WSYNC
        dey
        bne Fill

        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

; ===== 行毎カラー（下の行が先＝Y=HEIGHT-1..0 で参照・レインボー） =====
; 末尾(row0,画面最下)→先頭(row15,画面最上) の順。視覚的に虹。
ColorTab:
        byte $44,$44,$46,$46,$48,$48,$1A,$1A   ; row 0..7  : 赤→橙→黄緑
        byte $BA,$BA,$98,$98,$76,$76,$64,$64   ; row 8..15 : 水→青→紫→桃

; ===== 画像（列優先 6×HEIGHT・下の行が先・1 ページ内）= ハート =====
        org $FE00
; 各バイト = 8px。6 列で 48px。row0 が画面最下、row15 が最上。
; ハート: 上部に2山、下部はV字に絞る。
Col0:   ; 左端 8px
        byte $00,$00,$00,$01,$03,$07,$0F,$0F   ; row0..7
        byte $1F,$1F,$0F,$0E,$00,$00,$00,$00   ; row8..15
Col1:
        byte $00,$00,$00,$80,$F0,$F8,$FC,$FE   ; row0..7
        byte $FF,$FF,$FF,$FE,$3C,$3C,$00,$00   ; row8..15
Col2:
        byte $00,$00,$00,$01,$0F,$3F,$7F,$FF   ; row0..7
        byte $FF,$FF,$FF,$FF,$FF,$FF,$00,$00   ; row8..15
Col3:
        byte $00,$00,$00,$80,$F0,$FC,$FE,$FF   ; row0..7
        byte $FF,$FF,$FF,$FF,$FF,$FF,$00,$00   ; row8..15
Col4:
        byte $00,$00,$00,$00,$00,$80,$C0,$E0   ; row0..7
        byte $F0,$F0,$F8,$78,$3C,$3C,$00,$00   ; row8..15
Col5:   ; 右端 8px
        byte $00,$00,$00,$00,$00,$00,$00,$00   ; row0..7
        byte $00,$00,$00,$00,$00,$00,$00,$00   ; row8..15

        org $FFFC
        .word Start
        .word Start
