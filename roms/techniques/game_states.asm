; game_states — ゲーム状態機械の完全形（technique: game-states, U-M4）
; 実ゲーム標準の骨格: title → play → game-over → title。
;   ・title:    青背景。SELECT でバリアント巡回（0-3）。RESET か fire で開始。
;               放置 300f でアトラクト（背景明滅）。何か入力で解除。
;   ・play:     緑背景＋ドリフトするスプライト（毎フレーム HMOVE で右へ 1px）。
;               タイマー 320f で終了（P0 難易度 A/Pro なら 2 倍速＝160f — SWCHB D6 分岐）。
;   ・gameover: 赤背景 120f → title。RESET で即 play。
; 状態は $80（scenario で数値 assert）。VBLANK は TIM64T 管理（経路差を行勘定から切り離す）。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUP0  = $06
COLUBK  = $09
RESP0   = $10
GRP0    = $1B
HMP0    = $20
HMOVE   = $2A
INPT4   = $3C
SWCHB   = $0282
TIM64T  = $0296
INTIM   = $0284

ST_TITLE = 0
ST_PLAY  = 1
ST_OVER  = 2

state   = $80
tLo     = $81
tHi     = $82
variant = $83
prevFi  = $84       ; fire 前回値（エッジ）
prevRe  = $85       ; RESET 前回値
prevSe  = $86       ; SELECT 前回値
idleLo  = $87
idleHi  = $88
attract = $89       ; 1=アトラクト中
ln      = $8A       ; カーネル行カウンタ

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #$0E
        sta COLUP0

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
        sta WSYNC
        sta HMOVE           ; play 中の横ドリフト適用（HMP0 は状態側で設定）
        lda #43
        sta TIM64T

        ; ================= フレームロジック（状態機械） =================
        ; --- 入力スナップショット（エッジ検出の材料） ---
        ldx #0
        lda INPT4
        bmi FiOff
        ldx #1
FiOff:  stx $8C             ; 今回 fire
        ldx #0
        lda SWCHB
        and #$01
        bne ReOff           ; D0=1 → 非押下
        ldx #1
ReOff:  stx $8D             ; 今回 reset
        ldx #0
        lda SWCHB
        and #$02
        bne SeOff
        ldx #1
SeOff:  stx $8E             ; 今回 select

        lda state
        cmp #ST_PLAY
        bne Disp1
        jmp StPlay
Disp1:  cmp #ST_OVER
        bne Disp2
        jmp StOver
Disp2:

        ; ---------- title ----------
        lda #$84            ; 青
        sta COLUBK
        lda #0
        sta GRP0
        sta HMP0
        ; SELECT エッジ → バリアント
        lda $8E
        beq TiSel0
        lda prevSe
        bne TiSelDone
        inc variant
        lda variant
        and #$03
        sta variant
        jsr IdleReset
        jmp TiSelDone
TiSel0:
TiSelDone:
        ; fire/RESET エッジ → play
        lda $8C
        ora $8D
        beq TiNoStart
        lda prevFi
        ora prevRe
        bne TiHeld
        jsr EnterPlay
        jmp Logic9
TiHeld:
TiNoStart:
        ; アトラクト: 放置 300f で背景明滅
        inc idleLo
        bne TiIdle1
        inc idleHi
TiIdle1:
        lda idleHi
        cmp #1
        bcc TiAttrOff       ; <256f はまだ
        lda idleLo
        cmp #44             ; 256+44=300f
        bcc TiAttrOff
        lda #1
        sta attract
TiAttrOff:
        lda attract
        beq Logic9
        lda idleLo
        and #$10            ; 16f 周期で明滅
        beq Logic9
        lda #$86
        sta COLUBK
        jmp Logic9

        ; ---------- play ----------
StPlay: lda #$C8            ; 緑
        sta COLUBK
        lda #$F0            ; 右 1px/フレーム
        sta HMP0
        ; タイマー（P0 難易度 A/Pro なら 2 倍速） SWCHB D6
        lda SWCHB
        and #$40
        beq PlNormal
        inc tLo
        bne PlNormal
        inc tHi
PlNormal:
        inc tLo
        bne PlChk
        inc tHi
PlChk:  lda tHi
        cmp #1
        bcc Logic9
        lda tLo
        cmp #64             ; 256+64=320f（Pro は半分の実時間）
        bcc Logic9
        lda #ST_OVER
        sta state
        lda #0
        sta tLo
        sta tHi
        jmp Logic9

        ; ---------- gameover ----------
StOver: lda #$46            ; 赤
        sta COLUBK
        lda #0
        sta GRP0
        sta HMP0
        ; RESET エッジ → 即 play
        lda $8D
        beq OvTick
        lda prevRe
        bne OvTick
        jsr EnterPlay
        jmp Logic9
OvTick: inc tLo
        lda tLo
        cmp #120
        bcc Logic9
        lda #ST_TITLE
        sta state
        lda #0
        sta tLo
        sta idleLo
        sta idleHi
        sta attract

Logic9: ; --- 前回入力の更新 ---
        lda $8C
        sta prevFi
        lda $8D
        sta prevRe
        lda $8E
        sta prevSe

        ; --- VBLANK 終了待ち ---
VBwait: lda INTIM
        bne VBwait
        sta WSYNC
        lda #0
        sta VBLANK

        ; ================= 可視 192 行 =================
        lda #0
        sta ln
KLine:  sta WSYNC
        lda state
        cmp #ST_PLAY
        bne KBlank
        lda ln              ; play: 行 100-107 にスプライト
        sec
        sbc #100
        cmp #8
        bcc KDraw
KBlank: lda #0
        beq KStore
KDraw:  tax
        lda Art,x
KStore: sta GRP0
        inc ln
        lda ln
        cmp #192
        bne KLine

        lda #2
        sta VBLANK
        lda #0
        sta GRP0
        ldx #29
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

; EnterPlay: play 状態へ（スプライト位置を決定的に初期化）
EnterPlay:
        lda #ST_PLAY
        sta state
        lda #0
        sta tLo
        sta tHi
        sta attract
        jsr IdleReset
        sta WSYNC           ; X を決定的に: 固定サイクルで RESP0
        ds 13, $EA          ; SLEEP 26
        sta RESP0
        rts

IdleReset:
        lda #0
        sta idleLo
        sta idleHi
        rts

Art:    byte %00111100
        byte %01111110
        byte %11011011
        byte %11111111
        byte %10111101
        byte %11000011
        byte %01111110
        byte %00111100

        org $FFFC
        .word Start
        .word Start
