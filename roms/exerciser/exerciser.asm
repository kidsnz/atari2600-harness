; exerciser — 全能力統合のショーケース ROM（Road to 1.0）
; M1: F8 2バンク骨格＋シーンディスパッチ＋fire 切替（v0.56.0 検証済）。
; M2: S0=タイトル（bank1）: 48px "EXRCSR" ロゴ（6書換 kernel・テーブル参照版）＋6桁 BCD スコア。
;
; シーン契約: 「ちょうど 192 scanline を消費するサブルーチン」。フレーム骨格は bank0 が所有。
; シーン: 0=Title(bank1) / 1=Blue placeholder(bank0)。fire 押下エッジで前進・wrap。
;
; RAM マップ:
;  グローバル: $80=scene  $81=prevFire  $82=frameCt  $90-$92=スコアBCD(6桁,LE)  $9E/$9F=実行センチネル
;  Title ローカル: $A0=row  $A1-$A3=t3,t4,t5  $A4-$A6=t0,t1,t2  $A8-$B3=p0..p5(各2B, digit ptr)
;
; 48px ロゴ kernel（litmus_48px6 の検証済み振付のテーブル参照版・1ライン/行・行倍化テーブルで16行）:
;  tail(前行末): B5→t5, B0→GRP0, B1→A, WSYNC
;  head: sta GRP1(B1)@3 → B2→GRP0@10 → B3/B4/B5→A/X/Y@21 → SLEEP10 → timed store 完了 34/37/40/43
; スコア kernel（2ライン交互=stage/draw, 縞表示）:
;  stage 行: (p0..p5),y から t0..t5 へ（76cy 内で余裕）
;  draw 行: zp 21cy プリロード → SLEEP4 → timed 34/37/40/43（litmus_48px6 と同一）
; M3 実機裏取り済（v0.58.0）: 非対称山並み2種（PF1 早書き≤27/右38=pf_async 窓）＋6ゾーン12スプライト・ドリフト
;（4行分割で各行≤76cy）＋地面。シーン進入初期化=lastScene 方式。264→262 は assert_line_budget 系の勘定で調整。
; 実機裏取り済（Gopher2600, v0.57.0）: 48px "EXRCSR" 可読（P0=24/P1=32, NUSIZ=3, VDEL, timed 34/37/40/43）。
; 6桁BCDスコアがフレーム毎 +1（frame123 で "000122" 表示・$90-92 BCD 一致）。スコアは stage/draw 交互（縞）。
; 修正史: ①ロゴ表 align 256（abs,y ページ跨ぎ+1cy のシアー） ②ループ折返し jmp を WSYNC 後 0-2cy に置く +3 シフト head
; ③stage 行頭でスプライト消灯（VDEL 影の残骸） ④GLYPH を逆順格納（row 降順描画で正立）。回帰固定=scenarios/m2_title.json。
        processor 6502
VSYNC   = $00
VBLANK  = $01
WSYNC   = $02
NUSIZ0  = $04
NUSIZ1  = $05
COLUP0  = $06
COLUP1  = $07
COLUBK  = $09
RESP0   = $10
RESP1   = $11
GRP0    = $1B
GRP1    = $1C
HMP0    = $20
HMP1    = $21
HMOVE   = $2A
HMCLR   = $2B
VDELP0  = $25
VDELP1  = $26
INPT4   = $3C
INPT0   = $38
AUDC0   = $15
AUDF0   = $17
AUDV0   = $19
AUDC1   = $16
AUDF1   = $18
AUDV1   = $1A
SWCHA   = $0280
ENAM0   = $1D
ENABL   = $1F
RESM0   = $12
RESBL   = $14
HMM0    = $22
PF0     = $0D
PF2     = $0F
CXCLR   = $2C
CTRLPF  = $0A
COLUPF  = $08
PF1     = $0E

NSCENES = 5

