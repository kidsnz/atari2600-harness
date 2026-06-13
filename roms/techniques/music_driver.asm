; music_driver — 楽器エンベロープ式 音楽ドライバ（technique: music-driver, 技候補⑦）
; TIATracker（forums.atariage.com/topic/250014）のデータモデルをクリーンルームで縮約。
; 既存 sound_driver との決定的な差 = **AUDV を「楽器の音量エンベロープ」で毎フレーム駆動**し、
;   **ノートごとに楽器を選べる**こと（＝音量/ゲート付き＝候補⑦の本質）。
;
; データ3層（TIATracker の instrument / pattern / song を縮約）:
;   ・Instrument = { AUDC, エンベロープ開始オフセット, サステイン index }
;       エンベロープ = 4bit 音量の列。envIdx を毎フレーム進め、sus に達したら保持（attack/decay→sustain）。
;       sus が末尾=0 の楽器は「撥弦/打（pluck）」＝減衰して無音に落ちる＝ゲートの実演。
;   ・Pattern = ノート列。各ノート = { 音高(AUDF/$FF=休符), 楽器id, 長さ(frames) }。
;   ・Song = パターンを末尾でループ（goto 相当・idx wrap）。2 チャンネル（ch0 リード / ch1 ベース）。
;   ・tick は overscan の TIM64T 内（行勘定から分離・実ゲームの正攻法。sound_driver と同様）。
;
; Env[0]=0 を「無音セル」に予約し、休符はそこを指す（フラグ不要でゲートオフを表現）。
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
TIM64T  = $0296
INTIM   = $0284

P0LEN   = 8         ; ch0 パターン長
P1LEN   = 4         ; ch1 パターン長

; --- ゼロページ状態（ch あたり 5 バイト×2 = 10・TIATracker の RAM 予算と同程度）---
c0dur   = $80       ; ch0 残フレーム
c0idx   = $81       ; ch0 パターン位置
c0eoff  = $82       ; ch0 現在エンベロープ開始オフセット
c0eidx  = $83       ; ch0 エンベロープ内位置
c0sus   = $84       ; ch0 サステイン index
c1dur   = $85
c1idx   = $86
c1eoff  = $87
c1eidx  = $88
c1sus   = $89
tmp     = $8A

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        ; 先頭ノートをロード（idx=0 のまま）
        jsr Adv0
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
        ; ---- overscan: ドライバ tick ----
        lda #2
        sta VBLANK
        lda #37
        sta TIM64T

        ; --- ch0: ノート進行 → エンベロープ step ---
        dec c0dur
        bne C0env
        inc c0idx
        jsr Adv0
C0env:  ldx c0eoff
        txa
        clc
        adc c0eidx
        tax
        lda Env,x
        sta AUDV0           ; ← 音量はエンベロープ駆動
        lda c0eidx          ; envIdx < sus なら進める、達したら保持
        cmp c0sus
        bcs C0hold
        inc c0eidx
C0hold:

        ; --- ch1: ノート進行 → エンベロープ step ---
        dec c1dur
        bne C1env
        inc c1idx
        jsr Adv1
C1env:  ldx c1eoff
        txa
        clc
        adc c1eidx
        tax
        lda Env,x
        sta AUDV1
        lda c1eidx
        cmp c1sus
        bcs C1hold
        inc c1eidx
C1hold:

OSwait: lda INTIM
        bne OSwait
        jmp NextFrame

; ===== ノート読込（idx は呼び出し側で管理）=====
; Adv0: ch0 の現ノートをロード（AUDF/AUDC/楽器エンベロープ）。envIdx=0 でトリガ。
Adv0:   ldx c0idx
        cpx #P0LEN
        bcc Adv0b
        ldx #0
        stx c0idx
Adv0b:  lda Durs0,x
        sta c0dur
        lda #0
        sta c0eidx          ; ノートトリガ＝エンベロープ先頭から
        lda Notes0,x
        cmp #$FF
        beq Rest0
        sta AUDF0
        ldy Inst0,x         ; 楽器選択
        lda InstAudc,y
        sta AUDC0
        lda InstSus,y
        sta c0sus
        lda InstEoff,y
        sta c0eoff
        rts
Rest0:  lda #0              ; 休符 = 無音セル(Env[0]) を指す・sus=0
        sta c0eoff
        sta c0sus
        rts

; Adv1: ch1 版
Adv1:   ldx c1idx
        cpx #P1LEN
        bcc Adv1b
        ldx #0
        stx c1idx
Adv1b:  lda Durs1,x
        sta c1dur
        lda #0
        sta c1eidx
        lda Notes1,x
        cmp #$FF
        beq Rest1
        sta AUDF1
        ldy Inst1,x
        lda InstAudc,y
        sta AUDC1
        lda InstSus,y
        sta c1sus
        lda InstEoff,y
        sta c1eoff
        rts
Rest1:  lda #0
        sta c1eoff
        sta c1sus
        rts

; ===== 楽器テーブル =====
; 楽器0 = lead   AUDC4 : Env 15,12,10,8 (sus=3 で 8 保持)        offset 1
; 楽器1 = bass   AUDC12: Env 11,9,7      (sus=2 で 7 保持)        offset 5
; 楽器2 = pluck  AUDC4 : Env 15,10,6,3,1,0 (sus=5 で 0 = 減衰消音) offset 8
InstAudc: byte 4, 12, 4
InstSus:  byte 3,  2, 5
InstEoff: byte 1,  5, 8

; Env[0]=0 は休符用の無音セル。各楽器のエンベロープは offset 1 以降。
Env:    byte 0                       ; off0: 無音
        byte 15,12,10,8              ; off1..4: 楽器0 lead
        byte 11,9,7                  ; off5..7: 楽器1 bass
        byte 15,10,6,3,1,0           ; off8..13: 楽器2 pluck

; ===== ソング（パターン）=====
; ch0 lead: C5 E5 G5 C6 A5 G5 E5 G5 を 楽器0/2 交互（楽器選択の実演）
Notes0: byte $1D,$17,$13,$0E,$11,$13,$17,$13
Inst0:  byte   0,  2,  0,  2,  0,  2,  0,  2
Durs0:  byte $10,$10,$10,$10,$10,$10,$10,$10
; ch1 bass: C4 F4 G4 C4（全て楽器1）
Notes1: byte $13,$0E,$0C,$13
Inst1:  byte   1,  1,  1,  1
Durs1:  byte $20,$20,$20,$30

        org $FFFC
        .word Start
        .word Start
