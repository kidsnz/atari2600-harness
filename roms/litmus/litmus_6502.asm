; litmus_6502 — 6502/6507 の精度事実を実機裏取り（V2-6, 出典: 6502.org）
; TIM1T（1 cycle/tick）で命令列の経過サイクルを ROM 自身が測り、結果を RAM に記録 → read_ram/scenario で assert。
; 各測定は「TIM1T=$80 設定 → 被測定命令 → lda INTIM」の差分（共通オーバーヘッドは比較で相殺・絶対値で固定）。
; RAM マップ:
;  $90=BCD A結果($99+$01,SED→期待$00) $91=その P（C=bit0 期待1 / Z=bit1 は NMOS では信頼不可の実録）
;  $92=JMP($xxFF)バグ経路マーカー（$A5=バグ経路=NMOS 正）
;  $93=LDA abs,X 跨ぎ無 / $94=跨ぎ有（差+1 期待） $95=STA abs,X 跨ぎ無 / $96=跨ぎ有（差0 期待=ストア固定）
;  $97=BNE 不成立 / $98=成立（差+1） $9A=成立+ページ跨ぎ（さらに+1）
;  $99=DCP zp（違法命令, 5cy）
; 実機裏取り済（Gopher2600, v0.44.0）— 全測定が 6502.org と一致:
;  BCD: $90=$00（A 正）/$91=$BD（C=1 正・Z=0=NMOS では信頼不可を実録・N=1 同様）。JMP($F3FF): $92=$A5（バグ経路）。
;  LDA abs,X: 窓8/9（4cy→跨ぎ5cy=+1）。STA abs,X: 窓9/9（5cy 固定＝ストア無ペナルティ→kernel 決定性の根拠）。
;  BNE: 窓8/9/10（2cy 不成立 / 3cy 成立 / 4cy 成立+跨ぎ）。DCP zp（違法 $C7）: 窓9（5cy・違法命令サポートも立証）。
;  回帰固定=scenarios/cpu6502.json。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
INTIM   = $0284
TIM1T   = $0294

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

        ; --- (1) NMOS BCD: $99+$01 ---
        sed
        clc
        lda #$99
        adc #$01
        php
        cld
        sta $90
        pla
        sta $91

        ; --- (2) JMP ($F3FF) バグ ---
        jmp ($F3FF)
Cont2:

        ; --- (3) LDA abs,X 跨ぎ無/有 ---
        ldx #$05
        lda #$80
        sta TIM1T
        lda $F010,x         ; $F015 同ページ (4cy)
        lda INTIM
        eor #$FF            ; 経過 = $80 - INTIM → 記録は (INTIM xor FF) で単調化せず素のまま差し引き
        sta $93
        lda #$80
        sta TIM1T
        lda $F0FB,x         ; $F100 ページ跨ぎ (5cy)
        lda INTIM
        eor #$FF
        sta $94

        ; --- (4) STA abs,X 跨ぎ無/有（ROM 空間へ書く＝無害） ---
        lda #$80
        sta TIM1T
        sta $F010,x         ; 同ページ (5cy 固定)
        lda INTIM
        eor #$FF
        sta $95
        lda #$80
        sta TIM1T
        sta $F0FB,x         ; 跨ぎ (5cy 固定のはず)
        lda INTIM
        eor #$FF
        sta $96

        ; --- (5) BNE 不成立/成立（lda を計測窓の内側に置く: ldy が Z を潰すため）---
        ldy #$80
        sty TIM1T
        lda #$00            ; 2cy, Z=1
        bne NT1             ; 不成立 (2cy) → 窓 = 2+2+4
NT1:    lda INTIM
        eor #$FF
        sta $97
        ldy #$80
        sty TIM1T
        lda #$01            ; 2cy, Z=0
        bne TK1             ; 成立・同ページ (3cy) → 窓 = 2+3+4
TK1:    lda INTIM
        eor #$FF
        sta $98

        ; --- (6) DCP zp（違法 $C7, 5cy）---
        lda #$10
        sta $A0
        lda #$80
        sta TIM1T
        .byte $C7, $A0      ; DCP $A0
        lda INTIM
        eor #$FF
        sta $99

        ; --- (7) BNE 成立＋ページ跨ぎ（専用セグメントへ）---
        jmp CrossSeg

        ; JMP バグ用の経路（バグ: high を $F300 から読む → $F440 へ飛ぶ）
        org $F300
        .byte $F4           ; ← バグ時の high byte
        org $F3FF
        .byte $40           ; pointer low（$F3FF）
        .byte $F5           ; $F400 = 正しい high（バグ無しなら $F540 へ）

        org $F440
        lda #$A5            ; バグ経路（NMOS 正）
        sta $92
        jmp Cont2
        org $F540
        lda #$5A            ; 非バグ経路（来ないはず）
        sta $92
        jmp Cont2

        ; BNE 成立＋跨ぎ: 分岐命令の次アドレスとターゲットが別ページになる配置
        org $F5F4
CrossSeg:
        ldy #$80            ; $F5F4-F5
        sty TIM1T           ; $F5F6-F8
        lda #$01            ; $F5F9-FA, Z=0（計測窓内 2cy）
        bne CrossTgt        ; $F5FB-FC → 次アドレス $F5FD、ターゲット $F601（跨ぎ, 4cy）→ 窓 = 2+4+4
        nop
        nop
        nop
        nop
        org $F601
CrossTgt:
        lda INTIM
        eor #$FF
        sta $9A
        jmp Frames

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

        org $FFFC
        .word Start
        .word Start