scene    = $80
prevFire = $81
frameCt  = $82
lastScene = $83     ; シーン進入検出（初期化 1 回/進入）
m0idx    = $84      ; 音楽: ch0 ノート index
m0dur    = $85      ; 音楽: ch0 残フレーム
m1idx    = $86
m1dur    = $87
score0   = $90      ; BCD 下2桁
score1   = $91
score2   = $92      ; BCD 上2桁
sent0    = $9E
sent1    = $9F
row      = $A0
t3       = $A1
t4       = $A2
t5       = $A3
t0       = $A4
t1       = $A5
t2       = $A6
p0       = $A8      ; digit ptr ×6（各2B: $A8,$AA,$AC,$AE,$B0,$B2）
p1       = $AA
p2       = $AC
p3       = $AE
p4       = $B0
p5       = $B2
; Zone シーン・ローカル（Title と排他オーバーレイ）
zx0      = $A4      ; P0 X ×6（$A4-$A9）
zx1      = $AA      ; P1 X ×6（$AA-$AF）
; Paddle シーン・ローカル
pdlCnt   = $B6      ; 今フレームの充電カウント
pdlPos   = $B7      ; 前フレーム確定のカーソル X（描画用）
pdlDone  = $B8
; Gradient シーン・ローカル
sfxTmr   = $B9      ; SFX 減衰タイマ
; Procedural シーン・ローカル
worldSeed = $BA     ; 現在の世界シード（毎フレーム進む＝星空スクロール）
rnd      = $BB      ; 行毎 LFSR 作業用
mPF0     = $C0      ; 山バンド PF0 ×10（$C0-$C9, 上→下）
mPF1     = $CA      ; 山バンド PF1 ×10（$CA-$D3）
mPF2     = $D4      ; 山バンド PF2 ×10（$D4-$DD）
mrnd     = $DE      ; 山生成シード（進入時のみ使用）
t6       = $DF      ; 星空パイプライン作業用（今行の描画値）
t7       = $BC      ; 星空: 前行のペア値（2連ANDで密度 6.25% に間引く）
; Playground シーン・ローカル（排他オーバーレイ）
px       = $A4      ; P0 X（10-140）
py       = $A5      ; P0 Y（10-170）
mx       = $A6      ; missile X
mAct     = $A7      ; missile 飛行中フラグ
hitCol   = $A9      ; 衝突フィードバック色
lineCt   = $B4      ; kernel 行カウンタ

; ================= bank 0: フレームワーク＋S1 =================
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
        sta $2C             ; CXCLR
        lda #$FF
        sta lastScene       ; 全シーンの進入初期化を初回必ず発火させる

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
        ; --- 入力: fire 押下エッジで scene++（wrap）---
        lda INPT4
        and #$80
        tay
        cmp prevFire
        beq NoEdge
        cmp #0
        bne NoEdge
        inc scene
        lda scene
        cmp #NSCENES
        bcc NoWrap
        lda #0
        sta scene
NoWrap:
NoEdge: sty prevFire
        inc frameCt
        ldx #37
VB:     sta WSYNC
        dex
        bne VB
        lda #0
        sta VBLANK

        ; --- シーンディスパッチ ---
        lda scene
        beq DoTitle
        cmp #1
        beq DoZone
        cmp #2
        beq DoPlay
        cmp #3
        beq DoGrad
        jsr SceneProc       ; scene 4（bank0）
        jmp AfterScene
DoGrad: jsr SceneGrad       ; scene 3（bank0）
        jmp AfterScene
DoPlay: jsr ScenePlay       ; scene 2（bank0）
        jmp AfterScene
DoZone: jsr SceneZone       ; scene 1（bank0）
        jmp AfterScene
DoTitle:
        jsr CallB1Scene     ; scene 0 = Title（bank1）
AfterScene:

        lda #2
        sta VBLANK
        ldx #30
OS:     sta WSYNC
        dex
        bne OS
        jmp NextFrame


; --- S1: ゾーン風景（bank0）---
; 合成: 非対称PF山並み(48行, pf_async の窓) + 6ゾーン×P0/P1=12個の動くスプライト(96行,
; techniques/zone_multiplex の検証済み構造) + 地面(45行)。先頭 3 行=進入初期化 or ドリフト更新。
SceneZone:
        lda #$C2
        sta $9D             ; 実行センチネル
        lda #0
        sta COLUBK
        sta CTRLPF          ; repeat
        lda #$0E
        sta COLUP0
        lda #$48
        sta COLUP1
        lda #0
        sta NUSIZ0
        sta NUSIZ1
        sta VDELP0
        sta VDELP1
        lda #$D6
        sta COLUPF          ; 山の色（黄緑）
        sta WSYNC           ; 構成行を閉じる（1）
        ; --- 行2-4: 進入初期化（X 初期値コピー）or ドリフト（P0右/P1左, and #$7F）---
        lda lastScene
        cmp #1
        beq ZDrift
        ; 初期化パス（6 行・各行 ≤76cy: 6連コピーは1行 ~82cy で零れる→3+3 分割。R1 で根治）
        lda #0
        sta AUDV0
        sta AUDV1
        ldx #5
ZI1a:   lda ZoneXInit0,x
        sta zx0,x
        dex
        cpx #2
        bne ZI1a
        sta WSYNC           ; 2
ZI1b:   lda ZoneXInit0,x
        sta zx0,x
        dex
        bpl ZI1b
        sta WSYNC           ; 3
        ldx #5
ZI2a:   lda ZoneXInit1,x
        sta zx1,x
        dex
        cpx #2
        bne ZI2a
        sta WSYNC           ; 4
ZI2b:   lda ZoneXInit1,x
        sta zx1,x
        dex
        bpl ZI2b
        sta WSYNC           ; 5
        lda #1
        sta lastScene
        sta WSYNC           ; 6
        sta WSYNC           ; 7
        jmp ZMountains
ZDrift: ; ドリフトパス（6 行: 2ゾーンずつ＝0..159 の全幅ラップでも 76cy 内）
        ldx #5
ZD1:    lda zx0,x
        clc
        adc #1
        cmp #160
        bcc ZD1w
        lda #0
