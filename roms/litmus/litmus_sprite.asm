; litmus_sprite — player0 GRP litmus（pkg/sprite のビット順を実機裏取り）
; 8x8 の非対称ランプ（上=0x80 の1px → 下=0xFF の8px）を player0 で描く。
; ランプは左右非対称なので、ビット順の取り違え（D7↔D0 や行の上下反転）が一目で出る。
; ASCII 設計（pkg/sprite.Encode で各行 1 バイト・D7=最左）:
;   X.......=$80  XX......=$C0  XXX.....=$E0  XXXX....=$F0
;   XXXXX...=$F8  XXXXXX..=$FC  XXXXXXX.=$FE  XXXXXXXX=$FF
; 実機裏取り済（Gopher2600 read_row, P0 を X=3 に置いた状態）:
;   可視ライン 96..103 にランプが出る（カーネルの SpriteTop=88 は Gopher2600 可視0起点で +8 ずれる）。
;   read_row(96)=clock3 に白 1px（=$80）→ read_row(103)=clock3..10 に白 8px（=$FF）。左→右に widening＝
;   ビット順 D7=最左・行順 上→下 が正しいことの数値証明。golden シナリオ roms/litmus/scenarios/sprite.json で回帰固定。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUBK  = $09
RESP0   = $10
GRP0    = $1B

SpriteTop = 88          ; スプライト最上段の可視 scanline（8 行: 88..95）

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
        lda #$0E          ; player0 = 白
        sta COLUP0
        lda #0
        sta NUSIZ0        ; 1 コピー・通常幅
        sta COLUBK        ; 背景 = 黒（スプライトを読みやすく）

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
        sta RESP0         ; 最初の VBLANK ライン先頭で 1 回だけ位置決め（決定的）
NoPos:  dex
        bne VBlank
        lda #0
        sta VBLANK

        ldx #192          ; 可視 192 ライン
Visible:
        sta WSYNC
        lda GfxLine-1,x   ; x:192..1 → GfxLine[191..0]。GfxLine[191]=最上段
        sta GRP0          ; HBLANK 中に GRP0 を確定（P0 の X に依らず正しい行が出る）
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

; GfxLine[k] = 可視ライン (191-k) の GRP バイト。スプライトは可視ライン 88..95。
;   line 88(top)→idx 103=$80 … line 95(bottom)→idx 96=$FF
GfxLine:
        ds 96, 0                                  ; idx 0..95（可視ライン 191..96）= 消灯
        .byte $FF,$FE,$FC,$F8,$F0,$E0,$C0,$80     ; idx 96..103（可視ライン 95..88）= ランプ
        ds 88, 0                                  ; idx 104..191（可視ライン 87..0）= 消灯

        org $FFFC
        .word Start
        .word Start
