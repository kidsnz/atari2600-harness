; litmus_ctrlpf — CTRLPF の SCORE(D1)/priority(D2)/ball幅(D4-5) を実機裏取り（V2-7）
; 監査の宿題: 未仕様の SCORE×PFP 相互作用を測る。
; 色: COLUP0=$44赤 / COLUP1=$84青 / COLUPF=$1E黄 / COLUBK=$00黒。
; PF 全点灯（PF0=$F0,PF1=$FF,PF2=$FF）で左右半分を塗る。P0 は左半分に8pxバー（overlap用）。
; バンド:
;  A SCORE($02): 左半=COLUP0赤 / 右半=COLUP1青（PFがプレイヤー色を取る）
;  B priority default($00): P0(赤)が PF(黄)の上 → overlap は赤
;  C PFP($04): PF が P0 の上 → overlap は黄（P0 隠れる）
;  D SCORE+PFP($06): 未仕様の相互作用を実測（左半は赤か黄か）
;  E ball幅: PF消灯, ENABL, CTRLPF D4-5=00/01/10/11 → ball幅 1/2/4/8 を read_row
; 実機裏取り済（Gopher2600, v0.45.0, read_row 観測行は A=12/B=36/C=60/D=84/E=106,114,122,128）:
;  A SCORE($02): 左半=COLUP0赤 / 右半=COLUP1青（clock80で分割）。✓仕様通り
;  B default($00): P0赤(clock3-10)が PF黄の上。 C PFP($04): 全面 PF黄＝P0隠れ。✓priority 反転
;  D SCORE+PFP($06): 全面 COLUPF黄（=SCORE色置換が抑制され COLUPF が出る）＋P0隠れ。
;     ※監査の未仕様項目。Gopher2600 の実測＝エミュ差が出やすい角。F-4 Stella オラクル(V2-17)で要照合。
;  E ball幅 D4-5=$00/$10/$20/$30 → 1/2/4/8px（倍々, read_row len=1/2/4/8）。✓
;  回帰固定=scenarios/ctrlpf.json（golden）。
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
RESBL   = $14
GRP0    = $1B
ENABL   = $1F
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
        lda #$44
        sta COLUP0
        lda #$84
        sta COLUP1
        lda #$1E
        sta COLUPF
        lda #0
        sta COLUBK
        lda #$FF
        sta GRP0            ; P0 = 8px バー
        ; P0 を左半分へ（WSYNC 後 約12cy 遅延ストローブ）
        sta WSYNC
        ldx #3
P0d:    dex
        bne P0d
        sta RESP0
        ; ball を右半分(〜clock 80)へ
        sta WSYNC
        ldx #10
Bd:     dex
        bne Bd
        sta RESBL

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

        ; PF 全点灯（バンドA-D 用）
        lda #$F0
        sta PF0
        lda #$FF
        sta PF1
        sta PF2

        ; --- バンドA: SCORE ($02) ---
        lda #$02
        sta CTRLPF
        ldy #24
BA:     sta WSYNC
        dey
        bne BA
        ; --- バンドB: priority default ($00) ---
        lda #$00
        sta CTRLPF
        ldy #24
BB:     sta WSYNC
        dey
        bne BB
        ; --- バンドC: PFP ($04) ---
        lda #$04
        sta CTRLPF
        ldy #24
BC:     sta WSYNC
        dey
        bne BC
        ; --- バンドD: SCORE+PFP ($06) ---
        lda #$06
        sta CTRLPF
        ldy #24
BD:     sta WSYNC
        dey
        bne BD

        ; --- バンドE: ball幅（PF消灯, ENABL on）---
        lda #0
        sta PF0
        sta PF1
        sta PF2
        lda #$02
        sta ENABL
        ; E1 幅1 ($00)
        lda #$00
        sta CTRLPF
        ldy #8
BE1:    sta WSYNC
        dey
        bne BE1
        ; E2 幅2 ($10)
        lda #$10
        sta CTRLPF
        ldy #8
BE2:    sta WSYNC
        dey
        bne BE2
        ; E3 幅4 ($20)
        lda #$20
        sta CTRLPF
        ldy #8
BE3:    sta WSYNC
        dey
        bne BE3
        ; E4 幅8 ($30)
        lda #$30
        sta CTRLPF
        ldy #8
BE4:    sta WSYNC
        dey
        bne BE4

        ; --- 残り消灯（96 使用 → 96 fill）---
        lda #0
        sta ENABL
        ldy #64
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
