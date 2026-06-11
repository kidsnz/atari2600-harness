; litmus_timer — RIOT タイマーの実機裏取り（V2-10）
; INTIM 連続読みでカウントダウン、アンダーフロー後の $FF ラップ/1per-cycle、TIMINT D7、INTIM 読みが D7 を消すか。
; 純 RAM 検証。RAM マップ:
;  A: TIM1T=$40 直後の INTIM 連続3読み（lda abs=5cy なので 5ずつ減るはず）→ $90,$91,$92
;  B: TIM1T=$02 → 満了通過 → $93=INTIM($FFラップ域) $94=TIMINT(D7立つ=bit7) $95=INTIM(さらに下) $96=INTIM読み後のTIMINT(D7消える?)
; 実機裏取り済（Gopher2600, v0.47.0）:
;  A カウントダウン: TIM1T=$40 直後の連続読み $90/$91/$92=$3C/$35/$2E（-7=読みループ実費, 1/cycle）。
;  B アンダーフロー: 満了後 $94=$EF（$FF域から 1/cycle 継続, $96=$E1 でさらに減）。
;  TIMINT D7: $93=$C0（INTIM 読む前=満了D7+D6 立つ）。
;  ★監査の問い: $95=$00 ＝ INTIM を読むと TIMINT がクリアされる（D7 が消える）。回帰固定=scenarios/timer.json。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
INTIM   = $0284
TIMINT  = $0285
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
        sta $2C             ; CXCLR

        ; --- A: カウントダウン（TIM1T=$40, 連続3読み）---
        lda #$40
        sta TIM1T
        lda INTIM
        sta $90
        lda INTIM
        sta $91
        lda INTIM
        sta $92

        ; --- B: アンダーフロー & TIMINT ---
        lda #$02
        sta TIM1T           ; 2 cycle で 0、以後 $FF から 1/cycle
        nop
        nop
        nop
        nop                 ; 満了を通過
        lda TIMINT
        sta $93             ; ★INTIM 読む前: D7(満了)立つはず
        lda INTIM
        sta $94             ; $FF ラップ域（この読みが D7 を消す説）
        lda TIMINT
        sta $95             ; ★INTIM 読んだ後: D7 消えてる?
        lda INTIM
        sta $96             ; さらに減（1/cycle 継続）

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
