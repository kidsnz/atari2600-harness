; asym Monet shimmer — internal/playfield.GenerateAsymmetricShimmerASM
; 非対称 Monet を水面きらめきアニメ化。RAM 水テーブルを毎フレーム VBLANK でスクロール再充填。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUPF  = $08
COLUBK  = $09
CTRLPF  = $0A
PF0     = $0D
PF1     = $0E
PF2     = $0F
INTIM   = $0284
TIM64T  = $0296
PFHGT   = $80
Offset  = $81
SubCnt  = $82
WaterRAM = $90

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
        lda #0
        sta CTRLPF
        sta Offset
        sta SubCnt
        lda #$C8
        sta COLUPF      ; 睡蓮（定数 COLUPF）

NextFrame:
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #44
        sta TIM64T      ; VBLANK タイマー

        ; --- スクロール位置更新（5 フレームに 1 シフト）---
        inc SubCnt
        lda SubCnt
        cmp #5
        bcc NoShift
        lda #0
        sta SubCnt
        inc Offset
        lda Offset
        cmp #48
        bcc NoShift
        lda #0
        sta Offset
NoShift:
        ; --- WaterRAM[i] = BGCycle[Offset+i], i=0..rows-1 ---
        ldx #47
Fill:   txa
        clc
        adc Offset
        tay
        lda BGCycle,y
        sta WaterRAM,x
        dex
        bpl Fill

WaitVB: lda INTIM
        bne WaitVB
        sta WSYNC
        lda #0
        sta VBLANK

        ldx #47
        lda #4
        sta PFHGT
PFLoop:
        sta WSYNC                 ; (0)
        lda WaterRAM,x            ; (4)  per-row 水（RAM・アニメ）
        sta COLUBK                ; (7)
        lda PF0DataA,x            ; (11)
        sta PF0                   ; (14)
        lda PF1DataA,x            ; (18)
        sta PF1                   ; (21)
        lda PF2DataA,x            ; (25)
        sta PF2                   ; (28)
        lda PF0DataB,x            ; (32)
        tay                       ; (34)
        lda PF1DataB,x            ; (38)
        sta PF1                   ; (41)
        sty PF0                   ; (44)
        lda PF2DataB,x            ; (48)
        sta PF2                   ; (51)
        dec PFHGT                 ; (56)
        bne PFSkip
        lda #4
        sta PFHGT
        dex
        cpx #$FF
        beq PFDone
PFSkip:
        jmp PFLoop
PFDone:
        lda #0
        sta PF0
        sta PF1
        sta PF2
        sta COLUBK
        lda #2
        sta VBLANK
        lda #35
        sta TIM64T      ; Overscan タイマー
WaitOS: lda INTIM
        bne WaitOS
        sta WSYNC
        jmp NextFrame

PF0DataA
        .byte $00
        .byte $00
        .byte $00
        .byte $C0
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $80
        .byte $80
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $E0
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00

PF1DataA
        .byte $00
        .byte $00
        .byte $00
        .byte $80
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $1E
        .byte $1E
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $F0
        .byte $F0
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $80
        .byte $00
        .byte $00
        .byte $00
        .byte $07
        .byte $07
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $78
        .byte $78
        .byte $00
        .byte $00

PF2DataA
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $E0
        .byte $E0
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $F8
        .byte $F8
        .byte $F8
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $0F
        .byte $0F
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $C0
        .byte $C0
        .byte $00
        .byte $00
        .byte $00

PF0DataB
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $10
        .byte $10
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $C0
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $F0
        .byte $F0
        .byte $F0
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $70
        .byte $70
        .byte $00
        .byte $00
        .byte $00

PF1DataB
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $7C
        .byte $7C
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $C0
        .byte $00
        .byte $00
        .byte $00
        .byte $03
        .byte $03
        .byte $00
        .byte $00
        .byte $00
        .byte $C0
        .byte $C0
        .byte $C0
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $0F
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00

PF2DataB
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $38
        .byte $38
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $03
        .byte $03
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $3E
        .byte $3E
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00
        .byte $00

BGCycle
        .byte $9A
        .byte $94
        .byte $9A
        .byte $7C
        .byte $7A
        .byte $88
        .byte $74
        .byte $C4
        .byte $C4
        .byte $B6
        .byte $C8
        .byte $BC
        .byte $64
        .byte $B6
        .byte $64
        .byte $5C
        .byte $5A
        .byte $5A
        .byte $98
        .byte $56
        .byte $5C
        .byte $9C
        .byte $96
        .byte $8A
        .byte $84
        .byte $86
        .byte $88
        .byte $7C
        .byte $C4
        .byte $C6
        .byte $C4
        .byte $B4
        .byte $C6
        .byte $BA
        .byte $B8
        .byte $6A
        .byte $6A
        .byte $5A
        .byte $56
        .byte $56
        .byte $54
        .byte $56
        .byte $8C
        .byte $9C
        .byte $96
        .byte $78
        .byte $8A
        .byte $78
        .byte $9A
        .byte $94
        .byte $9A
        .byte $7C
        .byte $7A
        .byte $88
        .byte $74
        .byte $C4
        .byte $C4
        .byte $B6
        .byte $C8
        .byte $BC
        .byte $64
        .byte $B6
        .byte $64
        .byte $5C
        .byte $5A
        .byte $5A
        .byte $98
        .byte $56
        .byte $5C
        .byte $9C
        .byte $96
        .byte $8A
        .byte $84
        .byte $86
        .byte $88
        .byte $7C
        .byte $C4
        .byte $C6
        .byte $C4
        .byte $B4
        .byte $C6
        .byte $BA
        .byte $B8
        .byte $6A
        .byte $6A
        .byte $5A
        .byte $56
        .byte $56
        .byte $54
        .byte $56
        .byte $8C
        .byte $9C
        .byte $96
        .byte $78
        .byte $8A
        .byte $78

        org $FFFC
        .word Start
        .word Start
