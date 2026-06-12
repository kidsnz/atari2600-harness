; sound_driver — ゲーム内サウンドドライバ（technique: sound-driver, U-M3）
; jingle（単独 ROM）と違い**ゲームカーネルに組み込める**形の完全形:
;   ・ch0 = 音楽リード（jingle 互換の Notes/Durs 表・ループ）
;   ・ch1 = 音楽ベース、ただし SFX 発火中は SFX が横取り（優先）→ 終了で音楽へ復帰
;   ・tick は overscan 内・TIM64T で行勘定から切り離す（実ゲームの正攻法）
; 音楽はオリジナル 144f ループ（lead AUDC=4 / bass AUDC=12、cmd/jingle で生成した表）。
; SFX = laser（sound-effects 技と同じ 2バイト/フレーム表）。fire エッジで発火。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
AUDC0   = $15
AUDC1   = $16
AUDF0   = $17
AUDF1   = $18
AUDV0   = $19
AUDV1   = $1A
INPT4   = $3C
TIM64T  = $0296
INTIM   = $0284

M0LEN   = 9         ; ch0 イベント数
M1LEN   = 4         ; ch1 イベント数
SFXLEN  = 24        ; laser 表バイト長
VOL0    = 8
VOL1    = 6
BASSC   = 12        ; ch1 音楽の AUDC

m0idx   = $80
m0dur   = $81
m1idx   = $82
m1dur   = $83
m1f     = $84       ; ch1 音楽の現在 AUDF（$FF=休符）— SFX 復帰用
sfxOn   = $85
sfxIdx  = $86
prevFi  = $87
tmp     = $88

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        lda #4
        sta AUDC0
        lda #BASSC
        sta AUDC1
        jsr Adv0            ; 先頭ノートをロード
        jsr Adv1

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
        ldx #192
Vis:    sta WSYNC
        dex
        bne Vis
        ; ---- overscan: タイマー管理＋ドライバ tick ----
        lda #2
        sta VBLANK
        lda #37
        sta TIM64T
        ; --- 入力（fire エッジ → laser 発火） ---
        lda INPT4
        bmi NoPress
        lda prevFi
        bne Held
        lda #1
        sta sfxOn
        lda #0
        sta sfxIdx
Held:   lda #1
        sta prevFi
        bne Tick
NoPress:
        lda #0
        sta prevFi
        ; --- ch0 音楽 ---
Tick:   dec m0dur
        bne M0hold
        inc m0idx
        jsr Adv0
M0hold:
        ; --- ch1 音楽（SFX 中も時間は進める） ---
        dec m1dur
        bne M1hold
        inc m1idx
        jsr Adv1
M1hold:
        ; --- SFX（ch1 横取り） ---
        lda sfxOn
        beq OSwait
        ldy sfxIdx
        lda Sfx,y           ; AUDC<<4 | AUDV
        sta tmp
        and #$0F
        sta AUDV1
        lda tmp
        lsr
        lsr
        lsr
        lsr
        sta AUDC1
        iny
        lda Sfx,y
        sta AUDF1
        iny
        sty sfxIdx
        cpy #SFXLEN
        bcc OSwait
        lda #0              ; SFX 終了 → 音楽へ復帰
        sta sfxOn
        jsr WriteM1
OSwait: lda INTIM
        bne OSwait
        jmp NextFrame

; Adv0: ch0 の次イベントをロードしてレジスタへ
Adv0:   ldx m0idx
        cpx #M0LEN
        bcc Adv0b
        ldx #0
        stx m0idx
Adv0b:  lda Durs0,x
        sta m0dur
        lda Notes0,x
        cmp #$FF
        beq Rest0
        sta AUDF0
        lda #VOL0
        sta AUDV0
        rts
Rest0:  lda #0
        sta AUDV0
        rts

; Adv1: ch1 の次イベント → m1f に保持し、SFX 非発火なら書く
Adv1:   ldx m1idx
        cpx #M1LEN
        bcc Adv1b
        ldx #0
        stx m1idx
Adv1b:  lda Durs1,x
        sta m1dur
        lda Notes1,x
        sta m1f
        lda sfxOn
        bne Adv1e           ; SFX 中はレジスタに触らない
        jsr WriteM1
Adv1e:  rts

; WriteM1: m1f を ch1 レジスタへ（AUDC/音量の復帰込み）
WriteM1:
        lda #BASSC
        sta AUDC1
        lda m1f
        cmp #$FF
        beq Rest1
        sta AUDF1
        lda #VOL1
        sta AUDV1
        rts
Rest1:  lda #0
        sta AUDV1
        rts

; --- 音楽表（cmd/jingle 生成: lead "C5 E5 G5 C6 A5 G5 E5 G5 R" / bass "C4 F4 G4 C4"） ---
Notes0: byte $1D,$17,$13,$0E,$11,$13,$17,$13,$FF
Durs0:  byte $10,$10,$10,$10,$10,$10,$10,$18,$08
Notes1: byte $13,$0E,$0C,$13
Durs1:  byte $20,$20,$20,$30

; --- SFX: laser（PitchSweep(4, 4, 22, 12, 12)・sound-effects 技と同一） ---
Sfx:    byte $4C,$04
        byte $4C,$05
        byte $4C,$07
        byte $4C,$08
        byte $4C,$0A
        byte $4C,$0C
        byte $4C,$0D
        byte $4C,$0F
        byte $4C,$11
        byte $4C,$12
        byte $4C,$14
        byte $4C,$16

        org $FFFC
        .word Start
        .word Start
