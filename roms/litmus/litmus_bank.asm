; litmus_bank — F8 (8K, 2バンク) bankswitch の実機裏取り（V2-5）
; ベストプラクティス準拠: 全バンクに ベクタ＋同一リセットスタブ（どのバンクで電源投入しても bank0 へ）。
; 毎フレーム overscan で ping-pong: bank0 が $80=$A0/$81++ → hotspot $FFF9 read で bank1 へ →
; bank1 が $80=$B1/$82++ → $FFF8 read で bank0 へ戻り rts。
; 期待: $80 はフレーム末に常に $B1（bank1 が走った証拠）、$81/$82 が同数で増える（毎フレーム往復）、
;       フレーム境界の read_bank/bank.number = 0（kernel は bank0）。
; 実機裏取り済（Gopher2600, v0.43.0）: 8192B → AUTO で F8 認識。5フレーム後 $80=$B1（bank1 実行の証拠）、
; $81=$82（毎フレーム両バンクを往復・同数）、フレーム境界 bank.number=0（kernel=bank0）。
; 回帰固定=scenarios/bank.json（新 resolver bank.number / 新ツール read_bank も本件で追加）。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02

; ================= bank 0 =================
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
        ldy #192
Vis:    sta WSYNC
        dey
        bne Vis
        lda #2
        sta VBLANK
        ; --- overscan: bank0 の印 → bank1 へ ping-pong ---
        lda #$A0
        sta $80
        inc $81
        jsr PingPong        ; $FF00（中で bank1 を経由して戻る）
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

        ; --- 切替ゾーン（bank0 側）: $FF00 ---
        ORG  $0F00
        RORG $FF00
PingPong:
        lda $FFF9           ; hotspot → bank1 が map。次の fetch $FF03 は bank1 のコード
        ds 9, $EA           ; $FF03-$FF0B は bank1 が実行（bank0 側は不使用 NOP 埋め）
        rts                 ; $FF0C: bank1 が $FFF8 を読んだ直後にここ（bank0）へ戻る

        ; --- リセットスタブ（両バンク同一・同位置）---
        ORG  $0FE0
        RORG $FFE0
        lda $FFF8           ; どのバンクで起動しても bank0 を選択
        jmp $F000

        ORG  $0FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0

; ================= bank 1 =================
        ORG  $1000
        RORG $F000
        ds 3, 0             ; bank1 の $F000 は不使用

        ; --- 切替ゾーン（bank1 側）: $FF00 ---
        ORG  $1F00
        RORG $FF00
        ds 3, $EA           ; $FF00-$FF02 は bank0 が実行（不使用）
        lda #$B1            ; $FF03: ここから bank1 のコード
        sta $80
        inc $82
        lda $FFF8           ; $FF09: hotspot → bank0 へ戻る。次の fetch $FF0C は bank0 の rts

        ; --- リセットスタブ（bank0 と同一）---
        ORG  $1FE0
        RORG $FFE0
        lda $FFF8
        jmp $F000

        ORG  $1FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0
