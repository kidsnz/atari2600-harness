; litmus_lfsr — 8bit Galois LFSR の数学的性質を実機裏取り（V2-9, 手続き生成の基盤）
; RNG: lsr A; bcc skip; eor #$8E（DaveC Random-Dungeon / 一般的なゲーム乱数）。
; 純 RAM 検証（描画なし）。RAM マップ:
;  $90-$97 = seed $01 からの最初の8値
;  $9E = 255 ステップ中に $00 が一度も出ない → 1（ゼロ状態に落ちない＝非縮退）
;  $9F = 255 ステップ後に seed($01) へ戻る → 1（周期ちょうど 255）
; 実機裏取り済（Gopher2600, v0.46.0）: seed $01 から最初の8値 = 01,8E,47,AD,D8,6C,36,1B（手計算一致）。
; $9E=1（255ステップ中 $00 が一度も出ない＝非縮退）/ $9F=1（255ステップで seed に戻る＝周期ちょうど255）。
; 回帰固定=scenarios/lfsr.json（純 read_ram 数値）。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02

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
        sta $2C             ; CXCLR（init 副作用の衝突を一掃・決定化、smoke と同じ理由）

        ; --- 最初の8値を記録 ---
        lda #$01
        sta $90             ; v0 = seed
        ldx #1
Seq:    jsr Rnd
        sta $90,x
        inx
        cpx #8
        bne Seq

        ; --- 255 ステップ掃引: ゼロ非出現＋周期チェック ---
        lda #$01            ; seed
        ldy #1              ; ゼロ非出現フラグ（1=一度も0なし）
        ldx #255            ; 255 回
Sweep:  jsr Rnd
        cmp #$00
        bne NotZero
        ldy #0              ; 0 が出た
NotZero:
        dex
        bne Sweep
        sty $9E             ; ゼロ非出現フラグ
        cmp #$01            ; 255 ステップ後 == seed?
        beq Period
        lda #0
        sta $9F
        jmp Frames
Period:
        lda #1
        sta $9F

Frames:
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
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

        ; --- LFSR: A を1ステップ進めて返す ---
Rnd:    lsr
        bcc RndSkip
        eor #$8E
RndSkip:
        rts

        org $FFFC
        .word Start
        .word Start
