#!/usr/bin/env python3
"""check_traps.py — カーネル出荷前の「実機で死ぬ罠」静的リンター。

`docs/known-traps.md` の **static 判定可** な罠を生成/手書きの .asm に対して検出する
（[[feedback-authoring-loop-system]] のプリフライト②・[[project-roadmap-to-pong-capstone]] の Pong 前ゲート）。
runtime 専用の罠（RIOT タイマ wraparound 等）は scenario/`breakif` 側の責務＝ここでは扱わない。

使い方:
    cd harness
    python3 scripts/check_traps.py [file.asm ...]   # 省略時 roms/techniques/*.asm を検査
    python3 scripts/check_traps.py --selftest        # 検出器の自己テスト（bait 文字列で全検出を確認）

判定: ERROR が1つでもあれば exit 1（CI を落とす）。WARN は情報（exit には影響しない）。
誤検出ゼロを最優先（既存 roms/techniques は全て clean）。低確度の罠は WARN 止まり。
"""
import glob
import os
import re
import sys

HARNESS = os.path.normpath(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))


def strip_comment(line):
    # ; 以降を落とす（文字列リテラルは 2600 asm では稀なので簡易処理）。
    i = line.find(";")
    return (line[:i] if i >= 0 else line)



# TIA の「書込専用」レジスタ = $0E(PF1)-$2C(CXCLR)。読み出し側は $00-$0D しか無い
# （Gopher2600 cpubus.go TIAReadRegisters は CXM0P..INPT5 の $00-$0D のみ）。そこを読むと
# レジスタではなくバスの残留値が返る＝エミュレータでは「たまたま動く」典型。
TIA_WRITE_ONLY = {
    "PF1": 0x0E, "PF2": 0x0F, "RESP0": 0x10, "RESP1": 0x11, "RESM0": 0x12, "RESM1": 0x13,
    "RESBL": 0x14, "AUDC0": 0x15, "AUDC1": 0x16, "AUDF0": 0x17, "AUDF1": 0x18, "AUDV0": 0x19,
    "AUDV1": 0x1A, "GRP0": 0x1B, "GRP1": 0x1C, "ENAM0": 0x1D, "ENAM1": 0x1E, "ENABL": 0x1F,
    "HMP0": 0x20, "HMP1": 0x21, "HMM0": 0x22, "HMM1": 0x23, "HMBL": 0x24, "VDELP0": 0x25,
    "VDELP1": 0x26, "VDELBL": 0x27, "RESMP0": 0x28, "RESMP1": 0x29, "HMOVE": 0x2A,
    "HMCLR": 0x2B, "CXCLR": 0x2C,
}
READ_OP = re.compile(
    r"\b(lda|ldx|ldy|bit|cmp|cpx|cpy|adc|sbc|and|ora|eor)\s+(?!#)(\$?[0-9a-zA-Z_]+)", re.I)

