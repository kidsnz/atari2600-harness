#!/usr/bin/env python3
"""gen_mining_digest.py — idempotently regenerate docs/mining-digest.md(+ja) from MINED.csv.

Distills the mined AtariAge threads (`reference/atariage/MINED.csv`) into a self-contained
"takeaway index" inside the harness. Maps each thread to the design-principles section /
pkg/design function / technique candidate it feeds. Raw thread captures stay in reference/
as provenance (they do not go in here).

Note: the fetch/ledger-generation scraping tools (old aa_fetch/aa_index/aa_manifest) have been
moved out of the repository into local research tooling (`reference/atariage/_tools/`).
MINED.csv is generated there. This script only distills the already-generated CSV; it does
no fetching.

Usage:
    cd harness
    python3 scripts/gen_mining_digest.py  # regenerate docs/mining-digest.md(+ja) from the existing MINED.csv (idempotent)

Mapping = (1) curated FEED entries for known slugs -> (2) slug keyword inference -> (3) default Reference.
Newly mined threads are auto-classified by (2)/(3). Refine a high-value thread by adding one FEED line.
"""
import csv
import os
import re

HERE = os.path.dirname(os.path.abspath(__file__))
HARNESS = os.path.normpath(os.path.join(HERE, ".."))
MINED = os.path.normpath(os.path.join(HARNESS, "..", "reference", "atariage", "MINED.csv"))
DOCS = os.path.join(HARNESS, "docs")

CATORDER = ["Color", "Sprite", "Text/HUD", "Multiplex", "Playfield", "Kernel",
            "Bitmap", "3D/Vector", "Audio", "Tools", "Reference", "Pizza Boy"]
CATNAME = {
    "Color": "色・パレット", "Sprite": "スプライト・位置決め", "Text/HUD": "テキスト/HUD/スコア",
    "Multiplex": "多重化・フリッカー", "Playfield": "プレイフィールド・スクロール",
    "Kernel": "カーネル予算・最適化", "Bitmap": "ビットマップ・高度カート",
    "3D/Vector": "3D・レイキャスト・ベクタ（技候補⑫）", "Audio": "音楽・効果音・音声",
    "Tools": "作画/音ツール（参照・著述には非直結）", "Reference": "逆アセンブル・参照ゲーム",
    "Pizza Boy": "Pizza Boy / DaveC（ground-truth）",
}

