; litmus_vdel — VDEL（vertical delay）の「書込トリガ影コピー」を実機裏取り（V2-1）
; 仕様（Stella PG §6.D）: 各 GRP は new/old の2重レジスタ。GRP0 書込→P1 の new→old コピー、
; GRP1 書込→P0 の new→old コピー＋ENABL の new→old コピー。VDELxx D0=1 で old 側を表示。
; 6バンドで全3経路を1フレームに固定:
;   B1: GRP0=$FF 済みでも P0 非表示（old=0 表示）  B2: GRP1 書込→P0 出現（old←$FF）
;   B3: ENABL=on でも ball 非表示（old=off）        B4: GRP1 書込→ball 出現
;   B5: GRP1=$3C でも P1 非表示（old=0）            B6: GRP0 書込→P1 出現（old←$3C）
; 毎フレーム VBLANK 先頭で影クリア列（GRP0,GRP1,GRP0,ENABL,GRP1 を 0 書込）→決定的。
; 実機裏取り済（Gopher2600, v0.39.0）: B1=P0非表示 / B2=GRP1書込でP0出現(read_row(24)=X3,$FF,8px) /
; B3=ball非表示 / B4=GRP1書込でball出現(X=2,1px) / B5=P1非表示 / B6=GRP0書込でP1出現(read_row(88)=$3C→clock41,len4)。
; 回帰固定=scenarios/vdel.json。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
COLUP1  = $07
COLUPF  = $08
COLUBK  = $09
CTRLPF  = $0A
RESP0   = $10
RESP1   = $11
RESBL   = $14
GRP0    = $1B
GRP1    = $1C
ENABL   = $1F
VDELP0  = $25
VDELP1  = $26
VDELBL  = $27

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
        sta COLUP0          ; P0 白
        lda #$48
        sta COLUP1          ; P1 赤系
        lda #$1E
        sta COLUPF          ; ball 黄系（COLUPF）
        ; 位置決め（1回だけ）: P0=HBLANK ストローブ（X=3）, BL も HBLANK（X 実測）, P1 は遅延後
        sta WSYNC
        sta RESP0
        sta RESBL
        sta WSYNC
        ldx #6
P1d:    dex
        bne P1d
        sta RESP1
        ; VDEL 全部 ON
        lda #1
        sta VDELP0
        sta VDELP1
        sta VDELBL

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
        ; --- 影クリア列（VBLANK 1行目の HBLANK 内）---
        lda #0
        sta GRP0            ; P1.old←P1.new
        sta GRP1            ; P0.old←P0.new / BL.old←BL.new
        sta GRP0
        sta ENABL
        sta GRP1
        ldx #36
VB:     sta WSYNC
        dex
        bne VB
        ; VBLANK 最終行: 次フレームの仕込み（P0.new=$FF）＋ VBLANK 解除
        sta WSYNC
        lda #$FF
        sta GRP0            ; P0.new=$FF（old は 0 のまま → B1 で非表示のはず）
        lda #0
        sta VBLANK

        ; --- B1: P0 非表示（VDELP0=1 が old=0 を表示）---
        ldy #16
B1:     sta WSYNC
        dey
        bne B1
        ; --- B2: GRP1 書込 → P0.old←$FF → P0 出現 ---
        sta WSYNC
        lda #0
        sta GRP1
        ldy #15
B2:     sta WSYNC
        dey
        bne B2
        ; --- B3: ENABL=on（new）でも ball 非表示（VDELBL=1 が old=off を表示）---
        sta WSYNC
        lda #$02
        sta ENABL
        ldy #15
B3:     sta WSYNC
        dey
        bne B3
        ; --- B4: GRP1 書込 → BL.old←on → ball 出現 ---
        sta WSYNC
        lda #0
        sta GRP1
        ldy #15
B4:     sta WSYNC
        dey
        bne B4
        ; --- B5: GRP1=$3C（new）でも P1 非表示（VDELP1=1 が old=0 を表示）---
        sta WSYNC
        lda #$3C
        sta GRP1
        ldy #15
B5:     sta WSYNC
        dey
        bne B5
        ; --- B6: GRP0 書込 → P1.old←$3C → P1 出現 ---
        sta WSYNC
        lda #$FF
        sta GRP0
        ldy #15
B6:     sta WSYNC
        dey
        bne B6
        ; --- 残り 96 行は全消灯（影クリア列で確実に）---
        sta WSYNC
        lda #0
        sta GRP0
        sta GRP1
        sta GRP0
        sta ENABL
        sta GRP1
        ldy #95
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