ZD1w:   sta zx0,x
        dex
        cpx #3
        bne ZD1
        sta WSYNC           ; 2
ZD2:    lda zx0,x
        clc
        adc #1
        cmp #160
        bcc ZD2w
        lda #0
ZD2w:   sta zx0,x
        dex
        cpx #1
        bne ZD2
        sta WSYNC           ; 3
ZD3:    lda zx0,x
        clc
        adc #1
        cmp #160
        bcc ZD3w
        lda #0
ZD3w:   sta zx0,x
        dex
        bpl ZD3
        sta WSYNC           ; 4
        ldx #5
ZD4:    lda zx1,x
        sec
        sbc #1
        bcs ZD4w
        lda #159
ZD4w:   sta zx1,x
        dex
        cpx #3
        bne ZD4
        sta WSYNC           ; 5
ZD5:    lda zx1,x
        sec
        sbc #1
        bcs ZD5w
        lda #159
ZD5w:   sta zx1,x
        dex
        cpx #1
        bne ZD5
        sta WSYNC           ; 6
ZD6:    lda zx1,x
        sec
        sbc #1
        bcs ZD6w
        lda #159
ZD6w:   sta zx1,x
        dex
        bpl ZD6
        sta WSYNC           ; 7
ZMountains:
        ; --- 山並み 48 行: 非対称 PF1（左=MtnL 早書き≤27 / 右=MtnR cyc38 書き=窓37-53）---
        ldy #0
ZMt:    sta WSYNC
        lda MtnL,y          ; 4
        sta PF1             ; 7   左 PF1（窓 ≤27 ✓）
        lda MtnR,y          ; 11
        ds 12, $EA          ; 35
        sta PF1             ; 38  右 PF1（窓 37-53 ✓）
        iny
        cpy #48
        bne ZMt
        lda #0
        sta PF1
        ; --- ゾーン 6×16 行（zone_multiplex の検証済み構造）---
        ldx #0
ZLoop:  sta WSYNC
        lda zx0,x
        sec
ZDv0:   sbc #15
        bcs ZDv0
        tay
        lda ZHmove,y
        sta HMP0
        sta RESP0
        sta WSYNC
        lda zx1,x
        sec
ZDv1:   sbc #15
        bcs ZDv1
        tay
        lda ZHmove,y
        sta HMP1
        sta RESP1
        sta WSYNC
        lda ZoneBG,x
        sta COLUBK
        sta HMOVE
        ldy #0
ZSpr:   sta WSYNC
        lda ZSprite,y
        sta GRP0
        sta GRP1
        iny
        cpy #8
        bne ZSpr
        lda #0
        sta GRP0
        sta GRP1
        ldy #5
ZPad:   sta WSYNC
        dey
        bne ZPad
        inx
        cpx #6
        bne ZLoop
        ; --- 地面 41 行（1+6+48+96+41 = 192）---
        lda #$E4
        sta COLUBK
        ldy #41
ZGnd:   sta WSYNC
        dey
        bne ZGnd
        lda #0
        sta COLUBK
        sta HMCLR
        rts

ZoneXInit0:
        .byte 20, 50, 90, 120, 110, 70
ZoneXInit1:
        .byte 100, 70, 30, 60, 95, 120
ZoneBG:
        .byte $84, $94, $A4, $B4, $C4, $24
ZSprite:
        .byte $18,$3C,$7E,$FF,$FF,$7E,$3C,$18
MtnL:   ; 山シルエット左（PF1, きっかり 48 エントリ）
        ds 12, $00
        .byte $01,$01,$03,$03,$07,$07,$0F,$0F,$1F,$1F,$3F,$3F
        .byte $7F,$7F,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF
        ds 12, $FF
MtnR:   ; 山シルエット右（別形状=非対称の証明・きっかり 48 エントリ）
        ds 6, $00
        .byte $80,$80,$C0,$C0,$E0,$E0,$F0,$F0,$F8,$F8,$FC,$FC,$FE,$FE
        .byte $FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF,$FF
        ds 14, $FF


; --- S2: 操作プレイグラウンド（bank0）---
; ジョイスティックで P0 移動（SWCHA, litmus_input）/ ミサイル自動発射（32フレーム毎, 右へ 1px/フレーム=HMOVE ドリフト）
; / PF 側壁＋中央 ball / 衝突ラッチで P0 色が変わる（litmus_collide_all の意味づけ）。CXCLR 毎フレーム。
ScenePlay:
        lda #$D4
        sta $9C             ; 実行センチネル
        lda lastScene
        cmp #2
        beq PNoInit
        ; 進入初期化
        lda #70
        sta px
        lda #80
        sta py
        lda #0
        sta mAct
        sta AUDV0
        sta AUDV1
        lda #2
        sta lastScene