# (1) curated mapping: slug -> (category, feeds)
FEED = {
    "symbolic-color-names": ("Color", "§色 / design.Hue,Luminance,HueName"),
    "rgb-color-values": ("Color", "§色 / palette_stella.go(正本)"),
    "palettes-compared": ("Color", "§色 / design.WashoutRisk,HueName,GradientSameHue"),
    "5cycle-color-cycling": ("Color", "§色 / 省サイクル色替え(doc)"),
    "multiple-colors-per-scanline": ("Color", "§色 / design.MinColorBandWidthPx,CheckColorBands"),
    "multi-colored-sprites": ("Color", "§色 / design.Hue,Luminance"),
    "bg-pf-per-scanline": ("Color", "§作画craft / design.GradientSameHue"),
    "interlaced-multicolored-playfield": ("Color", "§多重化 / design.InterlaceColorsSafe"),
    "drawing-wizard-sprite": ("Sprite", "§作画craft / 8pxシルエット可読性(doc)"),
    "detailed-missile-sprite-drawing-trick": ("Sprite", "§スプライト / missile=線(doc)"),
    "creative-use-of-the-missile-sprites": ("Sprite", "§スプライト / missile=線(doc)"),
    "back-to-back-sprite-data": ("Sprite", "§スプライト / pkg/sprite"),
    "horizontal-positioning": ("Sprite", "§スプライト / design.PositionSplit,CoarseIterations"),
    "48px-positioning": ("Sprite", "§スプライト / design.PositionSplit, pkg/sprite.SplitWide"),
    "animated-48px-sprite-routine": ("Sprite", "§スプライト / pkg/sprite.SplitWide, design.WalkFrame"),
    "free-sprites-for-the-taking": ("Sprite", "§スプライト / 既製GRP流用(doc)"),
    "walk-cycle-two-frames": ("Sprite", "§作画craft / design.WalkFrame"),
    "couch-compliant-logo": ("Sprite", "§作画craft / サムネ可読性,2:1(doc/design.PixelAspectRatio)"),
    "hi-res-title-screens": ("Text/HUD", "§PF(HUD) / design.MaxChars"),
    "the-titlescreen-kernel": ("Text/HUD", "§PF(HUD) / design.MaxChars(Text48px)"),
    "32-character-text-display": ("Text/HUD", "§PF(HUD) / design.MaxChars(TextVenetian)"),
    "text-hud-icons": ("Text/HUD", "§PF(HUD) / design.MaxChars"),
    "six-digit-scores-in-the-atari-2600": ("Text/HUD", "§PF(HUD) / pkg/sprite.DigitFont, score6"),
    "title-screen-opinion": ("Text/HUD", "§作画craft / 字形誤読(doc)"),
    "title-to-game-transition": ("Text/HUD", "§カーネル予算 / GameState(doc)"),
    "pf48-title-tool": ("Text/HUD", "§PF(HUD) / 48pxタイトル(tool)"),
    "interlacing-multi-sprites": ("Multiplex", "§多重化 / design.NeedsFlicker,FitsMultiSprite"),
    "flicker-to-enhance-graphics": ("Multiplex", "§多重化 / design.NeedsFlicker(意図フリッカ)"),
    "asymmetric-reflected-playfield": ("Playfield", "§PF / design.AsymRightWindow,FitsAsymRightWrite"),
    "castlevania-port": ("Playfield", "§PF / design.AsymRightWindow(非対称高コスト)"),
    "tile-based-scrolling-engines": ("Playfield", "§PF / design.ScrollScanlinesConstant"),
    "smooth-scrolling-playfield": ("Playfield", "§PF / design.ScrollScanlinesConstant"),
    "vertical-scrolling-questions": ("Playfield", "§PF / design.ScrollScanlinesConstant"),
    "tile-character-graphics-engine": ("Playfield", "§PF / タイル差分エンジン(technique候補)"),
    "atari-background-builder": ("Playfield", "§作画craft / design.BackgroundSpec.Feasible"),
    "trees": ("Playfield", "§作画craft / design.BackgroundSpec"),
    "maze-wall-detection": ("Playfield", "衝突/PF / ⑫a 壁判定(technique候補)"),
    "tankmaze": ("Playfield", "衝突/PF / ⑫a 迷路(tankmaze GitHubソース有)"),
    "oozy-maze-quest": ("Playfield", "衝突/PF / 迷路ゲーム(部分採掘)"),
    "illegal-opcodes": ("Kernel", "§カーネル予算 / ISC/ISB(doc,要litmus)"),
    "fast-divide-by-seven": ("Kernel", "§カーネル予算 / 除算最適化(doc)"),
    "pointer-optimization": ("Kernel", "§カーネル予算 / ポインタ最適化(doc)"),
    "modular-kernel": ("Kernel", "§カーネル予算 / design.LineBudget"),
    "wip-battle-pong": ("Kernel", "§カーネル予算 / cyclebound((ind),Y ページ境界+1cy) / PONG capstone"),
    # Mining for the PONG capstone (2026-08-04; includes re-evaluations of old triage REJECTs)
    "two-player-tetris": ("Kernel", "§カーネル予算 / HMOVEはVBLANKで撃つ(櫛回避) / CTRLPF SCOREモード"),
    "collision-not-working": ("Sprite", "§スプライト・位置決め / CXCLR必須, PositionSprite(÷15,sta.wx)"),
    "ball-help": ("Sprite", "§スプライト・位置決め / VDELBL誤設定=ボール伸び"),
    "bounce-catch-wip": ("Sprite", "§スプライト・位置決め / ball+missile=16clkパッド, パドル読取の枠"),
    "how-to-set-initial-score-using-this-kernel": ("Text/HUD", "§PF(HUD) / BCD 3バイト桁上げ連鎖, score6"),
    "score-not-displaying": ("Text/HUD", "§PF(HUD) / GetDigitPointers(48x8), 描画順序・バンク同居"),
    "displaying-the-score": ("Text/HUD", "§PF(HUD) / BCDで持つ理由(AND #$0f / LSR×4)"),
    "score-wount-stop-incrementing-at-gameover": ("Text/HUD", "§PF(HUD) / 位置と速度の命名衝突(薄)"),
    "newbie-question-about-breakout-code": ("Playfield", "§PF / Breakout実カーネル(行ポインタ束+ニブル合成)"),
    "breakout-2002-laserbeams-pal": ("Color", "§色 / PALは輝度の上下2行を捨てる(薄)"),
    "blob-ball-sound-effects": ("Audio", "§音 / 内容なし(移植音は再設計・本命は250014)"),
    "chimera-kernel-submissions": ("Kernel", "§カーネル予算 / design.LineBudget"),
    "screen-resolution": ("Kernel", "§カーネル予算 / ライン数設計(doc)"),
    "andrews-full-colour-bitmap-mode": ("Bitmap", "§カーネル予算 / bitmap48, DPC"),
    "bitmap-minikernel": ("Bitmap", "§カーネル予算 / bitmap48"),
    "bigger-bitmaps-with-dpc": ("Bitmap", "§カーネル予算 / DPC(高度カートG1)"),
    "harmony-dpc-programming": ("Bitmap", "§カーネル予算 / DPC(高度カートG1)"),
    "bang-superchip-demo": ("Bitmap", "§カーネル予算 / Superchip(高度カートG1)"),
    "high-resolution-vector-engine": ("3D/Vector", "⑫ raycasting/vector(technique候補)"),
    "mode-7-style-graphics": ("3D/Vector", "⑫ mode7(technique候補)"),
    "plotcube": ("3D/Vector", "⑫ vector(technique候補)"),
    "ray-casting-engine-demo": ("3D/Vector", "⑫a raycasting★(Joe's demo・必読)"),
    "raycasting-bus-stuffing": ("3D/Vector", "⑫ raycasting(bus stuffing依存=スコープ外)"),
    "runes-of-moria-3d-first-person": ("3D/Vector", "⑫a raycasting(DASM公開ソース有)"),
    "rampart-3d-kernel-test": ("3D/Vector", "⑫ 3Dカーネル(technique候補)"),
    "refhraktor": ("3D/Vector", "⑫ vector(DaveC作)"),
    "puedo-vector-gfx": ("3D/Vector", "⑫ 擬似ベクタ(technique候補)"),
    "tiatracker": ("Audio", "techniques/music-driver / read_audio"),
    "tiatrackerplus": ("Audio", "techniques/music-driver"),
    "tiamat-tia-music-tool": ("Audio", "techniques/music-driver(DaveC作ツール)"),
    "doctor-who-berzerk-speech": ("Audio", "techniques/sound / AUDV 4bit PCM(doc)"),
    "software-speech-synthesizer": ("Audio", "techniques/sound / デジタル音声(doc)"),
    "graphic-software": ("Tools", "reference / 作画ツール地図"),
    "tools-for-graphics-and-sound": ("Tools", "reference / ツール地図"),
    "my-atari-vcs-sprite-editor": ("Tools", "reference / スプライトエディタ"),
    "2600-screen-editor": ("Tools", "reference / 画面エディタ(349056)"),
    "sprite-animation-editor": ("Tools", "reference / アニメエディタ"),
    "playerpal-22": ("Tools", "reference / PlayerPal"),
    "playerpal-v20": ("Tools", "reference / PlayerPal"),
    "2600-sprite-creators": ("Tools", "reference / スプライト作成ツール"),
    "disassembling-2600-games": ("Reference", "reference/disassemblies / cmd/dissect"),
    "bus-stuffing-demos": ("Reference", "高度カートG1 / bus stuffing(スコープ外)"),
    "medieval-mayhem": ("Reference", "reference / MM(採掘起点・DaveC)"),
    "pizza-boy-atari-ar": ("Pizza Boy", "reference/pizza-boy/dissection.ja.md(grade-A正本)"),
    "legendary-spear": ("Pizza Boy", "reference / DaveC作"),
    "stocking-stuffer-marble-game": ("Pizza Boy", "reference / DaveC作"),
}

