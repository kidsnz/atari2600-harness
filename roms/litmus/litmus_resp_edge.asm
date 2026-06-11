; litmus_resp_edge — RESBL/RESPx の二度ストローブ挙動を実機裏取り（V2-11, Towers TIA_HW_Notes）
; 主張: RESBL は START を再発行する＝1ライン中に二度ストローブで【ボール2個】。
;       RESPx は次のラップ(160clk)まで再発行しない＝二度ストローブでも【プレイヤー1個】（最後の位置）。
; バンドA(16行): RESBL を1ライン中に2回ストローブ → read_row のボール run 数を測る。
; バンドB(16行): RESP0 を1ライン中に2回ストローブ → read_row のプレイヤー run 数を測る。
; 実機裏取り済（Gopher2600, v0.50.0）: バンドA(RESBL×2/ライン)=read_row(8) に【ボール2個】(clock 38, 140)＝
; RESBL は START 再発行（マルチボール）。バンドB(RESP0×2/ライン)=read_row(24) に【プレイヤー1個】(clock107, 8px)＝
; RESPx は次ラップまで再発行せず最後の位置のみ。✓Towers TIA_HW_Notes。回帰固定=scenarios/resp_edge.json（golden）。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
COLUPF  = $08
COLUBK  = $09
RESP0   = $10
RESBL   = $14
GRP0    = $1B
ENABL   = $1F

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
        sta $2C
        lda #$0E
        sta COLUP0
        sta COLUPF
        lda #0
        sta COLUBK

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

        ; --- バンドA: RESBL を2回/ライン ---
        lda #$02
        sta ENABL           ; ball on（幅1）
        ldy #16
BA:     sta WSYNC
        ldx #6
A1:     dex
        bne A1              ; 遅延1（〜clock 34）
        sta RESBL           ; 1回目
        ldx #6
A2:     dex
        bne A2              ; 遅延2（+〜31cy → 〜clock 127）
        sta RESBL           ; 2回目
        dey
        bne BA
        lda #0
        sta ENABL

        ; --- バンドB: RESP0 を2回/ライン ---
        lda #$FF
        sta GRP0            ; P0 = 8px
        ldy #16
BB:     sta WSYNC
        ldx #6
B1:     dex
        bne B1
        sta RESP0           ; 1回目
        ldx #6
B2:     dex
        bne B2
        sta RESP0           ; 2回目
        dey
        bne BB
        lda #0
        sta GRP0

        ; --- 残り 160 行消灯 ---
        ldy #158
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
