; litmus_pf_async — 非対称 playfield の書換窓を実機裏取り（V2-3, woodgrain Playfield_Timing）
; CTRLPF=repeat。PF1 のみ使用（PF0/PF2=0）。
; バンドA(32行): PF1=$AA を早書き(完了cyc5, LPF1窓≤27) → PF1=$55 を cyc40 書き(RPF1窓37–53)
;   期待: 左PF1=$AA（clock16-19,24-27,32-35,40-43 点灯）／右PF1=$55（100-103,108-111,116-119,124-127）＝非対称成立。
; バンドB(32行): PF1=$FF 早書き → PF1=$00 を「完了cyc33」で遅書き（LPF1表示中）
;   期待(woodgrainのworked example): 左PF1の前5ビット=旧$FF(点灯, clock16-35)・後3ビット=新$00(消灯)＝ピクセル単位の分割。
;   右PF1は $00（窓37前の書込が右半分に立つ）。
; 実機裏取り済（Gopher2600, v0.41.0）:
;  A: read_row(16) 左=$AA→16-19/24-27/32-35/40-43 点灯、右=$55→100-103/108-111/116-119/124-127 点灯（予測と完全一致）。
;  B: read_row(48) clock16-35 の 20px（5ビット）のみ点灯＝woodgrain の worked example（前5ビット旧/後3ビット新）を再現。
;  → woodgrain Playfield_Timing の窓テーブルは Gopher2600 上で正確。回帰固定=scenarios/pf_async.json。
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
Start:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #$0E
        sta COLUPF          ; PF 白
        lda #0
        sta COLUBK
        sta CTRLPF          ; repeat モード
        sta PF0
        sta PF2

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

        ; --- バンドA (32行): 非対称 左=$AA / 右=$55 ---
        ldy #32
BA:     sta WSYNC
        lda #$AA            ; 0-1
        sta PF1             ; 完了 cyc5（LPF1 窓内）
        lda #$55            ; 7
        ldx #5              ; 9
Ad:     dex
        bne Ad              ; 9+24=33
        nop                 ; 35
        nop                 ; 37
        sta PF1             ; 完了 cyc40（RPF1 窓 37-53 内）
        dey
        bne BA

        ; --- バンドB (32行): cyc33 完了の遅書きでピクセル分割 ---
        ldy #32
BB:     sta WSYNC
        lda #$FF            ; 0-1
        sta PF1             ; 完了 cyc5
        lda #$00            ; 7
        ldx #4              ; 9
Bd:     dex
        bne Bd              ; 9+19=28
        nop                 ; 30
        sta PF1             ; 完了 cyc33（LPF1 表示中＝分割発生点）
        dey
        bne BB

        ; --- 残り 128 行: PF 消灯 ---
        lda #0
        sta PF1
        ldy #128
Fill:   sta WSYNC
        dey
        bne Fill

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
