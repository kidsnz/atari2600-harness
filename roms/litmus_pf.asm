; litmus_pf.asm — playfield ビット順 litmus（gap 0）
; 目的: ABB / falukropp の2ソースから抽出した「列→PFレジスタ＋ビット順」変換表が、
;       我々の Gopher2600 実機で本当に成り立つかを数値で裏取りする。
;
; 抽出済み変換表（画面左→右に40列、各列=4カラークロック幅）:
;   col:  0 1 2 3 | 4 5 6 7 8 9 10 11 | 12 13 14 15 16 17 18 19
;   reg:  PF0     | PF1               | PF2
;   bit:  4 5 6 7 | 7 6 5 4 3 2 1  0  | 0  1  2  3  4  5  6  7
;
; 各レジスタの「最左列だけ」を点ける既知バイト:
;   PF0 = $10 (D4)  → col0  → clock 0..3
;   PF1 = $80 (D7)  → col4  → clock 16..19
;   PF2 = $01 (D0)  → col12 → clock 48..51
; CTRLPF=$00（repeat・非反射）なので右半 clock 80..159 に同じ3本が反復するはず。
; 期待: 注釈グリッド上で clock 0-3 / 16-19 / 48-51（＋80-83 / 96-99 / 128-131）に縦バー。

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUPF  = $08
COLUBK  = $09
CTRLPF  = $0A
PF0     = $0D
PF1     = $0E
PF2     = $0F

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

        ; ---- 色とPFを設定（全フレーム固定）----
        lda #$00
        sta COLUBK      ; 背景 黒
        lda #$0E
        sta COLUPF      ; playfield 白
        lda #$00
        sta CTRLPF      ; repeat・非反射・通常優先

        lda #$10
        sta PF0         ; col0 だけ（D4）
        lda #$80
        sta PF1         ; col4 だけ（D7）
        lda #$01
        sta PF2         ; col12 だけ（D0）

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
        sta VBLANK      ; 可視へ

; --- Visible: 192 lines（PFはそのまま描かれる）---
        ldx #192
VisibleLoop:
        sta WSYNC
        dex
        bne VisibleLoop

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK
        ldx #30
OverscanLoop:
        sta WSYNC
        dex
        bne OverscanLoop

        jmp MainLoop

        org $FFFC
        .word Reset
        .word Reset
