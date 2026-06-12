; dyn_multisprite — 動的マルチスプライト kernel 完全形（technique #10 の一般形, overnight M-G）
; 5 オブジェクト（X 固定・Y が交差バウンド）を毎フレーム:
;   ① ソーティングネットワーク（5要素9比較・1比較/行＝サイクル安全）で Y 昇順
;   ② 上から P0/P1 交互に動的割当（1オブジェクト/行で5行）。同スロットは「前の終端+2ペア」以降のみ。
;      埋まらない場合は他スロットへフォールバック、それも不可ならドロップ（先頭スロットは
;      frameCt&1 で交替＝公平回転）
;   ③ kernel（2LK）: スロット状態機械 WAIT→POSITION（トリガペアで RESP、HM は late-set →
;      次行頭の毎行 HMOVE で適用）→DRAW→次キュー。色は WAIT 行で先行ステージ。
; X は固定 5 値 → 粗遅延/HM/色は組立時定数表。表は実測で校正（コメントに記録）。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
COLUP1  = $07
RESP0   = $10
RESP1   = $11
GRP0    = $1B
GRP1    = $1C
HMP0    = $20
HMP1    = $21
HMOVE   = $2A
TIM64T  = $0296
INTIM   = $0284

NOBJ    = 5
H       = 8         ; ペア単位（=16走査線）

ys      = $80       ; ×5（ペア 2-86）
dirs    = $85       ; ×5
sortIdx = $8A       ; ×5
frameCt = $8F
q0y     = $90       ; P0 キュー: トリガペア ×3＋番兵
q0o     = $94       ; obj ×3
q0n     = $97
q1y     = $98       ; P1 キュー ×2＋番兵
q1o     = $9B
q1n     = $9D
sent    = $9E
p0st    = $A0
p0row   = $A1
p0qi    = $A2
p1st    = $A3
p1row   = $A4
p1qi    = $A5
pair    = $A6
tmp     = $A7
tIA     = $A8
tIB     = $A9
minS0   = $AA
minS1   = $AB
firstS  = $AC
aIdx    = $AD       ; Assign 用ループカウンタ

; 比較器マクロ: 位置 {1},{2} の sortIdx を ys 昇順に（スワップは固定8バイト→ bcs *+10）
        MAC CMPSW
        lda sortIdx+{1}
        sta tIA
        lda sortIdx+{2}
        sta tIB
        ldx tIA
        ldy tIB
        lda ys,y
        cmp ys,x
        bcs *+10
        lda tIB
        sta sortIdx+{1}
        lda tIA
        sta sortIdx+{2}
        ENDM

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
        ldx #NOBJ-1
Ini:    lda YInit,x
        sta ys,x
        lda DInit,x
        sta dirs,x
        dex
        bpl Ini
        lda #$D6
        sta sent

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
        lda #44
        sta TIM64T          ; ★VBLANK はタイマー管理（実ゲームの正攻法）: sort/assign の
                            ;  経路差（60〜160cy）を行勘定から切り離す。43×64≈2752cy ≈ 36行＋α
        inc frameCt
        ; --- バウンド ×5 ---
        ldx #4
BLoop:  jsr Bounce
        dex
        bpl BLoop
        ; --- sortIdx 初期化 ---
        ldx #4
SIni:   txa
        sta sortIdx,x
        dex
        bpl SIni
        ; --- ソーティングネットワーク（9比較・インライン）---
        CMPSW 0,1
        CMPSW 3,4
        CMPSW 2,4
        CMPSW 2,3
        CMPSW 1,4
        CMPSW 0,3
        CMPSW 0,2
        CMPSW 1,3
        CMPSW 1,2
        ; --- 割当 ×5 ---
        lda #0
        sta q0n
        sta q1n
        sta minS0
        sta minS1
        sta aIdx
        lda frameCt
        and #1
        sta firstS
AsnL:   jsr AssignOne
        lda aIdx
        cmp #NOBJ
        bcc AsnL
        ldx q0n             ; 番兵（トリガ値 0 は決して一致しない）
        lda #0
        sta q0y,x
        ldx q1n
        sta q1y,x
        ; --- kernel 状態初期化 ---
        lda #160            ; pair は 160..255 で 96 ペアを数える（尾部を inc+beq に短縮）
        sta pair
        lda #0
        sta p0row
        sta p1row
        sta p0qi
        sta p1qi
        sta GRP0
        sta GRP1
        sta HMP0
        sta HMP1
        ldx #0
        lda q0n
        beq Z0
        ldx #1
Z0:     stx p0st
        ldx #0
        lda q1n
        beq Z1
        ldx #1
Z1:     stx p1st
        ; --- タイマー待ち → 可視へ（VBLANK 合計はタイマーが保証）---
WaitVB: lda INTIM
        bne WaitVB
        sta WSYNC
        lda #0
        sta VBLANK

        ; ============ 可視 96 ペア × 2 行 ============
