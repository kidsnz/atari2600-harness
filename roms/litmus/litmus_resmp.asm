; litmus_resmp — RESMP（ミサイルのプレイヤーロック）解除時の出現位置を実測（U-M5）
; 文書では「ロック中はプレイヤー中央に隠れ、解除でそこに出現」（正確なオフセット未検証だった）。
; 手順: P0 を固定位置に配置 → RESMP0=2 でロック → frame 10 で解除 → missile0 と player0 の
; 位置差を read_tia で測る。frame 20 で HMOVE により P0 を +8 右へ → frame 30 で再ロック→解除
; → オフセットが追従するか確認。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
RESP0   = $10
RESMP0  = $28
HMP0    = $20
HMOVE   = $2A
HMCLR   = $2B
GRP0    = $1B
ENAM0   = $1D

fc      = $80       ; フレームカウンタ

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
        ; P0 配置（固定サイクル）
        sta WSYNC
        ds 13, $EA          ; SLEEP 26
        sta RESP0
        lda #2
        sta RESMP0          ; ロック
        lda #$FF
        sta GRP0            ; P0 可視（位置読みの都合）
        lda #2
        sta ENAM0           ; ミサイル有効（解除後に見える）

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
        lda #2
        sta VBLANK
        ; --- overscan: フレーム計画 ---
        inc fc
        lda fc
        cmp #10
        bne Chk20
        lda #0
        sta RESMP0          ; 解除 → ミサイル出現
Chk20:  lda fc
        cmp #20
        bne Chk30
        lda #$80            ; P0 を右へ 8（HMOVE）
        sta HMP0
        sta WSYNC
        sta HMOVE
        ds 12, $EA          ; SLEEP 24
        sta HMCLR
        ldx #28             ; HMOVE 行で 1 行使用済み → 帳尻
        bne OS
Chk30:  lda fc
        cmp #30
        bne Chk31
        lda #2
        sta RESMP0          ; 再ロック
Chk31:  lda fc
        cmp #31
        bne OSrun
        lda #0
        sta RESMP0          ; 再解除
OSrun:  ldx #29
OS:     sta WSYNC
        dex
        bne OS
        sta WSYNC
        jmp NextFrame

        org $FFFC
        .word Start
        .word Start
