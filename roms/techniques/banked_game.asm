; banked_game — バンク切替ゲーム構造テンプレート（technique: bankswitching, U-M8, F8 8K）
; litmus_bank（F8/F6/F4 検証済み v0.43.0）の authoring 側。実ゲームの標準形 3 点セット:
;   ① 全バンク同一のリセットスタブ＋ベクタ（どのバンクで電源投入しても bank0 へ）
;   ② 汎用クロスバンク・トランポリン（$FF80 振付）:
;        bank0 $FF80: lda $FFF9   ; bank1 選択 → 次フェッチ $FF83 は bank1
;        bank1 $FF83: jmp B1Work  ; bank1 側の仕事へ
;        ...仕事...   jmp $FF86
;        bank1 $FF86: lda $FFF8   ; bank0 復帰 → 次フェッチ $FF89 は bank0
;        bank0 $FF89: rts         ; 呼び出し元（bank0 の jsr $FF80）へ
;      ★ホットスポット $FFF8/9 の上に命令を置かないこと: フェッチ＝読み出しで
;        バンクが切替わる（$FFF9 に rts を置いてリブートループ化した実測バグ）
;   ③ データバンク参照: bank1 のレベル表を VBLANK で zp バッファへコピー（レベルロード）
; デモ: 120 フレーム毎にレベル切替 → bank1 のローダが PF パターン 8 バイトを $90-97 へ。
; kernel は bank0 がバッファから PF1 を 8 バンド描画。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUPF  = $08
PF1     = $0E

level   = $80       ; 0/1
fc      = $81
buf     = $90       ; レベル PF パターン ×8

; ================= bank 0（ゲーム本体） =================
        ORG  $0000
        RORG $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #$5A
        sta COLUPF
        jsr $FF80           ; 初回レベルロード（level=0）

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
        ; --- 120 フレーム毎にレベル切替 → bank1 ローダ ---
        inc fc
        lda fc
        cmp #120
        bcc NoSwitch
        lda #0
        sta fc
        lda level
        eor #1
        sta level
        jsr $FF80           ; クロスバンク呼び出し（bank1 がバッファを書き換える）
NoSwitch:
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; ===== 可視 192 行: バッファから PF1 を 8 バンド（24 行毎） =====
        ldx #0              ; バンド
KBand:  lda buf,x
        sta PF1
        ldy #24
KRow:   sta WSYNC
        dey
        bne KRow
        inx
        cpx #8
        bne KBand
        lda #0
        sta PF1

        lda #2
        sta VBLANK
        ldx #29
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

        ; --- トランポリン → リセットスタブ（bank0 側・ORG 昇順） ---
        ORG  $0F80
        RORG $FF80
        lda $FFF9           ; $FF80-2: bank1 選択
        ds 6, $EA           ; $FF83-8: bank1 が実行（bank0 側は不使用）
        rts                 ; $FF89: bank1 から復帰した直後にここ

        ORG  $0FE0
        RORG $FFE0
        lda $FFF8           ; リセットスタブ: 必ず bank0 で起動
        jmp $F000

        ORG  $0FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0

; ================= bank 1（データ＋ローダ） =================
        ORG  $1000
        RORG $F000
B1Work:                     ; レベルロード: LvTab[level*8..+7] → buf
        ldx #0
        lda level
        beq B1Copy
        ldx #8
B1Copy: ldy #0
B1Loop: lda LvTab,x
        sta buf,y
        inx
        iny
        cpy #8
        bne B1Loop
        jmp $FF86           ; bank0 へ復帰（トランポリン後半）

LvTab:  byte $81,$42,$24,$18,$18,$24,$42,$81   ; level 0（X 形）
        byte $FF,$7E,$3C,$18,$18,$3C,$7E,$FF   ; level 1（ダイヤ形）

        ; --- トランポリン → リセットスタブ（bank1 側・ORG 昇順） ---
        ORG  $1F80
        RORG $FF80
        ds 3, $EA           ; $FF80-2: bank0 が実行（不使用）
        jmp B1Work          ; $FF83-5: 入口
        lda $FFF8           ; $FF86-8: bank0 復帰
        ds 1, $EA           ; $FF89: bank0 の rts（こちら側は不使用）

        ORG  $1FE0
        RORG $FFE0
        lda $FFF8
        jmp $F000

        ORG  $1FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0
