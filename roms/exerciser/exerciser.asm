; exerciser — 全能力統合のショーケース ROM（Road to 1.0）
; M1: F8 2バンク骨格＋シーンディスパッチ＋fire 切替＋プレースホルダ2シーン（S1 は bank1 常駐=クロスバンク証明）。
; 構造は litmus_bank（V2-5 検証済み）の型: 全バンクにベクタ＋同一リセットスタブ＋同位置切替ゾーン($FF00)。
; シーン契約: 「ちょうど 192 scanline を消費するサブルーチン」。フレーム骨格(VSYNC/VBLANK/overscan)は bank0 が所有。
;
; RAM マップ（グローバル $80-$9F）:
;  $80 = scene id（fire 押下エッジで +1 mod NSCENES）
;  $81 = 前フレームの INPT4 D7（エッジ検出用）
;  $82 = フレームカウンタ（デバッグ用）
;  $9E = scene0 実行センチネル($A0) / $9F = scene1 実行センチネル($B1)＝クロスバンク実行の数値証拠
; 実機裏取り済（Gopher2600, v0.56.0）: scene0=青/$9E=$A0、fire エッジで scene1=緑/$9F=$B1（bank1 実行の数値証拠）、
; 再 fire で wrap。フレーム境界 read_bank=0（フレームワーク=bank0）。8192B=AUTO で F8。回帰固定=scenarios/m1_skeleton.json。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
INPT4   = $3C

NSCENES = 2

scene   = $80
prevFire = $81
frameCt = $82

; ================= bank 0: フレームワーク＋S0 =================
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
        sta $2C             ; CXCLR（init 副作用の決定化）

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
        ; --- 入力（VBLANK 1行目）: fire 押下エッジで scene++ ---
        lda INPT4
        and #$80            ; now (0=押下)
        tay
        cmp prevFire
        beq NoEdge
        cmp #0
        bne NoEdge          ; 離した側のエッジは無視
        ; 押下エッジ
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

        ; --- シーンディスパッチ（各シーン=192行ちょうど消費するサブルーチン）---
        lda scene
        beq DoS0
        jsr CallB1Scene     ; scene 1 は bank1 常駐
        jmp AfterScene
DoS0:   jsr Scene0
AfterScene:

        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

; --- S0: プレースホルダ（青背景, bank0 常駐）---
Scene0:
        lda #$A0
        sta $9E             ; 実行センチネル
        lda #$84
        sta COLUBK          ; 青
        ldy #192
S0L:    sta WSYNC
        dey
        bne S0L
        lda #0
        sta COLUBK
        rts

        ; --- 切替ゾーン（bank0 側, $FF00）---
        ORG  $0F00
        RORG $FF00
CallB1Scene:
        lda $FFF9           ; → bank1。次の fetch $FF03 は bank1 のコード
        ds 6, $EA           ; $FF03-$FF08 は bank1 が実行
        rts                 ; $FF09: bank1 が $FFF8 を読んだ直後ここ（bank0）→ 呼び元へ戻る

        ; --- リセットスタブ（両バンク同一・同位置）---
        ORG  $0FE0
        RORG $FFE0
        lda $FFF8           ; どのバンクで起動しても bank0 へ
        jmp $F000

        ORG  $0FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0

; ================= bank 1: S1（クロスバンク・シーン）=================
        ORG  $1000
        RORG $F000
Scene1:
        lda #$B1
        sta $9F             ; 実行センチネル（bank1 で走った数値証拠）
        lda #$C6
        sta COLUBK          ; 緑
        ldy #192
S1L:    sta WSYNC
        dey
        bne S1L
        lda #0
        sta COLUBK
        rts

        ; --- 切替ゾーン（bank1 側, $FF00）---
        ORG  $1F00
        RORG $FF00
        ds 3, $EA           ; $FF00-02 は bank0 が実行（不使用）
        jsr Scene1          ; $FF03-05: bank1 のシーンを呼ぶ
        lda $FFF8           ; $FF06-08: → bank0。次の fetch $FF09 = bank0 の rts

        ; --- リセットスタブ（bank0 と同一）---
        ORG  $1FE0
        RORG $FFE0
        lda $FFF8
        jmp $F000

        ORG  $1FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0