PNoInit:
        ; 構成: PF 側壁（reflect で左右対称・PF0 のみ）, ball 中央
        lda #$F0
        sta PF0
        lda #0
        sta PF1
        sta PF2
        lda #1
        sta CTRLPF          ; reflect
        lda #$3A
        sta COLUPF
        lda #0
        sta COLUBK
        sta NUSIZ0
        lda #$1E
        sta COLUP1
        lda #0
        sta GRP1
        sta WSYNC           ; 1
        ; --- 入力＋移動（行2）---
        lda SWCHA
        and #$80            ; right (0=押下)
        bne PNotR
        inc px
PNotR:  lda SWCHA
        and #$40            ; left
        bne PNotL
        dec px
PNotL:  lda SWCHA
        and #$20            ; down
        bne PNotD
        inc py
PNotD:  lda SWCHA
        and #$10            ; up
        bne PNotU
        dec py
PNotU:  sta WSYNC           ; 2
        ; --- クランプ＋衝突フィードバック（行3）---
        lda px
        cmp #10
        bcs PXlo
        lda #10
        sta px
PXlo:   lda px
        cmp #141
        bcc PXhi
        lda #140
        sta px
PXhi:   lda py
        cmp #10
        bcs PYlo
        lda #10
        sta py
PYlo:   lda py
        cmp #170
        bcc PYhi
        lda #169
        sta py
PYhi:   sta WSYNC           ; 3b（クランプ行と衝突行を分離: 合算 >76cy 対策）
        ; 衝突: 前フレームの描画で立ったラッチを「読んでから消す」（読み→CXCLR の順が肝）
        lda #$0E
        sta hitCol
        bit $32             ; CXP0FB: D7=p0-pf（壁）
        bpl PChkBL
        lda #$44
        sta hitCol          ; 赤=壁ヒット
PChkBL: bit $32             ; CXP0FB D6 = p0-bl（ポール）
        bvc PNoBL2
        lda #$86
        sta hitCol          ; 青=ポールヒット（CXP0FB D6 = p0-bl）
PNoBL2: lda hitCol
        sta COLUP0
        sta CXCLR           ; 読了後にクリア（次フレームの描画で再蓄積）
        sta WSYNC           ; 3
        ; --- ミサイル管理（行4）---
        lda mAct
        bne PMfly
        lda frameCt
        and #$1F
        bne PMnone          ; 32フレーム毎に発射
        lda px
        sta mx
        lda #1
        sta mAct
        jmp PMnone
PMfly:  inc mx
        lda mx
        cmp #150
        bcc PMok
        lda #0
        sta mAct
PMok:
PMnone: lda mAct
        asl                 ; 0/2
        sta ENAM0           ; D1
        lda #2
        sta ENABL           ; ball 常時
        sta WSYNC           ; 4
        ; --- 位置決め: P0（行5）, M0（行6）, BL（行7, 中央固定 X=80）, HMOVE（行8）---
        lda px
        sec
PDv0:   sbc #15
        bcs PDv0
        tay
        lda ZHmove,y
        sta HMP0
        sta RESP0
        sta WSYNC           ; 5
        lda mx
        sec
PDv1:   sbc #15
        bcs PDv1
        tay
        lda ZHmove,y
        sta HMM0
        sta RESM0
        sta WSYNC           ; 6
        lda #80
        sec
PDv2:   sbc #15
        bcs PDv2
        tay
        lda ZHmove,y
        sta $24             ; HMBL
        sta RESBL
        sta WSYNC           ; 7
        sta HMOVE
        ds 12, $EA          ; SLEEP 24（HMOVE 後の HMxx 凍結確保）
        sta HMCLR
        sta WSYNC           ; 8
        ; --- kernel 180 行: py ウィンドウで P0 描画 ---
        lda #0
        sta lineCt
PKer:   sta WSYNC
        lda lineCt          ; 3
        sec                 ; 5
        sbc py              ; 8
        cmp #8              ; 10
        bcs PBlank          ; 12
        tay                 ; 14
        lda ZSprite,y       ; 18
        sta GRP0            ; 21
        jmp PNext           ; 24
PBlank: lda #0              ; 15
        sta GRP0            ; 18
        nop
        nop                 ; 22
        nop                 ; 24（分岐を等長化）
PNext:  inc lineCt          ; 29
        lda lineCt          ; 32
        cmp #180            ; 34
        bne PKer            ; 37
        lda #0
        sta GRP0
        sta ENAM0
        sta ENABL
        sta WSYNC           ; 188
        ; --- 残り 1 行（合計 192）---
        sta WSYNC           ; 192
        lda #0
        sta PF0
        sta CTRLPF
        rts

; （パドルシーンは v1.0.1 で撤去: ROM 内の INPT0 読取を Stella が「パドル用 ROM」と自動判別し、
;   パドルが挿さると INPT4 が常時 High＝fire が効かなくなるため。パドル能力の検証は litmus_paddle に現存。）

; --- S4: グラデーション＋SFX（bank0）---
; per-scanline COLUBK 虹（litmus_color 系）＋ 進入時に kick（Buzz AUDC=15, AUDF=30, AUDV 減衰=Slocum レシピ）。
SceneGrad:
        lda #$F8
        sta $9A             ; 実行センチネル
        lda lastScene
        cmp #3
        beq GNoInit
        lda #3
        sta lastScene
        lda #0
        sta AUDV1
        lda #40
        sta sfxTmr          ; kick レジスタ書込は WSYNC 後（sfxTmr==40 が進入フレームの旗）
        jmp GRun
