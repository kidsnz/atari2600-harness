; litmus_hmove_side — HMOVE の副作用を実機裏取り（V2-2）
; (a) コーム: WSYNC 直後の HMOVE ストローブは HMxx 全0でも「左8pxを黒く」する（HBLANK+8CLK 延長, Towers TIA_HW_Notes）。
;     バンドA(32行)で1行おきにストローブ → 偶奇で左端 8px の有無を read_row 比較。
; (b) late HMOVE: 可視中（〜cycle 40）のストローブの変位を測る（Towers: plugging=右移動）。
;     バンドB(16行)で毎行ストローブ → P0 バーの x が行ごとにどう動くか read_row で実測。
; P0 は毎フレーム VBLANK で再配置（バンドBの変位の累積をフレーム間に持ち越さない＝決定的）。
; 実機裏取り済（Gopher2600, v0.40.0）:
;  (a) コーム: HMxx全0でも WSYNC直後 HMOVE の行は左8pxが黒（read_row(11,13,33)=clock0-7黒）。非ストローブ行は無し。✓Towers
;  (b) mid-visible (cycle〜39) の HMOVE: HM=0 でも $10 でも 変位ゼロ・コーム無し（バンドB/C全行 X=9 不変）。
;  (c) 行末 (cycle〜74) の HMOVE: HMP0=$10(+1) で 左9px/ストローブ ＝ 値+8（late HMOVE の +8 加算を数値確認）、コーム無し。
;  ※(b)(c)は Gopher2600 の実測値。実シリコンとの照合は F-4 Stella オラクル（V2-17）で。回帰固定=scenarios/hmove_side.json。
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
        sta HMCLR           ; HMxx 全 0
        lda #$86
        sta COLUBK          ; 青背景（コームの黒が見えるように）
        lda #$0E
        sta COLUP0
        lda #$FF
        sta GRP0            ; P0 = 8px バー（常時表示のマーカー）

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
        ; P0 再配置（毎フレーム）: WSYNC 後 約20cy 遅延 → 可視内ストローブ
        sta WSYNC
        ldx #4
Pd:     dex
        bne Pd              ; 2+19=21cy
        sta RESP0
        sta HMCLR           ; 念のため毎フレーム 0
        ldx #36
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; --- バンドA (32行): 偶数行=WSYNC直後 HMOVE / 奇数行=なし（HMxx は全0）---
        ldy #16
BA:     sta WSYNC
        sta HMOVE           ; cycle 3 で完了 ＝ WSYNC 直後
        sta WSYNC           ; ストローブ無し行
        dey
        bne BA

        ; --- バンドB (16行): 可視中 (〜cycle 40) で HMOVE ストローブ（HMxx 全0）---
        ldy #16
BB:     sta WSYNC
        ldx #7
Bd:     dex
        bne Bd              ; 2+34=36cy
        sta HMOVE           ; 〜cycle 39 で完了（可視域内）
        dey
        bne BB

        ; --- バンドC (16行): HMP0=$10（左1）で可視中ストローブ → plugging の変位を実測 ---
        lda #$10
        sta HMP0
        ldy #16
BC:     sta WSYNC
        ldx #7
Cd:     dex
        bne Cd
        sta HMOVE           ; 〜cycle 39（可視域内）、HMP0=$10
        dey
        bne BC
        sta HMCLR           ; 後始末

        ; --- バンドD (32行=16回): HMP0=$10 で cycle〜74 ストローブ（行末 HMOVE 技: コーム無し移動の検証）---
        ; 注: HMOVE(74)+dey/bne が行を跨ぐため 1 イテレーション=2行（ストローブは1行おき）。
        lda #$10
        sta HMP0
        ldy #16
BD:     sta WSYNC
        ldx #14
Dd:     dex
        bne Dd              ; 2+69=71cy
        sta HMOVE           ; 〜cycle 74 で完了（行末）
        dey
        bne BD
        sta HMCLR

        ; --- 残り 96 行: ストローブ無し（32+16+16+32+96=192）---
        ldy #96
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
