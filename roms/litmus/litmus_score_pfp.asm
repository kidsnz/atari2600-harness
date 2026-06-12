; litmus_score_pfp — CTRLPF の SCORE(D1) × PFP(D2) 相互作用の実測（U-M11）
; 文書: SCORE=PF を左右で COLUP0/COLUP1 色に / PFP=PF が選手より優先（COLUPF 色）。
; 未検証だった交差項: **両方セット（$06）のとき PF は何色か・優先はどうなるか**。
; 構成: PF1=$FF（左半分 clock16-47 / 右半分 96-127 に帯）、P0($FF, X=24)・P1($FF, X=104) を
; PF 帯に重ねる。CTRLPF を 60f 周期で $02 → $04 → $06 と切替（fc/60 で 3 状態巡回）。
; 判定: read_row（PF 部と重なり部の色）＋ golden。色: PF=$46赤 P0=$86青 P1=$C6緑 BK=$00。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
COLUP1  = $07
COLUPF  = $08
COLUBK  = $09
CTRLPF  = $0A
PF1     = $0E
RESP0   = $10
RESP1   = $11
GRP0    = $1B
GRP1    = $1C

fc      = $80
phase   = $81       ; 0=$02(SCORE) 1=$04(PFP) 2=$06(両方)

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #$46
        sta COLUPF
        lda #$86
        sta COLUP0
        lda #$C6
        sta COLUP1
        lda #$FF
        sta PF1
        sta GRP0
        sta GRP1
        ; P0=24 / P1=104（PF 帯に重ねる）
        sta WSYNC
        ds 13, $EA          ; SLEEP 26
        sta RESP0
        ds 11, $EA          ; +22cy
        sta RESP1

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
        ; --- 60f 周期で CTRLPF 切替 ---
        inc fc
        lda fc
        cmp #60
        bcc PhSet
        lda #0
        sta fc
        inc phase
        lda phase
        cmp #3
        bcc PhSet
        lda #0
        sta phase
PhSet:  ldx phase
        lda CtlTab,x
        sta CTRLPF
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
        ldx #29
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

CtlTab: byte $02,$04,$06

        org $FFFC
        .word Start
        .word Start
