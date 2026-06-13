; hscroll — プレイフィールド横スクロール（technique: hscroll, U-AR ⑪, sidescroll 参照）
; 系譜: 旧 ATARI AR/Studies の Side-Scroll/scroll.asm を読み解いた自前実装。
; 方式: 4px 縞（32px 周期）の PF パターンを **8 位相のテーブル**で持ち、scrollSpeed フレーム毎に
;   位相を 1 進める＝縞が 4px ずつ横移動。CTRLPF reflect で左右対称。
;   位相は read_row で縞位置の移動として数値検証できる（coarse 4px スクロール）。
; 発展（コメント）: 1px 精度はバススタッフィング/非対称 PF 書換が必要（技候補・別途）。
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
TIM64T  = $0296
INTIM   = $0284

phase   = $80      ; 0..7 スクロール位相
sdelay  = $81
sspeed  = $82      ; 位相を進める間隔（フレーム）

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #$01            ; reflect
        sta CTRLPF
        lda #$3E            ; 縞色
        sta COLUPF
        lda #6
        sta sspeed

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
        lda #43
        sta TIM64T
        ; --- スクロール: sspeed フレーム毎に位相 +1 ---
        inc sdelay
        lda sdelay
        cmp sspeed
        bcc NoScroll
        lda #0
        sta sdelay
        inc phase
        lda phase
        and #7
        sta phase
NoScroll:
        ; --- 位相の PF をロード ---
        ldx phase
        lda ScrPF0,x
        sta PF0
        lda ScrPF1,x
        sta PF1
        lda ScrPF2,x
        sta PF2
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK
        ; --- 可視 192 行: 同一 PF（縦縞）---
        ldx #192
Vis:    sta WSYNC
        dex
        bne Vis
        lda #2
        sta VBLANK
        ldx #29
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

ScrPF0: byte $F0,$70,$30,$10,$00,$80,$C0,$E0
ScrPF1: byte $0F,$1E,$3C,$78,$F0,$E1,$C3,$87
ScrPF2: byte $F0,$78,$3C,$1E,$0F,$87,$C3,$E1
        org $FFFC
        .word Start
        .word Start
