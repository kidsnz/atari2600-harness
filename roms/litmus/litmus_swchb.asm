; litmus_swchb — 本体スイッチ SWCHB の author 側読取の実機裏取り（U-M4）
; SWCHB ($0282): D0=RESET(0=押下) D1=SELECT(0=押下) D3=COLOR(1=カラー/0=白黒)
;                D6=P0難易度(1=A/Pro) D7=P1難易度(1=A/Pro)
; 毎フレーム overscan で SWCHB を $80 へ生コピー＋各ビットを $81-$85 へ展開（0/1）。
; 検証はシナリオ側: SetPanel(reset/select/color/p0pro/p1pro) を注入し各ビットを assert。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
SWCHB   = $0282

raw     = $80
bReset  = $81       ; 1=押されている（D0 反転）
bSelect = $82       ; 1=押されている（D1 反転）
bColor  = $83       ; 1=カラー（D3）
bP0pro  = $84       ; 1=A/Pro（D6）
bP1pro  = $85       ; 1=A/Pro（D7）

        org $F000
Start:  sei
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
        ldx #192
Vis:    sta WSYNC
        dex
        bne Vis
        lda #2
        sta VBLANK
        ; --- SWCHB 読取・展開 ---
        lda SWCHB
        sta raw
        and #$01
        eor #$01            ; 0=押下 → 1=押下 に反転
        sta bReset
        lda raw
        and #$02
        beq SelOn
        lda #0
        sta bSelect
        beq SelDone
SelOn:  lda #1
        sta bSelect
SelDone:
        lda raw
        and #$08
        beq ColBW
        lda #1
        sta bColor
        bne ColDone
ColBW:  lda #0
        sta bColor
ColDone:
        lda raw
        and #$40
        beq P0b
        lda #1
        sta bP0pro
        bne P0done
P0b:    lda #0
        sta bP0pro
P0done:
        lda raw
        and #$80
        beq P1b
        lda #1
        sta bP1pro
        bne P1done
P1b:    lda #0
        sta bP1pro
P1done:
        ldx #28
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

        org $FFFC
        .word Start
        .word Start