GNoInit:
        ; 減衰（分岐レス・毎フレーム AUDV0 = sfxTmr>>2。旧 4 フレーム毎更新の +12cy ジッタが
        ; 4 フレーム毎の 263 行の原因だった。包絡線は同一）
        lda sfxTmr
        beq GZero
        dec sfxTmr
GZero:  lda sfxTmr
        lsr
        lsr
        sta AUDV0
GRun:   sta WSYNC           ; 1
        ; 進入フレームのみ: kick 開始（dispatch+init 行が 88cy になるのを避けてここで）
        lda sfxTmr
        cmp #40
        bne GK
        lda #15
        sta AUDC0
        lda #30
        sta AUDF0
        lda #10
        sta AUDV0
GK:
        ; --- 虹 190 行: COLUBK = 行インデックス（луma 巡回）---
        ldy #0
GKer:   sta WSYNC
        tya                 ; 行 → 色（hue が流れる）
        clc
        adc frameCt         ; フレームでスクロール
        sta COLUBK
        iny
        cpy #190
        bne GKer
        lda #0
        sta COLUBK
        sta WSYNC           ; 192
        rts

; --- S5: 手続き生成（bank0）---
; LFSR（litmus_lfsr で数学検証済みの eor #$8E Galois）で行毎に星空 PF を生成。
; worldSeed を行頭で rnd に再ロード→各行 1 ステップ＝フレーム内は決定的（静止画）。
; 64 フレーム毎に worldSeed を 1 ステップ進める＝世界が周期的に組み変わる（Pitfall/DaveC オマージュ）。
SceneProc:
        lda #$E9
        sta $99             ; 実行センチネル
        ; 星空シード: 毎フレーム 1 ステップ＝連続して上へ流れる
        lda worldSeed
        lsr
        bcc RNoEor
        eor #$8E
RNoEor: sta worldSeed
        sta rnd
        lda #$0E
        sta COLUPF          ; 星=白
        lda #0
        sta COLUBK
        sta CTRLPF          ; 星空は repeat
        sta PF0
        sta WSYNC           ; 1
        lda lastScene
        cmp #4
        bne RInit
        jmp RStars
RInit:  ; --- 進入フレーム: 山脈を 1 バイトのシードから生成（以後静止・ROM に絵データ無し）---
        lda #4
        sta lastScene
        lda #$2F
        sta worldSeed       ; 星空の初期世界
        lda #$5A
        sta mrnd            ; 山の世界シード
        lda #0
        sta AUDV0
        sta AUDV1
        sta PF1
        sta PF2
        lda #$FF            ; 最下段 band9 = 地面（全 bit）
        sta mPF0+9
        sta mPF1+9
        sta mPF2+9
        ; band[b] = band[b+1] AND mask: 下が空いた列は上も必ず空く＝山形が必然。
        ; 裾野（band8..3）= r1 OR r2（生存75%, なだらかに痩せる）／山頂（band2..0）= r1 AND r2
        ;（生存25%, ほぼ届かない）→「裾野は広く頂はまれ」の山らしい高さ分布。
        ldx #8
RGenBand:
        sta WSYNC           ; バンド毎に 3 行（PF1/PF2/PF0 を 1 行ずつ＝行予算内）
        jsr MStep
        sta t6
        jsr MStep
        cpx #3
        bcc RH1
        ora t6
        jmp RJ1
RH1:    and t6
RJ1:    and mPF1+1,x
        sta mPF1,x
        sta WSYNC
        jsr MStep
        sta t6
        jsr MStep
        cpx #3
        bcc RH2
        ora t6
        jmp RJ2
RH2:    and t6
RJ2:    and mPF2+1,x
        sta mPF2,x
        sta WSYNC
        jsr MStep
        sta t6
        jsr MStep
        cpx #3
        bcc RH3
        ora t6
        jmp RJ3
RH3:    and t6
RJ3:    and mPF0+1,x
        sta mPF0,x
        sta WSYNC           ; ループ制御は専用行（PF0 行に dex/bpl を同居させると最悪 77cy）
        dex
        bpl RGenBand        ; 9 バンド × 4 行 = 36
        lda #0              ; 上 2 バンドは常に空＝峰の上限（LFSR の連続ステップ相関で
        sta mPF0            ;  特定 bit が生き残り続け、柱が天井まで届くのを防ぐ）
        sta mPF0+1
        sta mPF1
        sta mPF1+1
        sta mPF2
        sta mPF2+1
        ldx #75             ; 星空 111 行ぶんに揃える（36+75=111）
RGenPad:
        sta WSYNC
        dex
        bne RGenPad
        jmp RMtn

        ; --- 星空 111 行: 行毎ペア p=LFSR×2 の AND、描画=今行ペア&前行ペア（生存6.25%）---
        ; ＝固定カラムに頼らず全カラムに出る、まばらな星。たまに p を共有して 2 行星（大きい星）になる。
