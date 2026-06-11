; exerciser — 全能力統合のショーケース ROM（Road to 1.0）
; M1: F8 2バンク骨格＋シーンディスパッチ＋fire 切替（v0.56.0 検証済）。
; M2: S0=タイトル（bank1）: 48px "EXRCSR" ロゴ（6書換 kernel・テーブル参照版）＋6桁 BCD スコア。
;
; シーン契約: 「ちょうど 192 scanline を消費するサブルーチン」。フレーム骨格は bank0 が所有。
; シーン: 0=Title(bank1) / 1=Blue placeholder(bank0)。fire 押下エッジで前進・wrap。
;
; RAM マップ:
;  グローバル: $80=scene  $81=prevFire  $82=frameCt  $90-$92=スコアBCD(6桁,LE)  $9E/$9F=実行センチネル
;  Title ローカル: $A0=row  $A1-$A3=t3,t4,t5  $A4-$A6=t0,t1,t2  $A8-$B3=p0..p5(各2B, digit ptr)
;
; 48px ロゴ kernel（litmus_48px6 の検証済み振付のテーブル参照版・1ライン/行・行倍化テーブルで16行）:
;  tail(前行末): B5→t5, B0→GRP0, B1→A, WSYNC
;  head: sta GRP1(B1)@3 → B2→GRP0@10 → B3/B4/B5→A/X/Y@21 → SLEEP10 → timed store 完了 34/37/40/43
; スコア kernel（2ライン交互=stage/draw, 縞表示）:
;  stage 行: (p0..p5),y から t0..t5 へ（76cy 内で余裕）
;  draw 行: zp 21cy プリロード → SLEEP4 → timed 34/37/40/43（litmus_48px6 と同一）
; 実機裏取り済（Gopher2600, v0.57.0）: 48px "EXRCSR" 可読（P0=24/P1=32, NUSIZ=3, VDEL, timed 34/37/40/43）。
; 6桁BCDスコアがフレーム毎 +1（frame123 で "000122" 表示・$90-92 BCD 一致）。スコアは stage/draw 交互（縞）。
; 修正史: ①ロゴ表 align 256（abs,y ページ跨ぎ+1cy のシアー） ②ループ折返し jmp を WSYNC 後 0-2cy に置く +3 シフト head
; ③stage 行頭でスプライト消灯（VDEL 影の残骸） ④GLYPH を逆順格納（row 降順描画で正立）。回帰固定=scenarios/m2_title.json。
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
HMOVE   = $2A
HMCLR   = $2B
VDELP0  = $25
VDELP1  = $26
INPT4   = $3C

NSCENES = 2

scene    = $80
prevFire = $81
frameCt  = $82
score0   = $90      ; BCD 下2桁
score1   = $91
score2   = $92      ; BCD 上2桁
sent0    = $9E
sent1    = $9F
row      = $A0
t3       = $A1
t4       = $A2
t5       = $A3
t0       = $A4
t1       = $A5
t2       = $A6
p0       = $A8      ; digit ptr ×6（各2B: $A8,$AA,$AC,$AE,$B0,$B2）
p1       = $AA
p2       = $AC
p3       = $AE
p4       = $B0
p5       = $B2

; ================= bank 0: フレームワーク＋S1 =================
        ORG  $0000
        RORG $F000
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
        ; --- 入力: fire 押下エッジで scene++（wrap）---
        lda INPT4
        and #$80
        tay
        cmp prevFire
        beq NoEdge
        cmp #0
        bne NoEdge
        inc scene
        lda scene
        cmp #NSCENES
        bcc NoWrap
        lda #0
        sta scene
NoWrap:
NoEdge: sty prevFire
        inc frameCt
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; --- シーンディスパッチ ---
        lda scene
        beq DoTitle
        jsr SceneBlue       ; scene 1（bank0）
        jmp AfterScene
DoTitle:
        jsr CallB1Scene     ; scene 0 = Title（bank1）
AfterScene:

        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

; --- S1: プレースホルダ（青背景, bank0）---
SceneBlue:
        lda #$A0
        sta sent0
        lda #$84
        sta COLUBK
        ldy #192
SBL:    sta WSYNC
        dey
        bne SBL
        lda #0
        sta COLUBK
        rts

        ; --- 切替ゾーン（bank0 側, $FF00）---
        ORG  $0F00
        RORG $FF00
CallB1Scene:
        lda $FFF9
        ds 6, $EA
        rts                 ; $FF09

        ORG  $0FE0
        RORG $FFE0
        lda $FFF8
        jmp $F000

        ORG  $0FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0

; ================= bank 1: S0 タイトル =================
        ORG  $1000
        RORG $F000