# (2) slug keyword -> (category, feeds) inference (for new threads not in FEED; first match wins, top to bottom)
KEYWORD_RULES = [
    (r"hmove|positioning|reposition|div15|respx?|hmxx", ("Sprite", "§スプライト / design.PositionSplit, CLAUDE.md HMOVE")),
    (r"48-?(px|pixel|bit)|sprite", ("Sprite", "§スプライト / pkg/sprite, design.PositionSplit")),
    (r"color|colour|palette|hue|luminance", ("Color", "§色 / design.Hue/Luminance ほか")),
    (r"flicker|multiplex|interlac", ("Multiplex", "§多重化 / design.NeedsFlicker")),
    (r"scroll|playfield|\bpf\b|asym|reflect", ("Playfield", "§PF / design.AsymRightWindow,ScrollScanlinesConstant")),
    (r"title|text|hud|score|font|char", ("Text/HUD", "§PF(HUD) / design.MaxChars")),
    (r"music|sound|audio|speech|digiti|atarivox|tia.?track|sara|savekey", ("Audio", "techniques/sound・music-driver")),
    (r"bitmap|dpc|superchip|sara|bank", ("Bitmap", "§カーネル予算 / bitmap48・高度カートG1")),
    (r"raycast|vector|3d|mode-?7|perspective|maze", ("3D/Vector", "⑫ technique候補")),
    (r"kernel|cycle|timing|opcode|optim|status-register|overscan|vblank|scanline|roll", ("Kernel", "§カーネル予算 / design.LineBudget, CLAUDE.md timing")),
    (r"disassembl|source|reverse", ("Reference", "reference/disassemblies / cmd/dissect")),
    (r"editor|tool|software", ("Tools", "reference / ツール地図")),
]


def classify(slug):
    if slug in FEED:
        return FEED[slug]
    for pat, cf in KEYWORD_RULES:
        if re.search(pat, slug):
            return cf
    return ("Reference", "reference/atariage/")