RStars: lda #0
        sta t7
        sta t6              ; 先頭行は空（パイプライン充填）
        ldy #0
RKer:   sta WSYNC
        lda t6              ; 3
        sta PF1             ; 6   表示窓(28cy)前 ✓
        lsr                 ; 8
        sta PF2             ; 11  表示窓(38.6cy)前 ✓
        sta PF0             ; 14  上位ニブル, 表示窓(22.6cy)前 ✓
        ; 次行の星を行末の空き時間に計算
        lda rnd
        lsr
        bcc RSc
        eor #$8E
RSc:    sta rnd
        sta t6
        lsr
        bcc RSd
        eor #$8E
RSd:    sta rnd
        and t6              ; A = p = a&b（25%）
        ldx t7              ; X = 前行ペア
        sta t7              ; 次行のために保存
        stx t6
        and t6              ; A = p & 前行ペア（6.25%）
        sta t6              ; = 次行の描画値
        iny
        cpy #111
        bne RKer
        ; --- 山脈 80 行: 10 バンド × 8 行（RAM の帯をロードするだけ）---
RMtn:   ldy #0
RMb:    sta WSYNC
        lda #1
        sta CTRLPF          ; 5   reflect=中央対称の山並み（冪等書き）
        lda #$F4
        sta COLUPF          ; 10  山=茶
        lda mPF0,y
        sta PF0             ; 17 ✓(22.6 前)
        lda mPF1,y
        sta PF1             ; 24 ✓(28 前)
        lda mPF2,y
        sta PF2             ; 31 ✓(38.6 前)
        ldx #7
RMw:    sta WSYNC
        dex
        bne RMw
        iny
        cpy #10
        bne RMb
        lda #0              ; 後始末（行 191 の予算内）
        sta PF0
        sta PF1
        sta PF2
        rts                 ; 1+111+80 = 192 WSYNC（旧版の「ディスパッチ行零れ」依存を廃し明示所有）

MStep:  lda mrnd            ; 山生成 LFSR 1 ステップ（A=新値）
        lsr
        bcc MSx
        eor #$8E
MSx:    sta mrnd
        rts

        ; --- HMOVE 表（ページ跨ぎ回避の負インデックス, zone_multiplex と同型）---
        ORG  $0E00
        RORG $FE00
ZHmoveTbl:
        .byte $80,$70,$60,$50,$40,$30,$20,$10,$00,$F0,$E0,$D0,$C0,$B0,$A0,$90
ZHmoveEnd:
ZHmove = ZHmoveEnd - 256

        ; --- 切替ゾーン（bank0 側, $FF00）---
        ORG  $0F00
        RORG $FF00
CallB1Scene:
        lda $FFF9
        ds 6, $EA
        rts                 ; $FF09

        ORG  $0FE0
        RORG $FFE0
        lda $FFF8
        jmp $F000

        ORG  $0FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0

; ================= bank 1: S0 タイトル =================
        ORG  $1000
        RORG $F000
TitleScene:
        lda #$B1
        sta sent1
        ; （音楽 init は HMCLR 行へ移設＝進入フレームの pre 行超過を解消。R1）
        ; スプライト構成（48px: NUSIZ 3コピー, P0=24/P1=+8, VDEL on）
        lda #$03
        sta NUSIZ0
        sta NUSIZ1
        lda #1
        sta VDELP0
        sta VDELP1
        lda #$0E
        sta COLUP0
        sta COLUP1
        lda #0
        sta COLUBK

        ; --- 位置決め: litmus_48px レシピの中央化版（coarse 54 → fine で P0=56/P1=64＝画面中央の48px）---
        sta WSYNC
        ds 18, $EA          ; SLEEP 36（+10cy で coarse +30px）
        sta RESP0
        sta RESP1
        lda #$E0
        sta HMP0            ; P0 右2 → 56
        lda #$F0
        sta HMP1            ; P1 右1 → 64（=P0+8）
        sta WSYNC
        sta HMOVE
        ds 12, $EA          ; SLEEP 24
        sta HMCLR
        ; 音楽: タイトル進入で初期化（この行は ~30cy しか使っていない＝移設先として安全）
        lda lastScene
        beq TMusOk
        lda #0
        sta lastScene
        sta m0idx
        sta m1idx
        lda #1
        sta m0dur
        sta m1dur
