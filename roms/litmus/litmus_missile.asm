; litmus_missile — missile0 / ball の位置を実機裏取り（検証カバレッジ深化）
; missile0 と ball を可視域の別位置にストローブ配置し有効化。read_tia で各位置、read_row で縦線として読める。
; プレイヤー位置（litmus_pos, X=3N-54）に対し、ミサイル/ボール系（X=3N-55）の位置読み取りを裏取りする。
; 実機裏取り済（Gopher2600）: read_tia missile0=38 / ball=140、read_row(100) で各 clock に 1px 白。
; 回帰固定 = roms/litmus/scenarios/missile.json。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUPF  = $08
COLUBK  = $09
RESM0   = $12
RESBL   = $14
ENAM0   = $1D
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
        lda #$0E
        sta COLUP0      ; missile0 白
        sta COLUPF      ; ball 白
        lda #0
        sta COLUBK
        lda #2
        sta ENAM0       ; missile0 有効
        sta ENABL       ; ball 有効

NextFrame:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ; VBLANK line 1: 位置決め（可視域でストローブ）
        sta WSYNC
        ldy #6
DM:     dey
        bne DM          ; ~29cy ディレイ
        sta RESM0       ; missile0 を可視域へ
        ldy #6
DB:     dey
        bne DB          ; さらに ~29cy
        sta RESBL       ; ball をさらに右へ
        ldx #36
VBlank: sta WSYNC
        dex
        bne VBlank
        lda #0
        sta VBLANK
        ldx #192
Visible:
        sta WSYNC
        dex
        bne Visible
        lda #2
        sta VBLANK
        ldx #30
OScan:  sta WSYNC
        dex
        bne OScan
        jmp NextFrame

        org $FFFC
        .word Start
        .word Start