def clean_en(title):
    t = re.split(r"[（(]", title)[0].strip()
    t = re.sub(r"\s*蒸留ノート.*$", "", t).strip()
    return t or title


def ja_take(title):
    m = re.search(r"[（(]([^）)]*)[）)]", title)
    return m.group(1).strip() if m else ""


def emit(rows, lang):
    by = {c: [] for c in CATORDER}
    for r in rows:
        cat, feed = classify(r["slug"])
        by.setdefault(cat, [])
        by[cat].append((r["topic_id"], r["slug"], clean_en(r["title"]),
                        ja_take(r["title"]), feed, r["url"]))
    L = []
    if lang == "en":
        L.append("# Mining Digest — AtariAge mined threads, indexed to principles & checks\n")
        L.append("Distilled index of the mined AtariAge threads. Raw thread captures stay in the umbrella "
                 "`reference/atariage/<topic>/` (provenance); this is the citable, self-contained takeaway map "
                 "in the harness. Each row → the design-principles section / `pkg/design` function / technique "
                 "candidate it feeds. Generated by `scripts/gen_mining_digest.py` from `reference/atariage/MINED.csv`.\n")
    else:
        L.append("# 採掘ダイジェスト — AtariAge 採掘スレ索引（原則・pkg/design へ接続）\n")
        L.append("採掘した AtariAge スレの蒸留索引。生スレ保存は傘 `reference/atariage/<topic>/`（来歴）に残し、"
                 "ここは harness 単体で引用できる要点地図。各行→効く design-principles 節 / `pkg/design` 関数 / 技候補。"
                 "`scripts/gen_mining_digest.py` が `reference/atariage/MINED.csv` から生成。\n")
    L.append("出典正本: `reference/atariage/MINED.csv`（%d行）。詳細は各 `notes.ja.md`。\n" % len(rows))
    for c in CATORDER:
        items = sorted(by.get(c, []), key=lambda x: x[1])
        if not items:
            continue
        L.append("\n## %s — %s\n" % (c, CATNAME[c]))
        if lang == "en":
            L.append("| topic | technique | thread | feeds |\n|---|---|---|---|\n")
            for tid, slug, en, ja, feed, url in items:
                L.append("| [%s](%s) | `%s` | %s | %s |\n" % (tid, url, slug, en, feed))
        else:
            L.append("| topic | technique | 要点 | feeds |\n|---|---|---|---|\n")
            for tid, slug, en, ja, feed, url in items:
                L.append("| [%s](%s) | `%s` | %s | %s |\n" % (tid, url, slug, ja or en, feed))
    return "".join(L)


BLOGS = os.path.normpath(os.path.join(HARNESS, "..", "reference", "atariage", "blogs"))


def blog_section():
    """Generate the Dev-blogs index section from reference/atariage/blogs/*/notes.ja.md (source URL + heading)."""
    rows = []
    if os.path.isdir(BLOGS):
        for name in sorted(os.listdir(BLOGS)):
            note = os.path.join(BLOGS, name, "notes.ja.md")
            if not os.path.isfile(note):
                continue
            title, url = "", "https://forums.atariage.com/blogs/entry/%s/" % name
            with open(note, encoding="utf-8") as f:
                for line in f.readlines()[:6]:
                    if not title and line.startswith("# "):
                        title = line[2:].split("—")[0].strip()
                    m = re.search(r"(forums\.atariage\.com/blogs/entry/[\w\-]+)", line)
                    if m:
                        url = "https://" + m.group(1) + "/"
            rows.append((name, title or name, url))
    if not rows:
        return ""
    out = ["\n## Dev-blogs — AtariAge 開発日誌（SpiceWare の Collect/Stay Frosty 連載ほか）\n",
           "ブラウザ手取得不要・CDX 列挙→Wayback 取得→蒸留。詳細 `reference/atariage/blogs/<id>-<slug>/notes.ja.md`。"
           "金脈は design-principles / known-traps / technique-candidates へ吸収済み。\n",
           "| entry | title | source |\n|---|---|---|\n"]
    for name, title, url in rows:
        out.append("| `%s` | %s | [link](%s) |\n" % (name.split("-")[0], title.replace("|", "/"), url))
    return "".join(out) + "\n(%d dev-blog entries)\n" % len(rows)


def main():
    with open(MINED, encoding="utf-8") as f:
        rows = list(csv.DictReader(f))
    blog = blog_section()
    open(os.path.join(DOCS, "mining-digest.md"), "w", encoding="utf-8").write(emit(rows, "en") + blog)
    nblog = blog.count("| `") if blog else 0
    print("mining-digest: %d threads + %d dev-blogs -> docs/mining-digest.md" % (len(rows), nblog))


if __name__ == "__main__":
    main()