KPair:  sta WSYNC           ; ---- A 行（P0）----
        lda p0st            ; 3
        beq KA_id           ; 8
        cmp #2              ; 10
        beq KA_dr           ; 12
        ldx p0qi            ; WAIT: トリガ判定（枯渇は番兵 0 が吸収＝pair と一致しない）
        lda q0y,x
        cmp pair
        beq KA_pos
        ; WAIT（非トリガ）: 次オブジェクトの色を先行ステージ＋HM クリア
        lda q0o,x           ; 28
        tax                 ; 30
        lda ColTbl,x        ; 34
        sta COLUP0          ; 37
        lda #0
        sta GRP0
        jmp KA_end
KA_dr:  ldx p0row           ; 15
        lda Art,x           ; 19
        sta GRP0            ; 22
        inc p0row           ; 27
        lda p0row           ; 30
        cmp #H              ; 32
        bne KA_end
        lda #0              ; 完了: 消灯し WAIT へ（枯渇判定は WAIT 側ガード）
        sta GRP0
        sta p0row
        inc p0qi
        lda #1
        sta p0st
        bne KA_end2
KA_id:  lda #0
        sta GRP0
        jmp KA_end
KA_pos: lda q0o,x           ; POSITION（色は WAIT でステージ済み・fallthrough で -3cy）
        tax
        ldy DelTbl,x
KA_dl:  dey
        bne KA_dl
        sta RESP0           ; 粗のみ＝X は 66+15d の正確な格子（HM 不要）
        lda #2
        sta p0st
KA_end:
KA_end2:
        sta WSYNC           ; ---- B 行（P1・対称）----
        lda p1st
        beq KB_id
        cmp #2
        beq KB_dr
        ldx p1qi
        lda q1y,x
        cmp pair
        beq KB_pos
        lda q1o,x
        tax
        lda ColTbl,x
        sta COLUP1
        lda #0
        sta GRP1
        jmp KB_end
KB_dr:  ldx p1row
        lda Art,x
        sta GRP1
        inc p1row
        lda p1row
        cmp #H
        bne KB_end
        lda #0
        sta GRP1
        sta p1row
        inc p1qi
        lda #1
        sta p1st
        bne KB_end2
KB_id:  lda #0
        sta GRP1
        jmp KB_end
KB_pos: lda q1o,x
        tax
        ldy DelTbl,x
KB_dl:  dey
        bne KB_dl
        sta RESP1
        lda #2
        sta p1st
KB_end:
KB_end2:
        inc pair
        beq KDone           ; 160+96=256 → 0
        jmp KPair
KDone:
        lda #2
        sta VBLANK
        lda #0
        sta GRP0
        sta GRP1
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

; --- Bounce: obj X の Y ±1（範囲表）---
Bounce: lda dirs,x
        bne BUp
        inc ys,x
        lda ys,x
        cmp RangeHi,x
        bcc BDone
        lda #1
        sta dirs,x
        rts
BUp:    dec ys,x
        lda ys,x
        cmp RangeLo,x
        bcs BDone
        lda #0
        sta dirs,x
BDone:  rts

; --- AssignOne: ソート位置 aIdx のオブジェクトをスロットへ ---
; slotPref = (firstS + aIdx) & 1。可能なら pref、不可なら他方、両方不可ならドロップ。
AssignOne:
        ldx aIdx
        lda sortIdx,x
        tax                 ; X = obj
        lda ys,x
        sta tIA             ; y
        lda aIdx
        clc
        adc firstS
        and #1
        beq TryP0first
        jsr TryP1
        bcs AsnDone         ; 置けた
        jsr TryP0
        jmp AsnDone
TryP0first:
        jsr TryP0
        bcs AsnDone
        jsr TryP1
AsnDone:
        inc aIdx
        rts
; TryP0/1: tIA=y, X=obj。置けたら carry set。
TryP0:  lda q0n
        cmp #3
        bcs TFail
        lda tIA
        cmp minS0
        bcc TFail
        ldy q0n
        clc
        adc #159            ; トリガ = (y-1)+160（pair カウンタ系）
        sta q0y,y
        txa
        sta q0o,y
        iny
        sty q0n
        lda tIA
        clc
        adc #H+2
        sta minS0
        sec
        rts
TryP1:  lda q1n
        cmp #2
        bcs TFail
        lda tIA
        cmp minS1
        bcc TFail
        ldy q1n
        clc
        adc #159
        sta q1y,y
        txa
        sta q1o,y
        iny
        sty q1n
        lda tIA
        clc
        adc #H+2
        sta minS1
        sec
        rts
TFail:  clc
        rts

YInit:   byte 5, 20, 40, 60, 75
DInit:   byte 0, 1, 0, 1, 0
RangeLo: byte 2, 6, 10, 4, 8
RangeHi: byte 80, 70, 86, 76, 84
NetA:    byte 0,3,2,2,1,0,0,1,1
NetB:    byte 1,4,4,3,4,3,2,3,2
; X は粗格子 66+15d（HM 不要の正確な整数位置）: d=1,2,3,4,2 → X=81,96,111,126,96
ColTbl:  byte $1E,$56,$9A,$CA,$44
DelTbl:  byte 1,2,3,4,2
Art:     byte %00111100
         byte %01111110
         byte %11111111
         byte %11011011
         byte %11111111
         byte %11100111
         byte %01111110
         byte %00111100

        org $FFFC
        .word Start
        .word Start
