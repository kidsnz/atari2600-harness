; litmus_cycles.asm — read_cycles（B-1）の検証用 最小決定的 ROM
; 目的: CPU サイクル累積カウンタが「実機の実行サイクル」を正しく数えているかを数値で裏取りする。
; 仕掛け: WSYNC を一切使わない無限ループ。CPU は決して停止しない（RdyFlg 常時 true）。
;   → 不変条件「実行 CPU サイクル × 3 == 進んだカラークロック数」が命令境界で厳密に成立。
;     これはプログラム内容に依存しない普遍則なので、read_cycles の total を beam 座標と突き合わせるだけで
;     カウンタの正しさを検証できる（鉄則1: 判定は数値）。
; include は使わず自己完結。

        processor 6502

        org $F000

Reset:
        sei
        cld
Loop:
        nop             ; 2 cy
        nop             ; 2 cy
        nop             ; 2 cy
        nop             ; 2 cy
        jmp Loop        ; 3 cy（合計 11 cy/反復・WSYNC なし＝CPU 連続実行）

; --- vectors ---
        org $FFFC
        .word Reset     ; RESET
        .word Reset     ; IRQ/BRK