TitleScene:
        lda #$B1
        sta sent1
        ; スプライト構成（48px: NUSIZ 3コピー, P0=24/P1=+8, VDEL on）
        lda #$03
        sta NUSIZ0
        sta NUSIZ1
        lda #1
        sta VDELP0
        sta VDELP1
        lda #$0E
        sta COLUP0
        sta COLUP1
        lda #0
        sta COLUBK

        ; --- 位置決め（2行）: litmus_48px の検証済みレシピ ---
        sta WSYNC
        ds 13, $EA          ; SLEEP 26
        sta RESP0
        sta RESP1
        lda #$10
        sta HMP1
        sta WSYNC
        sta HMOVE
        ds 12, $EA          ; SLEEP 24
        sta HMCLR
        sta WSYNC           ; HMOVE 行をここで閉じる（閉じないと計算と合体して 93cy=2 scanline 跨ぎ→263 行）
        ; ここまで 3 行

        ; --- 計算行 1: BCD スコア +1 ＋ ptr hi 設定 ---
        sed
        clc
        lda score0
        adc #1
        sta score0
        lda score1
        adc #0
        sta score1
        lda score2
        adc #0
        sta score2
        cld
        lda #>Digits
        sta p0+1
        sta p1+1
        sta p2+1
        sta p3+1
        sta p4+1
        sta p5+1
        sta WSYNC           ; 計算行 1 終了（3 行目）

        ; --- 計算行 2: digit ptr lo（nibble*16; 上位 nibble は AND $F0 でそのまま）---
        ; 表示順: p0=score2上位 p1=score2下位 p2=score1上位 ... p5=score0下位
        lda score2
        and #$F0
        sta p0
        lda score2
        and #$0F
        asl
        asl
        asl
        asl
        sta p1
        lda score1
        and #$F0
        sta p2
        lda score1
        and #$0F
        asl
        asl
        asl
        asl
        sta p3
        sta WSYNC           ; 計算行 2（4 行目）
        lda score0
        and #$F0
        sta p4
        lda score0
        and #$0F
        asl
        asl
        asl
        asl
        sta p5
        sta WSYNC           ; 計算行 3（5 行目）

        ; --- 上余白: 35 行（ここまで 5 行 → 40 行）---
        ldy #35
TB1:    sta WSYNC
        dey
        bne TB1

        ; --- ロゴ 16 行（行倍化テーブル, 1ライン kernel）---
        ; ループ構造: tail 末尾で sta WSYNC(74) → 次行頭で jmp(0-2) → head は +3 シフト設計。
        ; プリステージ（row=15 の分）。GRP0=B0 書込は VDEL の影側が 0 のため画面には出ない。
        ldy #15
        sty row
        lda TblR2,y
        sta t5
        lda TblE,y
        sta GRP0            ; B0→P0新
        lda TblX,y          ; B1 を A に
        sta WSYNC
        jmp LogoHead        ; 次行の cycle 0-2 で実行（毎イテレーション同型）
LogoHead:
        sta GRP1            ; 6   B1→P1新（B0→P0影）
        lda TblR1,y         ; 10  B2
        sta GRP0            ; 13  B2→P0新（B1→P1影）
        lda TblC,y          ; 17  B3
        ldx TblS,y          ; 21  B4
        ldy t5              ; 24  B5
        cmp t5              ; 27  SLEEP 7（フラグのみ・A 不変）
        ds 2, $EA           ; 31
        sta GRP1            ; 34  B3→P1新, B2→P0影
        stx GRP0            ; 37  B4→P0新, B3→P1影
        sty GRP1            ; 40  B5→P1新, B4→P0影
        sta GRP0            ; 43  junk,    B5→P1影
        ldy row             ; 46
        dey                 ; 48
        sty row             ; 51
        bmi LogoDone        ; 53（不成立）
        lda TblR2,y         ; 57  次行 B5
        sta t5              ; 60
        lda TblE,y          ; 64  次行 B0
        sta GRP0            ; 67
        lda TblX,y          ; 71  次行 B1（A 保持）
        sta WSYNC           ; 74 → stall → 次行頭の jmp へ
        jmp LogoHead
LogoDone:
        lda #0
        sta GRP0
        sta GRP1
        sta GRP0            ; 影もクリア
        sta WSYNC           ; LogoDone の行を閉じる（17 行目）

        ; --- 中余白: 29 行（ここまで 40+17 → 86 行）---
        ldy #29
TB2:    sta WSYNC
        dey
        bne TB2

        ; --- スコア 32 行（16 行ぶんの行倍化 digit を stage/draw 交互で）---
        ldy #15
        sty row
