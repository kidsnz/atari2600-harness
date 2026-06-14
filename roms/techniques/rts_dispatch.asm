; rts_dispatch — RTS-stack modular kernel dispatch のクリーンルーム・デモ（technique ⑱）
; （reference/atariage/313777-modular-kernel = vitoco の modular kernel を学んだ自前実装）
;
; トリック: 画面を「縦ゾーン」に分割し、各ゾーンの描画ルーチンの (アドレス-1) を
;   スタックに push しておき、各ゾーン末尾の RTS で次ゾーンへ飛ぶ。
;   case 分岐の山（多サイクル）の代わりに、ゾーン切替が常に 6 サイクル（RTS）。
;   → 画面構成が「ゾーン ID の RAM リスト」でデータ駆動になる（zone_multiplex の動的版）。
;
; 6502 の JSR は (戻り先-1) を push し、RTS は pull 後に +1 して PC にする。
;   よって「飛びたい先-1」を自前で push → RTS で「飛びたい先」に着地できる。
;
; このデモのゾーン種別:
;   ID 0 = ZoneSolid : 単色バンド（COLUBK 一色）
;   ID 1 = ZonePF    : プレイフィールド模様バンド（縞の PF）
;   ID 2 = ZoneSprite: スプライト・バンド（P0 のダイヤ柄が並ぶ）
;   ID 3 = ZoneStripe: 走査線ごとに色が変わるバンド（縞のグラデ）
; 各ゾーンは固定 ZONE_H 走査線 → リスト長 NLIST 固定 → 可変ゾーンでもフレーム総数は一定。

        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
COLUP0  = $06
COLUPF  = $08
COLUBK  = $09
CTRLPF  = $0A
PF0     = $0D
PF1     = $0E
PF2     = $0F
RESP0   = $10
GRP0    = $1B
HMP0    = $20
HMOVE   = $2A
HMCLR   = $2B

NLIST   = 4         ; 表示するゾーン数（固定 = フレーム総走査線を一定に保つ）
ZONE_H  = 40        ; 1 ゾーンの高さ（走査線）。NLIST*ZONE_H = 160 visible

; --- RAM ---
zonelist = $90      ; ゾーン ID の並び（NLIST バイト）= 画面構成データ
evid     = $94      ; 各スロットで「実際に走ったゾーン ID」を記録（dispatch の証跡, NLIST バイト）
slot     = $98      ; 現在処理中のスロット番号（0..NLIST）
frcnt    = $99      ; フレームカウンタ（リスト書換えデモ用）

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
        sta COLUP0          ; スプライト = 白
        lda #$1C
        sta COLUPF          ; PF = 黄
        lda #0
        sta CTRLPF

        ; 初期ゾーンリストを ROM から RAM へ（ここを書換えると画面が変わる＝データ駆動）
        ldx #NLIST-1
InitList:
        lda DefaultList,x
        sta zonelist,x
        dex
        bpl InitList

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

        inc frcnt
        ; フレーム 60 でゾーンリストを別パターンへ差し替え（RAM 書換えが画面に反映される実証）。
        ; リスト書換えは「専用の WSYNC 区切り行」内で行うので、swap 有無で総走査線は変わらない。
        sta WSYNC                  ; 論理用の 1 行を確保（毎フレーム必ず消費）
        lda frcnt
        cmp #60
        bne NoSwap
        ldx #NLIST-1
Swap:   lda AltList,x
        sta zonelist,x
        dex
        bpl Swap
NoSwap:

        ; --- VBLANK 残りを WSYNC で埋める（VSYNC3 + 論理1 + 33 = 37 行）---
        ldx #33
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; ============================================================
        ; RTS ディスパッチのセットアップ
        ;   スタックに「EndKernel-1」を最初に push（最後の RTS の着地先）、
        ;   続いて zonelist を逆順に走査し、各 ID のゾーンアドレス-1 を push。
        ;   こうすると RTS 連鎖で list[0] → list[1] → ... → EndKernel と流れる。
        ; ============================================================
        lda #>(EndKernel-1)
        pha
        lda #<(EndKernel-1)
        pha

        ldx #NLIST-1
