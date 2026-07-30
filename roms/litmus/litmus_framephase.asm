; litmus_framephase.asm — オラクル間の「Nフレーム後」の意味を数値で突き合わせる ROM（G6）。
;
; 目的: RAM ダンプ型オラクル（Gopher2600 / MAME / Stella）は「N フレーム走らせて RAM を読む」
;   と言うが、**フレーム内のどこで読むか**は各エンジンの都合で決まる。Gopher は
;   RunFrames(N) 後の境界、MAME は machine frame notifier の N 回目。毎フレーム変化する
;   RAM を持つゲームでは、この位相差がそのまま「オラクルの不一致」に化ける——あるいは
;   もっと悪いことに、位相が偶然そろっている間だけ一致して見える。
;
; 手法: 1 フレームの中の**離れた 3 点**でそれぞれ別のカウンタを進める。
;   $80 … VSYNC 直後（フレームの先頭）
;   $81 … 可視領域の中央（192 ラインの 96 本目）
;   $82 … Overscan の最後（次の VSYNC の直前）
;   どの瞬間にダンプしても、この 3 つの関係が「どこで採ったか」を一意に指す:
;     先頭で採れば $80 == $81+1 == $82+1、末尾で採れば $80 == $81 == $82。
;   $83 は 3 点すべてを通過した完全なフレーム数（$82 と同値だが独立に数える＝計測器の自己検査）。
;
; 出典: 自前の検証根拠（G6・capability-gap-audit.md）。smoke.asm の NTSC 骨格を流用。

        processor 6502

VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
COLUBK  = $09
CXCLR   = $2C

        org $F000

Reset:
        sei
        cld
        ldx #$FF
        txs
        lda #0
ClearMem:
        sta $00,x
        dex
        bne ClearMem
        sta CXCLR

MainLoop:
; --- VSYNC: 3 lines ---
        lda #2
        sta VBLANK
        sta VSYNC
        sta WSYNC
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        inc $80         ; ★POINT A: frame start (immediately after VSYNC)

; --- VBLANK: 37 lines ---
        ldx #37
VBlankLoop:
        sta WSYNC
        dex
        bne VBlankLoop

        lda #0
        sta VBLANK

; --- Visible: 192 lines, with POINT B at the midpoint ---
        lda #$1E
        sta COLUBK
        ldx #192
VisibleLoop:
        sta WSYNC
        cpx #96
        bne NoMid
        inc $81         ; ★POINT B: middle of the visible field
NoMid:
        dex
        bne VisibleLoop

; --- Overscan: 30 lines ---
        lda #2
        sta VBLANK
        ldx #30
OverscanLoop:
        sta WSYNC
        dex
        bne OverscanLoop

        inc $82         ; ★POINT C: last instruction before the next VSYNC
        inc $83         ; a second, independent count of complete frames

        jmp MainLoop

        org $FFFC
        .word Reset
        .word Reset