ScoreLoop:
        ; stage 行: 頭でスプライト消灯（影まで, cycle 11 までに完了=表示前）→ t0..t5 ← (p0..p5),row
        sta WSYNC
        lda #0              ; 2
        sta GRP0            ; 5
        sta GRP1            ; 8   （P0影←0）
        sta GRP0            ; 11  （P1影←0）
        ldy row             ; 14
        lda (p0),y          ; 19
        sta t0              ; 11
        lda (p1),y          ; 16
        sta t1              ; 19
        lda (p2),y          ; 24
        sta t2              ; 27
        lda (p3),y          ; 32
        sta t3              ; 35
        lda (p4),y          ; 40
        sta t4              ; 43
        lda (p5),y          ; 48
        sta t5              ; 51
        ; draw 行: zp プリロード → timed（litmus_48px6 と同一形）
        sta WSYNC
        lda t0              ; 3
        sta GRP0            ; 6   B0
        lda t1              ; 9
        sta GRP1            ; 12  B1（B0→影）
        lda t2              ; 15
        sta GRP0            ; 18  B2（B1→影）
        lda t3              ; 21
        ldx t4              ; 24
        ldy t5              ; 27
        ds 2, $EA           ; 31  SLEEP 4
        sta GRP1            ; 34
        stx GRP0            ; 37
        sty GRP1            ; 40
        sta GRP0            ; 43
        ldy row             ; 46
        dey                 ; 48
        sty row             ; 51
        bpl ScoreLoop       ; 54
        lda #0
        sta GRP0
        sta GRP1
        sta GRP0
        sta WSYNC           ; 閉じ行（86+32+1=119 行目）

        ; --- 下余白: 72 行（合計 192）---
        ldy #72
TB3:    sta WSYNC
        dey
        bne TB3
        rts

; --- フォント（8×8, 各行を2回置いた 16B/グリフ＝行倍化） ---
        ; ロゴ "EXRCSR": B0=E B1=X B2=R B3=C B4=S B5=R
        MAC GLYPH           ; 8 行を倍化して並べる（kernel は row=15→0 の降順で描く＝最下バイトが画面最上段。
        ; よって行は逆順（{8}が先頭）に格納して、画面では {1}=上段 になる）
        .byte {8},{8},{7},{7},{6},{6},{5},{5},{4},{4},{3},{3},{2},{2},{1},{1}
        ENDM

        align 256
TblE:   GLYPH %01111110,%01100000,%01100000,%01111100,%01100000,%01100000,%01111110,%00000000
TblX:   GLYPH %01100110,%01100110,%00111100,%00011000,%00111100,%01100110,%01100110,%00000000
TblR1:  GLYPH %01111100,%01100110,%01100110,%01111100,%01101100,%01100110,%01100110,%00000000
TblC:   GLYPH %00111100,%01100110,%01100000,%01100000,%01100000,%01100110,%00111100,%00000000
TblS:   GLYPH %00111110,%01100000,%01100000,%00111100,%00000110,%00000110,%01111100,%00000000
TblR2:  GLYPH %01111100,%01100110,%01100110,%01111100,%01101100,%01100110,%01100110,%00000000

        ; 数字 0-9（16B ストライド・ページ内に収める）
        align 256
Digits:
        GLYPH %00111100,%01100110,%01101110,%01110110,%01100110,%01100110,%00111100,%00000000 ; 0
        GLYPH %00011000,%00111000,%00011000,%00011000,%00011000,%00011000,%01111110,%00000000 ; 1
        GLYPH %00111100,%01100110,%00000110,%00001100,%00110000,%01100000,%01111110,%00000000 ; 2
        GLYPH %00111100,%01100110,%00000110,%00011100,%00000110,%01100110,%00111100,%00000000 ; 3
        GLYPH %00001100,%00011100,%00111100,%01101100,%01111110,%00001100,%00001100,%00000000 ; 4
        GLYPH %01111110,%01100000,%01111100,%00000110,%00000110,%01100110,%00111100,%00000000 ; 5
        GLYPH %00111100,%01100000,%01111100,%01100110,%01100110,%01100110,%00111100,%00000000 ; 6
        GLYPH %01111110,%00000110,%00001100,%00011000,%00110000,%00110000,%00110000,%00000000 ; 7
        GLYPH %00111100,%01100110,%00111100,%01100110,%01100110,%01100110,%00111100,%00000000 ; 8
        GLYPH %00111100,%01100110,%01100110,%00111110,%00000110,%00000110,%00111100,%00000000 ; 9

        ; --- 切替ゾーン（bank1 側, $FF00）---
        ORG  $1F00
        RORG $FF00
        ds 3, $EA
        jsr TitleScene      ; $FF03
        lda $FFF8           ; $FF06 → bank0

        ORG  $1FE0
        RORG $FFE0
        lda $FFF8
        jmp $F000

        ORG  $1FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0