TMusOk: sta WSYNC           ; HMOVE 行をここで閉じる（閉じないと計算と合体して 93cy=2 scanline 跨ぎ→263 行）
        ; ここまで 3 行

        ; --- 計算行 1: BCD スコア +1 ＋ ptr hi 設定 ---
        sed
        clc
        lda score0
        adc #1
        sta score0
        lda score1
        adc #0
        sta score1
        lda score2
        adc #0
        sta score2
        cld
        lda #>Digits
        sta p0+1
        sta p1+1
        sta p2+1
        sta p3+1
        sta p4+1
        sta p5+1
        sta WSYNC           ; 計算行 1 終了（3 行目）

        ; --- 計算行 2: digit ptr lo（nibble*16; 上位 nibble は AND $F0 でそのまま）---
        ; 表示順: p0=score2上位 p1=score2下位 p2=score1上位 ... p5=score0下位
        lda score2
        and #$F0
        sta p0
        lda score2
        and #$0F
        asl
        asl
        asl
        asl
        sta p1
        lda score1
        and #$F0
        sta p2
        lda score1
        and #$0F
        asl
        asl
        asl
        asl
        sta p3
        sta WSYNC           ; 計算行 2（4 行目）
        lda score0
        and #$F0
        sta p4
        lda score0
        and #$0F
        asl
        asl
        asl
        asl
        sta p5
        sta WSYNC           ; 計算行 3（5 行目）

        ; --- 上余白: 35 行（ここまで 5 行 → 40 行）---
        ldy #35
TB1:    sta WSYNC
        dey
        bne TB1

        ; --- ロゴ 16 行（中央 P0=56 用の振付: 表示窓 41.3-57.0cy → timed stores 完了 44/47/50/53）---
        ; head で B0..B5 全ロード（tail は row-- と B5 ステージのみ＝76cy 内）
        ldy #15
        sty row
        lda TblR2,y
        sta t5
        sta WSYNC
        jmp LogoHead
LogoHead:                   ; jmp が次行 0-2cy で着地
        lda TblE,y          ; 6   B0
        sta GRP0            ; 9
        lda TblX,y          ; 13  B1
        sta GRP1            ; 16  （B0→P0影）
        lda TblR1,y         ; 20  B2
        sta GRP0            ; 23  （B1→P1影）
        lda TblC,y          ; 27  B3
        ldx TblS,y          ; 31  B4
        ldy t5              ; 34  B5
        cmp t5              ; 37  SLEEP 7
        ds 2, $EA           ; 41
        sta GRP1            ; 44  B3→P1新, B2→P0影
        stx GRP0            ; 47  B4→P0新, B3→P1影
        sty GRP1            ; 50  B5→P1新, B4→P0影
        sta GRP0            ; 53  junk,    B5→P1影
        dec row             ; 58
        bmi LogoDone        ; 60（不成立）
        ldy row             ; 63
        lda TblR2,y         ; 67  次行 B5
        sta t5              ; 70
        sta WSYNC           ; 73 → stall → 次行頭の jmp 相当（フォールスルーで LogoHead へ）
        jmp LogoHead
LogoDone:
        sta WSYNC           ; 出口行を即閉じ（クリアまで同居させると 77cy=予算+1 で行が零れる）
        lda #0
        sta GRP0
        sta GRP1
        sta GRP0            ; 影クリア（次行頭 ~14cy ＝表示窓 41cy より前なのでゴースト無し）

        ; --- 中余白: 29 行（ここまで 40+17 → 86 行）---
        ldy #29
TB2:    sta WSYNC
        dey
        bne TB2

        ; --- スコア 32 行（16 行ぶんの行倍化 digit を stage/draw 交互で）---
        ldy #15
        sty row
ScoreLoop:
        ; stage 行: 頭でスプライト消灯（影まで, cycle 11 までに完了=表示前）→ t0..t5 ← (p0..p5),row
        sta WSYNC
        lda #0              ; 2
        sta GRP0            ; 5
        sta GRP1            ; 8   （P0影←0）
        sta GRP0            ; 11  （P1影←0）
        ldy row             ; 14
        lda (p0),y          ; 19
        sta t0              ; 11
        lda (p1),y          ; 16
        sta t1              ; 19
        lda (p2),y          ; 24
        sta t2              ; 27
        lda (p3),y          ; 32
        sta t3              ; 35
        lda (p4),y          ; 40
        sta t4              ; 43
        lda (p5),y          ; 48
        sta t5              ; 51
        ; draw 行: zp プリロード → timed（litmus_48px6 と同一形）
        sta WSYNC
        lda t0              ; 3
        sta GRP0            ; 6   B0
        lda t1              ; 9
        sta GRP1            ; 12  B1（B0→影）
        lda t2              ; 15
        sta GRP0            ; 18  B2（B1→影）
        lda t3              ; 21
        ldx t4              ; 24
        ldy t5              ; 27
        ds 7, $EA           ; 41  SLEEP 14（中央位置用）
        sta GRP1            ; 44
        stx GRP0            ; 47
        sty GRP1            ; 50
        sta GRP0            ; 53
        ldy row             ; 46
        dey                 ; 48
        sty row             ; 51
        bpl ScoreLoop       ; 54
        sta WSYNC           ; 出口行を即閉じ（クリア同居だと予算+1 で零れる）
        lda #0
        sta GRP0
        sta GRP1
        sta GRP0            ; 影クリア（次行頭・表示窓前）

        ; --- 音楽 tick（2 行: ch0 / ch1）---
        sta WSYNC
        ; ch0（square リード）
        dec m0dur
        bne TM0ok
        ldy m0idx
        lda Song0,y
        cmp #$FF
        bne TM0note
        ldy #0              ; ループ
        lda Song0,y
