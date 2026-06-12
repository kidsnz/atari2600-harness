; litmus_bank_f4 — F4 (32K, 8バンク) bankswitch の実機裏取り（overnight M-C）
; litmus_bank(F8) の型を一般化: 全バンクに ベクタ＋同一リセットスタブ、$FF00 の切替ゾーンで
; 毎フレーム overscan に bank0→1→…→7→0 のチェーン巡回。各バンクが $90=ID, $9k++ を刻む。
; 期待: $90=フレーム末 $B7（最終バンク実行の証拠）、$91..$97 が同数で増加、境界 bank.number=0。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02

; ================= bank 0 =================
        ORG  $0000
        RORG $F000
Start:
        sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr
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
        ldy #192
Vis:    sta WSYNC
        dey
        bne Vis
        lda #2
        sta VBLANK
        jsr $FF00          ; 全バンク・チェーン巡回（8 セグメント ≈130cy ＝ 2 行に零れる）
        ldx #29             ; 零れた 1 行ぶん overscan を詰めて 262 を維持
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame
        ORG  $0F00
        RORG $FF00
        lda $FFF5           ; bank1 へ
        ds 63, $EA
        rts                 ; 最終バンクが bank0 hotspot を読んだ直後ここへ
        ORG  $0FE0
        RORG $FFE0
        lda $FFF4           ; どのバンクで起動しても bank0
        jmp $F000
        ORG  $0FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0

; ================= bank 1 =================
        ORG  $1000
        RORG $F000
        ORG  $1F00
        RORG $FF00
        ds 3, $EA
        lda #$B1
        sta $90
        inc $91
        lda $FFF6           ; 次へ
        ORG  $1FE0
        RORG $FFE0
        lda $FFF4           ; どのバンクで起動しても bank0
        jmp $F000
        ORG  $1FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0

; ================= bank 2 =================
        ORG  $2000
        RORG $F000
        ORG  $2F00
        RORG $FF00
        ds 12, $EA
        lda #$B2
        sta $90
        inc $92
        lda $FFF7           ; 次へ
        ORG  $2FE0
        RORG $FFE0
        lda $FFF4           ; どのバンクで起動しても bank0
        jmp $F000
        ORG  $2FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0

; ================= bank 3 =================
        ORG  $3000
        RORG $F000
        ORG  $3F00
        RORG $FF00
        ds 21, $EA
        lda #$B3
        sta $90
        inc $93
        lda $FFF8           ; 次へ
        ORG  $3FE0
        RORG $FFE0
        lda $FFF4           ; どのバンクで起動しても bank0
        jmp $F000
        ORG  $3FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0

; ================= bank 4 =================
        ORG  $4000
        RORG $F000
        ORG  $4F00
        RORG $FF00
        ds 30, $EA
        lda #$B4
        sta $90
        inc $94
        lda $FFF9           ; 次へ
        ORG  $4FE0
        RORG $FFE0
        lda $FFF4           ; どのバンクで起動しても bank0
        jmp $F000
        ORG  $4FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0

; ================= bank 5 =================
        ORG  $5000
        RORG $F000
        ORG  $5F00
        RORG $FF00
        ds 39, $EA
        lda #$B5
        sta $90
        inc $95
        lda $FFFA           ; 次へ
        ORG  $5FE0
        RORG $FFE0
        lda $FFF4           ; どのバンクで起動しても bank0
        jmp $F000
        ORG  $5FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0

; ================= bank 6 =================
        ORG  $6000
        RORG $F000
        ORG  $6F00
        RORG $FF00
        ds 48, $EA
        lda #$B6
        sta $90
        inc $96
        lda $FFFB           ; 次へ
        ORG  $6FE0
        RORG $FFE0
        lda $FFF4           ; どのバンクで起動しても bank0
        jmp $F000
        ORG  $6FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0

; ================= bank 7 =================
        ORG  $7000
        RORG $F000
        ORG  $7F00
        RORG $FF00
        ds 57, $EA
        lda #$B7
        sta $90
        inc $97
        lda $FFF4           ; 次へ
        ORG  $7FE0
        RORG $FFE0
        lda $FFF4           ; どのバンクで起動しても bank0
        jmp $F000
        ORG  $7FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0
