; maze — Entombed-style procedural playfield maze (technique: maze, 生成系)
; Reference: ../reference/atariage/296383-entombed-maze-origins/notes.ja.md
;   ・疑似乱数(LFSR)の別々の nybble を PF1 / PF2 に分配
;   ・各乱数ビットを 2 回 rol して 2px 幅に展開（通路をプレイヤーが通れる幅に）
;   ・$8F,x→$90,x のシフトループで迷路バッファを縦方向へ 1 ステップ送る（スクロール）
; 論文が騒いだ「32 バイトテーブル」は逆アセの中間表現＝red herring。生成は手続き型。
;
; 本デモ:
;   ・固定シード($5A)の 8bit Galois LFSR（litmus_lfsr と同形, period 255, 非ゼロ）
;   ・迷路は左半分のみ生成し CTRLPF reflect で左右対称（Entombed と同じ対称迷路）
;   ・毎フレーム上端へ新規 1 行を生成し、バッファ全体を下へシフト＝下スクロール
;   ・各バッファ行を 1 行のうち BAND_H 走査線ぶん表示（行が見える高さを持つ）
;   ・scroll カウンタを毎フレーム inc（スクロールが進んでいる証跡）
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

; ----- RAM レイアウト -----
lfsr    = $80           ; LFSR 状態（非ゼロ維持・周期 255）
scroll  = $81           ; スクロールカウンタ（毎フレーム +1）
tmp     = $82           ; LFSR 展開の作業バイト
tmp2    = $83
; 迷路バッファ: ROWS 行ぶんの PF1 / PF2（左半分の壁パターン）
ROWS    = 16
BAND_H  = 12            ; 1 バッファ行が占める走査線数（16*12 = 192 可視行）
pf1buf  = $90           ; $90..$9F = PF1 側 16 バイト
pf2buf  = $A0           ; $A0..$AF = PF2 側 16 バイト

SEED    = $5A

        org $F000
Start:  sei
        cld
        ldx #$FF
        txs
        lda #0
Clr:    sta $00,x
        dex
        bne Clr

        lda #SEED
        sta lfsr
        ; 初期バッファをシードから全行生成（起動時から迷路が埋まっている）
        ldx #0
InitRow:
        jsr GenRow          ; tmp=PF1, tmp2=PF2 を生成
        lda tmp
        sta pf1buf,x
        lda tmp2
        sta pf2buf,x
        inx
        cpx #ROWS
        bne InitRow

        ; 描画設定: 反射(対称)・壁色・背景
        lda #$01            ; CTRLPF D0=1 → reflect（左右対称迷路）
        sta CTRLPF
        lda #$1E            ; 壁色（黄白）
        sta COLUPF
        lda #$00
        sta COLUBK

NextFrame:
        lda #2
        sta VSYNC
        sta WSYNC           ; VSYNC 3 行
        sta WSYNC
        sta WSYNC
        lda #0
        sta VSYNC
        lda #2
        sta VBLANK

        ; ----- VBLANK 中に迷路を更新（全 CPU 処理をここで完結） -----
        ; バッファを下へ 1 行シフト（Entombed $8F,x→$90,x のスクロール相当）
        ; 末尾→先頭の順にコピーして上書き衝突を防ぐ
        ldx #ROWS-1
Shift:
        dex
        lda pf1buf,x
        sta pf1buf+1,x
        lda pf2buf,x
        sta pf2buf+1,x
        cpx #0
        bne Shift
        ; 先頭 row0 に新規行を生成（迷路が上から流れ込む = 下スクロール）
        jsr GenRow
        lda tmp
        sta pf1buf
        lda tmp2
        sta pf2buf
        inc scroll          ; スクロール進捗（毎フレーム +1）

        ; VBLANK を 37 行に整形。Shift+GenRow が WSYNC 無しで約 8 行ぶんの
        ; ビーム移動を消費するため、ループは 29 行ぶんにして 8+29=37 に合わせる。
        ldx #29
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; ===== 可視 192 行: ROWS(16) 行 × BAND_H(12) 走査線 =====
        ldx #0              ; X = バッファ行 index
RowLoop:
        lda pf1buf,x
        sta tmp             ; このバンドの PF1
        lda pf2buf,x
        sta tmp2            ; このバンドの PF2
        ldy #BAND_H
Band:   sta WSYNC
        lda tmp
        sta PF1
        lda tmp2
        sta PF2
        dey
        bne Band
        inx
        cpx #ROWS
        bne RowLoop

        ; ===== Overscan 30 行 =====
        lda #0
        sta PF1
        sta PF2
        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame

; ===== GenRow: LFSR を回し PF1/PF2 行を生成（各乱数ビットを 2px 幅に展開） =====
; 入力: lfsr / 出力: tmp=PF1, tmp2=PF2、lfsr 前進
; Entombed カーネル本質: 乱数の別バイトを PF1/PF2 へ振り分け、各ビットを
;   `lsr src` で 1 ビット取り出し → そのキャリーを `rol dest` で 2 回入れる
;   ＝ 1 乱数ビットが PF の 2 ビット(=2px)幅になる。原典の rol×2 と同じ展開。
; src の低 4 ビット → dest 8 ビット（2px×4 = 8px ぶんの壁/通路パターン）。
gsrc1   = $86
gsrc2   = $87
gcnt    = $88           ; ビット展開カウンタ（X/Y を汚さない＝呼び出し側のループ index を保護）
GenRow:
        jsr StepLFSR
        sta gsrc1           ; PF1 用乱数
        jsr StepLFSR
        sta gsrc2           ; PF2 用乱数
        lda #0
        sta tmp             ; PF1 出力
        sta tmp2            ; PF2 出力
        lda #4              ; 低 4 ビットを 2px ずつ展開
        sta gcnt
GenLoop:
        ; --- PF1 側: gsrc1 の 1 ビットを tmp に 2 回入れる ---
        lsr gsrc1           ; 取り出したビット → C（gsrc1 は 1 ビット消費）
        bcc G1lo
        sec                 ; C=1 を 2 回入れる
        rol tmp
        sec
        rol tmp
        jmp G2
G1lo:   clc                 ; C=0 を 2 回入れる
        rol tmp
        clc
        rol tmp
G2:     ; --- PF2 側: gsrc2 の 1 ビットを tmp2 に 2 回入れる ---
        lsr gsrc2
        bcc G2lo
        sec
        rol tmp2
        sec
        rol tmp2
        jmp Gnext
G2lo:   clc
        rol tmp2
        clc
        rol tmp2
Gnext:  dec gcnt
        bne GenLoop
        rts

; ===== StepLFSR: 8bit Galois LFSR（litmus_lfsr と同形） =====
StepLFSR:
        lda lfsr
        lsr
        bcc NoTap
        eor #$8E
NoTap:  sta lfsr
        rts

        org $FFFC
        .word Start
        .word Start
