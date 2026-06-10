; litmus_refp — REFP0（プレイヤー左右反転）の実機裏取り（スプライト軸の締め）
; 非対称ランプ（row0=0x80=最左1px … row7=0xFF=8px）を REFP0=$08（reflect）で描く。
; 反転すると各行の点灯が「右詰め」になり、ランプは右から左へ widening（非反転の鏡像）。
; これは pkg/sprite.Reflect（素データ側の反転）と REFP（ハード側の反転）が等価であることの裏付け。
; 実機裏取り済（Gopher2600）: read_tia_registers player0.reflected=true。read_row(96)=row0(0x80) が clock10（右端1px）、
; (100)=row4(0xF8) が clock6-10（右詰め5px）＝非反転の鏡像。回帰固定 = roms/litmus/scenarios/refp.json。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUBK  = $09
RESP0   = $10
GRP0    = $1B
REFP0   = $0B
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
        sta COLUP0
        lda #0
        sta NUSIZ0
        sta COLUBK
        lda #$08
        sta REFP0       ; プレイヤー0 を左右反転（D3）
NextFrame:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ldx #37
VBlank: sta WSYNC
        cpx #37
        bne NoPos
        sta RESP0
NoPos:  dex
        bne VBlank
        lda #0
        sta VBLANK
        ldx #192
Visible:
        sta WSYNC
        lda GfxLine-1,x
        sta GRP0
        dex
        bne Visible
        lda #0
        sta GRP0
        lda #2
        sta VBLANK
        ldx #30
OScan:  sta WSYNC
        dex
        bne OScan
        jmp NextFrame
; ランプ（litmus_sprite と同じ）: idx96..103 = row7..row0
GfxLine:
        ds 96, 0
        .byte $FF,$FE,$FC,$F8,$F0,$E0,$C0,$80
        ds 88, 0
        org $FFFC
        .word Start
        .word Start
