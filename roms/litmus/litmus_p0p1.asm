; litmus_p0p1 — P0+P1 連結で最大16px キャラ（hardening-roadmap S-3 旗艦）
; 16 幅 8 行の設計を pkg/sprite.SplitWide で P0(左8)・P1(右8)に分割し、P1 を P0 の +8px に隣接配置。
; 継ぎ目なしなら solid 行は連続 16px の白として出る（read_row 1 run）。P0/P1 同色（白）で継ぎ目連続を検証。
; 設計（X=点灯）:
;   row0 XXXXXXXXXXXXXXXX  P0=$FF P1=$FF  solid16（継ぎ目テスト）
;   row1 XXXXXXXX........  P0=$FF P1=$00  P0 のみ
;   row2 ........XXXXXXXX  P0=$00 P1=$FF  P1 のみ
;   row3 XXXXXXXXXXXXXXXX  P0=$FF P1=$FF  solid16
;   row4 X..............X  P0=$80 P1=$01  両端のみ
;   row5 XXXXXXXXXXXXXXXX  P0=$FF P1=$FF  solid16
;   row6 ........XXXXXXXX  P0=$00 P1=$FF  P1 のみ
;   row7 XXXXXXXXXXXXXXXX  P0=$FF P1=$FF  solid16
; 位置決め: RESP0→RESP1 を 3cy 間隔で撃つと P1=P0+9px、HMP1=$10(左1)＋HMOVE で P1=P0+8 に詰める。
; ※ HBLANK 中ストローブは最左クランプで位置が潰れる → ディレイで可視域へ出してから撃つ。
; HMOVE は VBLANK 中（可視に comb を出さない）。
; 実機裏取り済（Gopher2600）: read_tia で player0=69 / player1=77（＝ちょうど +8）。
;   read_row(可視96, solid16 行)=clock 69-84 が白 len16 の連続1run＝継ぎ目ゼロ。
;   (97)=左8px(P0) / (98)=右8px(P1, 77 から連続) / (100)=両端 1px ずつ。P0+P1 連結16px が継ぎ目なく成立。
;   回帰固定 = roms/litmus/scenarios/p0p1.json（位置アサート 69/77 ＋ golden）。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
NUSIZ1  = $05
COLUP0  = $06
COLUP1  = $07
COLUBK  = $09
RESP0   = $10
RESP1   = $11
GRP0    = $1B
GRP1    = $1C
HMP0    = $20
HMP1    = $21
HMOVE   = $2A
HMCLR   = $2B

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
        lda #$0E          ; P0/P1 とも白（継ぎ目連続の検証用）
        sta COLUP0
        sta COLUP1
        lda #0
        sta NUSIZ0
        sta NUSIZ1
        sta COLUBK

NextFrame:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        ; --- VBLANK 37 ラインのうち先頭 2 ラインで位置決め ---
        ; HBLANK 中ストローブは最左クランプ（位置が潰れる）→ ディレイで可視域へ出してから撃つ。
        sta WSYNC         ; VBLANK line 1: beam=clock-68
        ldy #8
DelayP: dey
        bne DelayP        ; ~39cy 消費 → beam を可視域へ
        sta RESP0         ; P0 を可視域に配置
        sta RESP1         ; 3cy 後 → P1 = P0 + 9px
        lda #0
        sta HMP0
        lda #$10
        sta HMP1          ; P1 左1 → +8
        sta WSYNC         ; VBLANK line 2
        sta HMOVE         ; WSYNC 直後に適用（comb は VBLANK 内）
        ldx #35
VBlank: sta WSYNC
        dex
        bne VBlank
        lda #0
        sta VBLANK

        ldx #192          ; 可視 192 ライン
Visible:
        sta WSYNC
        lda Gfx0Line-1,x
        sta GRP0
        lda Gfx1Line-1,x
        sta GRP1
        dex
        bne Visible
        lda #0
        sta GRP0
        sta GRP1

        lda #2
        sta VBLANK
        ldx #30
OScan:  sta WSYNC
        dex
        bne OScan
        jmp NextFrame

; GfxnLine[k] = 可視ライン (191-k) の GRP。スプライトは（カーネル基準）可視 88..95＝Gopher2600 可視 96..103。
; テーブルは下→上（idx96=row7 … idx103=row0）。
;   P0 rows: FF FF 00 FF 80 FF 00 FF（row0..7）→ reversed: FF 00 FF 80 FF 00 FF FF
;   P1 rows: FF 00 FF FF 01 FF FF FF（row0..7）→ reversed: FF FF FF 01 FF FF 00 FF
Gfx0Line:
        ds 96, 0
        .byte $FF,$00,$FF,$80,$FF,$00,$FF,$FF
        ds 88, 0
Gfx1Line:
        ds 96, 0
        .byte $FF,$FF,$FF,$01,$FF,$FF,$00,$FF
        ds 88, 0

        org $FFFC
        .word Start
        .word Start