def scan_text(asm):
    """asm 文字列を検査して (errors, warns) の (行番号, メッセージ) リストを返す。"""
    errors, warns = [], []
    has_cld = has_cleanstart = False
    lines = asm.splitlines()
    for n, raw in enumerate(lines, 1):
        code = strip_comment(raw)
        low = code.lower()
        if re.search(r"\bcld\b", low):
            has_cld = True
        if re.search(r"clean_start", low):
            has_cleanstart = True
        # 1) 不安定な違法オペコード（個体/温度依存で実機で壊れる）〔known-traps D / 採掘 168616,132496〕
        m = re.search(r"\b(lxa|xaa|ane)\b", low)
        if m:
            errors.append((n, f"unstable illegal opcode `{m.group(1)}` — HW-unreliable (use LAX/SAX/SBX/DCP instead)"))
        if re.search(r"\blax\s+#", low):
            errors.append((n, "`LAX #imm` (immediate) is the unstable LXA form — avoid"))
        # 2) NOP $00 / BIT $00 を skip 用途で（3F/X07 で誤バンク切替）〔known-traps C / 採掘 139089〕
        if re.search(r"\b(nop|bit)\s+\$00\b", low):
            warns.append((n, "`NOP $00`/`BIT $00` can trigger a bankswitch on 3F/X07 carts — use `NOP $80` or a safe address"))
        # 3) スタック衝突域($F8-$FF)への変数割当〔known-traps C / 採掘 302998,301766〕
        m = re.search(r"=\s*\$(f[89a-f])\b", low) or re.search(r"\bequ\s+\$(f[89a-f])\b", low)
        if m:
            warns.append((n, f"variable at $%s — JSR pushes onto the $0100/$00FF stack mirror and can clobber it (keep vars from $80)" % m.group(1).upper()))
        # 5) 書込専用 TIA レジスタの読み出し〔known-traps / Gopher2600 cpubus.TIAReadRegisters=$00-$0D〕
        #    誤検出ゼロを実測: roms/techniques + roms/litmus の 123 本で 0 件（読み系オペコードの
        #    マッチ自体は 509 件あるので、検出器が黙っているのではなく本当に無い）。
        m = READ_OP.search(code)
        if m:
            operand = m.group(2)
            reg = None
            if operand.upper() in TIA_WRITE_ONLY:
                reg = operand.upper()
            elif operand.startswith("$"):
                try:
                    v = int(operand[1:], 16)
                except ValueError:
                    v = -1
                if 0x0E <= v <= 0x2C:
                    reg = next((k for k, a in TIA_WRITE_ONLY.items() if a == v), "$%02X" % v)
            if reg:
                errors.append((n, f"reads {reg}, a WRITE-ONLY TIA register — the TIA answers reads only at "
                                  f"$00-$0D (CXxx/INPTx); this returns bus residue, which an emulator may "
                                  f"make look deterministic"))
    # 4) リセット初期化（CLD も CLEAN_START も無い）〔known-traps D / 採掘 261488,318346〕
    if not (has_cld or has_cleanstart):
        errors.append((0, "no CLD and no CLEAN_START — decimal flag / SP / RAM are undefined at power-up (BCD garbage, rolls)"))
    return errors, warns


def check_file(path):
    with open(path, encoding="utf-8", errors="ignore") as f:
        return scan_text(f.read())


# --- 自己テスト用 bait（各検出器が必ず1つは発火すること）---
BAIT = """
        processor 6502
Start
        lxa #$00          ; unstable illegal opcode
        lax #$ff          ; immediate LAX = unstable
        nop $00           ; bankswitch trap on 3F
flag    = $ff             ; var in stack-collision zone
        lda GRP0          ; read of a write-only TIA register
        ; (intentionally no CLD / CLEAN_START)
"""


def selftest():
    errors, warns = scan_text(BAIT)
    want = ["lxa", "LAX #imm", "bankswitch", "variable at $FF", "no CLD", "WRITE-ONLY TIA register"]
    blob = " ".join(m for _, m in errors + warns)
    missing = [w for w in want if w not in blob]
    if missing:
        print("SELFTEST FAIL — detectors didn't fire for:", missing)
        for ln, m in errors + warns:
            print("  got:", m)
        sys.exit(1)
    print("selftest OK — all %d trap detectors fire on the bait" % len(want))


def main():
    if "--selftest" in sys.argv:
        selftest()
        return
    files = [a for a in sys.argv[1:] if not a.startswith("-")]
    if not files:
        files = sorted(glob.glob(os.path.join(HARNESS, "roms", "techniques", "*.asm")))
    n_err = 0
    for p in files:
        errs, warns = check_file(p)
        rel = os.path.relpath(p, HARNESS)
        for ln, m in errs:
            print(f"ERROR {rel}:{ln}: {m}")
            n_err += 1
        for ln, m in warns:
            print(f"warn  {rel}:{ln}: {m}")
    if n_err:
        print(f"\n{n_err} trap error(s) — see docs/known-traps.md. (rule: feedback-authoring-loop-system)")
        sys.exit(1)
    print("traps OK — no emu-passes/HW-fails static traps in %d asm file(s)." % len(files))


if __name__ == "__main__":
    main()