TM0note:
        pha
        and #$1F
        sta AUDF0
        pla
        lsr
        lsr
        lsr
        lsr
        lsr
        tax
        lda MusTypes,x
        sta AUDC0
        lda #8
        sta AUDV0
        lda Song0+1,y
        sta m0dur
        iny
        iny
        sty m0idx
TM0ok:  sta WSYNC
        ; ch1（bass）
        dec m1dur
        bne TM1ok
        ldy m1idx
        lda Song1,y
        cmp #$FF
        bne TM1note
        ldy #0
        lda Song1,y
TM1note:
        pha
        and #$1F
        sta AUDF1
        pla
        lsr
        lsr
        lsr
        lsr
        lsr
        tax
        lda MusTypes,x
        sta AUDC1
        lda #8
        sta AUDV1
        lda Song1+1,y
        sta m1dur
        iny
        iny
        sty m1idx
TM1ok:
        ; --- 下余白: 70 行（合計 192）---
        ldy #70
TB3:    sta WSYNC
        dey
        bne TB3
        rts

        ; --- 音楽データ（Sequencer-Kit 互換 noteByte: 上位3bit=type idx, 下位5bit=AUDF / 第2バイト=持続フレーム）---
MusTypes:
        .byte 4, 6, 12, 8   ; idx0=square idx1=bass idx2=lead idx3=noise
Song0:  ; リード（square, type idx0）: AUDF 14,11,9,11
        .byte %00001110, 16
        .byte %00001011, 16
        .byte %00001001, 16
        .byte %00001011, 16
        .byte $FF
Song1:  ; ベース（bass, type idx1=%001xxxxx）: AUDF 31,23 をゆっくり
        .byte %00111111, 32
        .byte %00110111, 32
        .byte $FF

; --- フォント（8×8, 各行を2回置いた 16B/グリフ＝行倍化） ---
        ; ロゴ "EXRCSR": B0=E B1=X B2=R B3=C B4=S B5=R
        MAC GLYPH           ; 8 行を倍化して並べる（kernel は row=15→0 の降順で描く＝最下バイトが画面最上段。
        ; よって行は逆順（{8}が先頭）に格納して、画面では {1}=上段 になる）
        .byte {8},{8},{7},{7},{6},{6},{5},{5},{4},{4},{3},{3},{2},{2},{1},{1}
        ENDM

        align 256
TblE:   GLYPH %01111110,%01100000,%01100000,%01111100,%01100000,%01100000,%01111110,%00000000
TblX:   GLYPH %01100110,%01100110,%00111100,%00011000,%00111100,%01100110,%01100110,%00000000
TblR1:  GLYPH %01111100,%01100110,%01100110,%01111100,%01101100,%01100110,%01100110,%00000000
TblC:   GLYPH %00111100,%01100110,%01100000,%01100000,%01100000,%01100110,%00111100,%00000000
TblS:   GLYPH %00111110,%01100000,%01100000,%00111100,%00000110,%00000110,%01111100,%00000000
TblR2:  GLYPH %01111100,%01100110,%01100110,%01111100,%01101100,%01100110,%01100110,%00000000

        ; 数字 0-9（16B ストライド・ページ内に収める）
        align 256
Digits:
        GLYPH %00111100,%01100110,%01101110,%01110110,%01100110,%01100110,%00111100,%00000000 ; 0
        GLYPH %00011000,%00111000,%00011000,%00011000,%00011000,%00011000,%01111110,%00000000 ; 1
        GLYPH %00111100,%01100110,%00000110,%00001100,%00110000,%01100000,%01111110,%00000000 ; 2
        GLYPH %00111100,%01100110,%00000110,%00011100,%00000110,%01100110,%00111100,%00000000 ; 3
        GLYPH %00001100,%00011100,%00111100,%01101100,%01111110,%00001100,%00001100,%00000000 ; 4
        GLYPH %01111110,%01100000,%01111100,%00000110,%00000110,%01100110,%00111100,%00000000 ; 5
        GLYPH %00111100,%01100000,%01111100,%01100110,%01100110,%01100110,%00111100,%00000000 ; 6
        GLYPH %01111110,%00000110,%00001100,%00011000,%00110000,%00110000,%00110000,%00000000 ; 7
        GLYPH %00111100,%01100110,%00111100,%01100110,%01100110,%01100110,%00111100,%00000000 ; 8
        GLYPH %00111100,%01100110,%01100110,%00111110,%00000110,%00000110,%00111100,%00000000 ; 9

        ; --- 切替ゾーン（bank1 側, $FF00）---
        ORG  $1F00
        RORG $FF00
        ds 3, $EA
        jsr TitleScene      ; $FF03
        lda $FFF8           ; $FF06 → bank0

        ORG  $1FE0
        RORG $FFE0
        lda $FFF8
        jmp $F000

        ORG  $1FFC
        RORG $FFFC
        .word $FFE0
        .word $FFE0
