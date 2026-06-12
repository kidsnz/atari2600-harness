; litmus_bank_f6 — F6 (16K, 4バンク) bankswitch の実機裏取り（overnight M-C）
; litmus_bank(F8) の型を一般化: 全バンクに ベクタ＋同一リセットスタブ、$FF00 の切替ゾーンで
; 毎フレーム overscan に bank0→1→…→3→0 のチェーン巡回。各バンクが $90=ID, $9k++ を刻む。
; 期待: $90=フレーム末 $B3（最終バンク実行の証拠）、$91..$93 が同数で増加、境界 bank.number=0。
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
        jsr $FF00          ; 全バンク・チェーン巡回
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame
        ORG  $0F00
        RORG $FF00
        lda $FFF7           ; bank1 へ
        ds 27, $EA
        rts                 ; 最終バンクが bank0 hotspot を読んだ直後ここへ
        ORG  $0FE0
        RORG $FFE0
        lda $FFF6           ; どのバンクで起動しても bank0
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
        lda $FFF8           ; 次へ
        ORG  $1FE0
        RORG $FFE0
        lda $FFF6           ; どのバンクで起動しても bank0
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
        lda $FFF9           ; 次へ
        ORG  $2FE0
        RORG $FFE0
        lda $FFF6           ; どのバンクで起動しても bank0
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
        lda $FFF6           ; 次へ
        ORG  $3FE0
        RORG $FFE0
        lda $FFF6           ; どのバンクで起動しても bank0
        jmp $F000
        ORG  $3FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0
