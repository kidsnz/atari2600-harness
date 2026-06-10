; litmus_overrun.asm — per-scanline 予算ガード（B-3）の検証用 ROM
; 目的: 「ある可視ラインが 76 CPU サイクル予算を超えて物理スキャンラインを食い込む」状況を
;   わざと作り、assert_line_budget が over=true でその行を捕まえることを数値で裏取りする。
; 仕掛け: smoke と同じ正常フレーム構成だが、可視領域のちょうど中央付近に「重いライン」を 1 本だけ仕込む。
;   重いライン = WSYNC の前に ~100 CPU サイクルのビジーループを回す → work > 76cy → その論理ラインが
;   2 物理スキャンラインを消費する（WSYNC が次のラインに食い込む）＝ロール要因。
; include は使わず自己完結。

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09

        org $F000

Reset:
        sei
        cld
        ldx #$FF
        txs
        lda #0
ClearMem:
        sta $00,x
        dex
        bne ClearMem

MainLoop:
; --- VSYNC: 3 lines ---
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC

; --- VBLANK: 37 lines ---
        ldx #37
VBlankLoop:
        sta WSYNC
        dex
        bne VBlankLoop

        lda #0
        sta VBLANK

; --- Visible: 上半分 96 ライン（正常）---
        lda #$1E
        sta COLUBK
        ldx #96
TopLoop:
        sta WSYNC
        dex
        bne TopLoop

; --- 重いライン 1 本: WSYNC の前に ~100cy 浪費（予算 76 を超過）---
        ldy #20
Burn:
        dey             ; 2cy
        bne Burn        ; 3cy（成立時）→ 約 20*5 = 100cy > 76 ＝この論理ラインは 2 物理ラインを消費
        sta WSYNC

; --- Visible: 下半分 95 ライン（正常）---
        ldx #95
BotLoop:
        sta WSYNC
        dex
        bne BotLoop

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK
        ldx #30
OverscanLoop:
        sta WSYNC
        dex
        bne OverscanLoop

        jmp MainLoop

; --- vectors ---
        org $FFFC
        .word Reset
        .word Reset