PushLoop:
        lda zonelist,x      ; ゾーン ID
        asl                 ; *2 = ZoneTbl のワード索引
        tay
        lda ZoneTbl+1,y     ; アドレス-1 の上位
        pha
        lda ZoneTbl,y       ; アドレス-1 の下位
        pha
        dex
        bpl PushLoop

        lda #0
        sta slot            ; 証跡スロットを先頭へ

        ; 描画開始。最初の RTS で list[0] のゾーンへジャンプ。
        rts

; ------------------------------------------------------------
; 各ゾーン・ルーチン。末尾は必ず RTS（= 次ゾーン or EndKernel へ 6cy で飛ぶ）。
; 各ゾーンは ZONE_H 走査線ぶん描く。先頭で自分の ID を evid[slot] に記録（dispatch 証跡）。
; ------------------------------------------------------------

ZoneSolid:                  ; ID 0 — 単色バンド（青）
        ldy slot
        lda #0
        sta evid,y
        inc slot
        lda #$84            ; 青
        ldx #ZONE_H
SolidLp:
        sta WSYNC
        sta COLUBK
        dex
        bne SolidLp
        lda #0
        sta COLUBK
        rts

ZonePF:                     ; ID 1 — プレイフィールド模様バンド
        ldy slot
        lda #1
        sta evid,y
        inc slot
        lda #%01010101
        sta PF0
        sta PF1
        sta PF2
        lda #$46            ; 背景 = 暗い赤
        ldx #ZONE_H
PFLp:
        sta WSYNC
        sta COLUBK
        dex
        bne PFLp
        lda #0
        sta COLUBK
        sta PF0
        sta PF1
        sta PF2
        rts

ZoneSprite:                 ; ID 2 — スプライト・バンド（P0 のダイヤを 3 コピー）
        ldy slot
        lda #2
        sta evid,y
        inc slot
        sta WSYNC
        lda #$06            ; NUSIZ0 = 3 コピー（中間隔）
        sta NUSIZ0
        ; だいたい中央へ位置決め（divide-by-15 は省略、固定でよい）
        sta WSYNC
        nop                 ; ~30cy 待って中央寄りで RESP0（3 コピーが横に並ぶ）
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        nop
        sta RESP0
        lda #$2A            ; 背景 = 茶
        sta COLUBK
        ldx #ZONE_H-2
        ldy #0
SprLp:
        sta WSYNC
        lda DiaSprite,y
        sta GRP0
        iny
        cpy #8
        bne SprNoWrap
        ldy #0
SprNoWrap:
        dex
        bne SprLp
        lda #0
        sta GRP0
        sta COLUBK
        sta NUSIZ0
        rts

ZoneStripe:                 ; ID 3 — 走査線ごとに色が変わるバンド（縞グラデ）
        ldy slot
        lda #3
        sta evid,y
        inc slot
        ldx #ZONE_H
        lda #$30            ; 開始色
StripeLp:
        sta WSYNC
        sta COLUBK
        clc
        adc #2              ; 走査線ごとに色相を回す
        dex
        bne StripeLp
        lda #0
        sta COLUBK
        rts

; ------------------------------------------------------------
EndKernel:                  ; 全ゾーン終了 → overscan → 次フレーム
        lda #2
        sta VBLANK
        ; ここまで visible = NLIST*ZONE_H = 160 行。残り visible 32 行 + overscan 30 行を埋める。
        ldx #63
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

; ------------------------------------------------------------
; ディスパッチ表: ゾーン ID → (ルーチンアドレス - 1)。RTS の +1 を見越して -1。
; ------------------------------------------------------------
ZoneTbl:
        .word ZoneSolid-1   ; ID 0
        .word ZonePF-1      ; ID 1
        .word ZoneSprite-1  ; ID 2
        .word ZoneStripe-1  ; ID 3

DefaultList:
        byte 0, 1, 2, 3     ; solid, PF, sprite, stripe（上→下）
AltList:
        byte 3, 2, 1, 0     ; フレーム 60 以降は逆順（書換え反映の実証）

DiaSprite:
        byte $18,$3C,$7E,$FF,$FF,$7E,$3C,$18

        org $FFFC
        .word Start
        .word Start
