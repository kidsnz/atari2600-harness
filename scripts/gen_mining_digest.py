#!/usr/bin/env python3
"""gen_mining_digest.py — idempotently regenerate docs/mining-digest.md from MINED.csv.

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
    python3 scripts/gen_mining_digest.py  # regenerate docs/mining-digest.md from the existing MINED.csv (idempotent)

Mapping = (1) curated FEED entries for known slugs -> (2) slug keyword inference -> (3) default Reference.
Newly mined threads are auto-classified by (2)/(3). Refine a high-value thread by adding one FEED line.

The output is English only (harness is a public repository; see CLAUDE.md "Language policy").
The mined data is not: MINED.csv titles and the dev-blog note headings are Japanese summaries.
So the thread column takes the forum's own title (index-forum*.csv), and every `feeds` value is
"<section key> / <target>", where the section key is looked up in SECTIONS. A section key missing
from SECTIONS, a SECTIONS heading missing from its document, or any Japanese character left in the
output stops the script with a non-zero exit instead of writing the file.
"""
import csv
import os
import re
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
HARNESS = os.path.normpath(os.path.join(HERE, ".."))
MINED = os.path.normpath(os.path.join(HARNESS, "..", "reference", "atariage", "MINED.csv"))
DOCS = os.path.join(HARNESS, "docs")

# Interlace? sits right after Multiplex because that is the category it was split out of
# (87f6a92): until it was listed here, emit() dropped every thread classified into it.
CATORDER = ["Color", "Sprite", "Text/HUD", "Multiplex", "Interlace?", "Playfield", "Kernel",
            "Bitmap", "3D/Vector", "Audio", "Tools", "Reference", "Pizza Boy"]
CATNAME = {
    "Color": "colour and palette", "Sprite": "sprites and positioning", "Text/HUD": "text, HUD and score",
    "Multiplex": "multiplexing and flicker",
    "Interlace?": "ambiguous: alternate-line multiplexing OR true two-field interlace (read the thread)",
    "Playfield": "playfield and scrolling",
    "Kernel": "kernel budget and optimisation", "Bitmap": "bitmaps and advanced cartridges",
    "3D/Vector": "3D, raycasting and vectors (technique candidate #12)", "Audio": "music, sound effects and speech",
    "Tools": "drawing and sound tools (reference; not direct authoring input)",
    "Reference": "disassemblies and reference games",
    "Pizza Boy": "Pizza Boy / DaveC (ground truth)",
}

# Section keys used by FEED / KEYWORD_RULES / the default -> how the `feeds` column names them.
# The keys are the original (partly Japanese) section labels; the values are what the digest prints,
# plus the harness documents and headings the label stands for. Every (doc, heading) pair is checked
# against the document when the script runs (heading = prefix of a markdown heading line; None = the
# file only), so a renamed heading stops regeneration instead of leaving a dead pointer.
# A key with no refs names something outside docs/ (the umbrella's reference/, or a technique candidate
# number from the umbrella's candidate list) and is printed as is.
SECTIONS = {
    "§色": ("design-principles §Colour", [("docs/design-principles.md", "Colour (most important)")]),
    "§スプライト": ("design-principles §Sprites", [("docs/design-principles.md", "Sprites (P0/P1)")]),
    "§スプライト・位置決め": ("design-principles §Sprites (positioning)",
                          [("docs/design-principles.md", "Sprites (P0/P1)")]),
    "§PF": ("design-principles §Playfield", [("docs/design-principles.md", "Playfield")]),
    "§PF(HUD)": ("design-principles §Playfield (HUD)", [("docs/design-principles.md", "Playfield")]),
    "§多重化": ("design-principles §Multiplexing and flicker",
             [("docs/design-principles.md", "Multiplexing and flicker")]),
    "§作画craft": ("design-principles §Drawing craft", [("docs/design-principles.md", "Drawing craft")]),
    "§カーネル予算": ("resources §Frame budget (kernel cycles per line)",
                 [("docs/resources.md", "Frame budget (settled values)")]),
    "§音": ("techniques/sound-driver", [("docs/techniques/sound-driver.md", None)]),
    "衝突/PF": ("resources §Collision registers, design-principles §Playfield",
              [("docs/resources.md", "Collision registers (CXxx)"), ("docs/design-principles.md", "Playfield")]),
    "高度カートG1": ("capability-gap-audit §G1 (advanced cartridges)",
                 [("docs/capability-gap-audit.md", "G1 — advanced cartridge support")]),
    "⑫": ("technique candidate #12", []),
    "⑫a": ("technique candidate #12a", []),
    "techniques/sound・music-driver": ("techniques/sound-driver, techniques/music-driver",
                                      [("docs/techniques/sound-driver.md", None),
                                       ("docs/techniques/music-driver.md", None)]),
    "techniques/music-driver": ("techniques/music-driver", [("docs/techniques/music-driver.md", None)]),
    # The one row under this key (234209, Doctor Who Berzerk's speech) is sample playback through
    # AUDV, and tia-pcm.md cites that thread; there is no docs/techniques/sound.md.
    "techniques/sound": ("techniques/tia-pcm", [("docs/techniques/tia-pcm.md", None)]),
    # SAM2600 (309689) is formant synthesis with no sample data, which G3 names as NOT covered.
    "G3": ("capability-gap-audit §G3 (digital speech)",
           [("docs/capability-gap-audit.md", "G3 — digital speech")]),
    "reference": ("reference", []),
    "reference/atariage/": ("reference/atariage/", []),
    "reference/disassemblies": ("reference/disassemblies", []),
    "reference/pizza-boy/dissection.ja.md": ("reference/pizza-boy/dissection.ja.md", []),
}

# (1) curated mapping: slug -> (category, feeds)
FEED = {
    "symbolic-color-names": ("Color", "§色 / design.Hue,Luminance,HueName"),
    "rgb-color-values": ("Color", "§色 / palette_stella.go (source of record)"),
    "palettes-compared": ("Color", "§色 / design.WashoutRisk,HueName,GradientSameHue"),
    "5cycle-color-cycling": ("Color", "§色 / low-cycle colour change (doc)"),
    "multiple-colors-per-scanline": ("Color", "§色 / design.MinColorBandWidthPx,CheckColorBands"),
    "multi-colored-sprites": ("Color", "§色 / design.Hue,Luminance"),
    "bg-pf-per-scanline": ("Color", "§作画craft / design.GradientSameHue"),
    "interlaced-multicolored-playfield": ("Color", "§多重化 / design.SameLuminance"),
    "drawing-wizard-sprite": ("Sprite", "§作画craft / 8px silhouette legibility (doc)"),
    "detailed-missile-sprite-drawing-trick": ("Sprite", "§スプライト / missile = a line (doc)"),
    "creative-use-of-the-missile-sprites": ("Sprite", "§スプライト / missile = a line (doc)"),
    "back-to-back-sprite-data": ("Sprite", "§スプライト / pkg/sprite"),
    "horizontal-positioning": ("Sprite", "§スプライト / design.PositionSplit,CoarseIterations"),
    "48px-positioning": ("Sprite", "§スプライト / design.PositionSplit, pkg/sprite.SplitWide"),
    "animated-48px-sprite-routine": ("Sprite", "§スプライト / pkg/sprite.SplitWide, design.WalkFrame"),
    "free-sprites-for-the-taking": ("Sprite", "§スプライト / reusing ready-made GRP data (doc)"),
    "walk-cycle-two-frames": ("Sprite", "§作画craft / design.WalkFrame"),
    "couch-compliant-logo": ("Sprite", "§作画craft / thumbnail legibility, 2:1 (doc/design.PixelAspectRatio)"),
    "hi-res-title-screens": ("Text/HUD", "§PF(HUD) / design.MaxChars"),
    "the-titlescreen-kernel": ("Text/HUD", "§PF(HUD) / design.MaxChars(Text48px)"),
    "32-character-text-display": ("Text/HUD", "§PF(HUD) / design.MaxChars(TextVenetian)"),
    "text-hud-icons": ("Text/HUD", "§PF(HUD) / design.MaxChars"),
    "six-digit-scores-in-the-atari-2600": ("Text/HUD", "§PF(HUD) / pkg/sprite.DigitFont, score6"),
    "title-screen-opinion": ("Text/HUD", "§作画craft / misread glyphs (doc)"),
    "title-to-game-transition": ("Text/HUD", "§カーネル予算 / GameState (doc)"),
    "pf48-title-tool": ("Text/HUD", "§PF(HUD) / 48px title (tool)"),
    "interlacing-multi-sprites": ("Multiplex", "§多重化 / design.NeedsFlicker"),
    "flicker-to-enhance-graphics": ("Multiplex", "§多重化 / design.NeedsFlicker (deliberate flicker)"),
    "asymmetric-reflected-playfield": ("Playfield", "§PF / design.AsymRightWindow,FitsAsymRightWrite"),
    "castlevania-port": ("Playfield", "§PF / design.AsymRightWindow (asymmetric is expensive)"),
    "tile-based-scrolling-engines": ("Playfield", "§PF / design.ScrollScanlinesConstant"),
    "smooth-scrolling-playfield": ("Playfield", "§PF / design.ScrollScanlinesConstant"),
    "vertical-scrolling-questions": ("Playfield", "§PF / design.ScrollScanlinesConstant"),
    "tile-character-graphics-engine": ("Playfield", "§PF / tile-delta engine (technique candidate)"),
    "atari-background-builder": ("Playfield", "§作画craft / design.BackgroundSpec.Feasible"),
    "trees": ("Playfield", "§作画craft / design.BackgroundSpec"),
    "maze-wall-detection": ("Playfield", "衝突/PF / technique candidate #12a: wall detection"),
    "tankmaze": ("Playfield", "衝突/PF / technique candidate #12a: maze (tankmaze source on GitHub)"),
    "oozy-maze-quest": ("Playfield", "衝突/PF / maze game (partly mined)"),
    "illegal-opcodes": ("Kernel", "§カーネル予算 / ISC/ISB (doc, needs a litmus)"),
    "fast-divide-by-seven": ("Kernel", "§カーネル予算 / division optimisation (doc)"),
    "pointer-optimization": ("Kernel", "§カーネル予算 / pointer optimisation (doc)"),
    "modular-kernel": ("Kernel", "§カーネル予算 / design.LineBudget"),
    "wip-battle-pong": ("Kernel", "§カーネル予算 / cyclebound ((ind),Y page crossing +1cy) / PONG capstone"),
    # Mining for the PONG capstone (2026-08-04; includes re-evaluations of old triage REJECTs)
    "two-player-tetris": ("Kernel", "§カーネル予算 / strobe HMOVE in VBLANK (avoids the comb) / CTRLPF SCORE mode"),
    "collision-not-working": ("Sprite", "§スプライト・位置決め / CXCLR required, PositionSprite (÷15, sta.wx)"),
    "ball-help": ("Sprite", "§スプライト・位置決め / a wrong VDELBL setting stretches the ball"),
    "bounce-catch-wip": ("Sprite", "§スプライト・位置決め / ball+missile = 16-clock pad, paddle-read frame"),
    "how-to-set-initial-score-using-this-kernel": ("Text/HUD", "§PF(HUD) / 3-byte BCD carry chain, score6"),
    "score-not-displaying": ("Text/HUD", "§PF(HUD) / GetDigitPointers (48x8), draw order, sharing a bank"),
    "displaying-the-score": ("Text/HUD", "§PF(HUD) / why the score is held in BCD (AND #$0f / LSR x4)"),
    "score-wount-stop-incrementing-at-gameover": ("Text/HUD", "§PF(HUD) / position and velocity names collide (thin)"),
    "newbie-question-about-breakout-code": ("Playfield", "§PF / Breakout's real kernel (a bundle of row pointers + nibble merge)"),
    "breakout-2002-laserbeams-pal": ("Color", "§色 / PAL drops the top and bottom two luminance rows (thin)"),
    "blob-ball-sound-effects": ("Audio", "§音 / no content (ported sound is redesigned; the real source is 250014)"),
    "chimera-kernel-submissions": ("Kernel", "§カーネル予算 / design.LineBudget"),
    "screen-resolution": ("Kernel", "§カーネル予算 / designing the line count (doc)"),
    "andrews-full-colour-bitmap-mode": ("Bitmap", "§カーネル予算 / bitmap48, DPC"),
    "bitmap-minikernel": ("Bitmap", "§カーネル予算 / bitmap48"),
    "bigger-bitmaps-with-dpc": ("Bitmap", "§カーネル予算 / DPC (advanced cartridges, G1)"),
    "harmony-dpc-programming": ("Bitmap", "§カーネル予算 / DPC (advanced cartridges, G1)"),
    "bang-superchip-demo": ("Bitmap", "§カーネル予算 / Superchip (advanced cartridges, G1)"),
    "high-resolution-vector-engine": ("3D/Vector", "⑫ / raycasting/vector"),
    "mode-7-style-graphics": ("3D/Vector", "⑫ / mode7"),
    "plotcube": ("3D/Vector", "⑫ / vector"),
    "ray-casting-engine-demo": ("3D/Vector", "⑫a / raycasting ★ (Joe's demo, must read)"),
    "raycasting-bus-stuffing": ("3D/Vector", "⑫ / raycasting (depends on bus stuffing = out of scope)"),
    "runes-of-moria-3d-first-person": ("3D/Vector", "⑫a / raycasting (DASM source published)"),
    "rampart-3d-kernel-test": ("3D/Vector", "⑫ / 3D kernel"),
    "refhraktor": ("3D/Vector", "⑫ / vector (by DaveC)"),
    "puedo-vector-gfx": ("3D/Vector", "⑫ / pseudo-vector"),
    "tiatracker": ("Audio", "techniques/music-driver / read_audio"),
    "tiatrackerplus": ("Audio", "techniques/music-driver"),
    "tiamat-tia-music-tool": ("Audio", "techniques/music-driver / DaveC's tool"),
    "doctor-who-berzerk-speech": ("Audio", "techniques/sound / AUDV 4bit PCM(doc)"),
    "software-speech-synthesizer": ("Audio", "G3 / formant synthesis (SAM2600), not sample playback; not covered"),
    "graphic-software": ("Tools", "reference / map of drawing tools"),
    "tools-for-graphics-and-sound": ("Tools", "reference / map of tools"),
    "my-atari-vcs-sprite-editor": ("Tools", "reference / sprite editor"),
    "2600-screen-editor": ("Tools", "reference / screen editor (349056)"),
    "sprite-animation-editor": ("Tools", "reference / animation editor"),
    "playerpal-22": ("Tools", "reference / PlayerPal"),
    "playerpal-v20": ("Tools", "reference / PlayerPal"),
    "2600-sprite-creators": ("Tools", "reference / sprite-making tools"),
    "disassembling-2600-games": ("Reference", "reference/disassemblies / cmd/dissect"),
    "bus-stuffing-demos": ("Reference", "高度カートG1 / bus stuffing (out of scope)"),
    "medieval-mayhem": ("Reference", "reference / MM (where the mining started; DaveC)"),
    "pizza-boy-atari-ar": ("Pizza Boy", "reference/pizza-boy/dissection.ja.md / grade-A source of record"),
    "legendary-spear": ("Pizza Boy", "reference / by DaveC"),
    "stocking-stuffer-marble-game": ("Pizza Boy", "reference / by DaveC"),
}

# (2) slug keyword -> (category, feeds) inference (for new threads not in FEED; first match wins, top to bottom)
KEYWORD_RULES = [
    (r"hmove|positioning|reposition|div15|respx?|hmxx", ("Sprite", "§スプライト / design.PositionSplit, CLAUDE.md HMOVE")),
    (r"48-?(px|pixel|bit)|sprite", ("Sprite", "§スプライト / pkg/sprite, design.PositionSplit")),
    (r"color|colour|palette|hue|luminance", ("Color", "§色 / design.Hue/Luminance and others")),
    # ★2026-09-04: `interlac` をこの行から外した。**同義ではない。**
    #   ★2600 の文脈で "interlacing" は【走査線ごとに物を交互に描く】＝多重化の一種を指すことも、
    #   ★★【2フィールドで1枚の絵を作る本来のインターレース】を指すこともある。
    #   ★★★harness で `interlac` に当たる 14 行は【14 行とも前者】——後者は 1 行も無い
    #   （唯一の近い行は design-principles の「2600 は 240p progressive」＝インターレースでは【ない】）。
    #   ★3語を同義にしていたので、撃った人が「持っている」と読める形になっていた。
    #   ★★件数は正しく、結論だけが嘘になる形。★見つけたのは蒸留（helper-2）。
    (r"flicker|multiplex", ("Multiplex", "§多重化 / design.NeedsFlicker")),
    (r"interlac", ("Interlace?", "§多重化 / ★the word has two meanings — read the thread")),
    (r"scroll|playfield|\bpf\b|asym|reflect", ("Playfield", "§PF / design.AsymRightWindow,ScrollScanlinesConstant")),
    (r"title|text|hud|score|font|char", ("Text/HUD", "§PF(HUD) / design.MaxChars")),
    (r"music|sound|audio|speech|digiti|atarivox|tia.?track|sara|savekey", ("Audio", "techniques/sound・music-driver")),
    (r"bitmap|dpc|superchip|sara|bank", ("Bitmap", "§カーネル予算 / bitmap48, advanced cartridges (G1)")),
    (r"raycast|vector|3d|mode-?7|perspective|maze", ("3D/Vector", "⑫")),
    (r"kernel|cycle|timing|opcode|optim|status-register|overscan|vblank|scanline|roll", ("Kernel", "§カーネル予算 / design.LineBudget, CLAUDE.md timing")),
    (r"disassembl|source|reverse", ("Reference", "reference/disassemblies / cmd/dissect")),
    (r"editor|tool|software", ("Tools", "reference / map of tools")),
]


def classify(slug):
    if slug in FEED:
        return FEED[slug]
    for pat, cf in KEYWORD_RULES:
        if re.search(pat, slug):
            return cf
    return ("Reference", "reference/atariage/")


INDEXES = os.path.normpath(os.path.join(HARNESS, "..", "reference", "atariage"))

# Japanese script and the full-width/CJK punctuation that comes with it. Any of these left in the
# output stops the script (the digest lives in a public, English-only repository).
JA = re.compile(r"[　-〿぀-ヿ㐀-䶿一-鿿豈-﫿＀-￯]")


def fail(msg):
    print("gen_mining_digest: " + msg, file=sys.stderr)
    sys.exit(1)


def forum_titles():
    """topic_id -> the forum's own (English) thread title, from reference/atariage/index-forum*.csv."""
    out = {}
    for name in sorted(os.listdir(INDEXES)):
        if not (name.startswith("index-forum") and name.endswith(".csv")):
            continue
        with open(os.path.join(INDEXES, name), encoding="utf-8") as f:
            for r in csv.DictReader(f):
                m = re.search(r"/topic/(\d+)", r.get("topic_url", ""))
                if m and r.get("title"):
                    out.setdefault(m.group(1), r["title"].strip())
    return out


def clean_en(title):
    t = re.split(r"[（(]", title)[0].strip()
    t = re.sub(r"\s*蒸留ノート.*$", "", t).strip()
    return t or title


def humanize(slug):
    return slug.replace("-", " ")


def thread_title(row, forum):
    """English title for a MINED.csv row: MINED's own title when it has no Japanese in it (what the
    digest printed before), else the forum's own title, else the slug with spaces. Returns (title, source)."""
    t = clean_en(row["title"])
    # A Japanese summary that opens with a symbol leaves a fragment that is not a title once cut at
    # the first bracket ("`" from "`(zp,X)` は…", "★★★★★2600 Words" from "★★★★★2600 Words（…）——…").
    whole = t and not JA.search(t) and len(re.findall(r"[A-Za-z]", t)) >= 2 and not t.startswith("★")
    # clean_en cuts at the first bracket of either width, but a half-width "(" belongs to the English
    # title itself ("An (almost) full featured playfield editor" was printed as "An"). Rule: if the
    # MINED title goes on with a half-width "(" right after the cut (optionally one space before it),
    # the cut fell inside the title, and the forum's own title is used when the index has one.
    # Without an index entry the cut title stays (85667 "Medieval Mayhem (SpiceWare, 2006)").
    raw = row["title"]
    cut_inside = whole and re.match(r" ?\(", raw[raw.find(t) + len(t):]) is not None
    forum_t = forum.get(row["topic_id"], "")
    if whole and not (cut_inside and forum_t and not JA.search(forum_t)):
        return t, "MINED.csv"
    t = forum_t
    if t and not JA.search(t):
        return t, "forum index"
    return humanize(row["slug"]), "slug"


def feeds_en(feed):
    """'<section key> / <target>' -> English, through SECTIONS. An unknown key stops the script."""
    key, _, target = feed.partition(" / ")
    if key not in SECTIONS:
        fail("section key %r (feeds value %r) is not in SECTIONS — add it there with its English "
             "name; the digest is not written with a Japanese or unmapped section" % (key, feed))
    name = SECTIONS[key][0]
    return name + (" / " + target if target else "")


def check_sections():
    """Every feeds value the script can produce must map, and every SECTIONS heading must exist."""
    values = [v for _, v in FEED.values()] + [v for _, (_, v) in KEYWORD_RULES] + [classify("")[1]]
    for v in values:
        feeds_en(v)
    for key, (name, refs) in SECTIONS.items():
        for doc, heading in refs:
            path = os.path.join(HARNESS, doc)
            if not os.path.isfile(path):
                fail("SECTIONS[%r] names %s, which does not exist" % (key, doc))
            if heading is None:
                continue
            with open(path, encoding="utf-8") as f:
                heads = [re.sub(r"^#+\s*", "", l).strip() for l in f if l.startswith("#")]
            if not any(h.startswith(heading) for h in heads):
                fail("SECTIONS[%r] names heading %r in %s, which has no such heading" % (key, heading, doc))


# A prose section that sits under the header. It was first written into docs/mining-digest.md
# by hand, which this generator then silently deleted on every regeneration (found
# 2026-10-01 by diffing emit() against the committed file). It lives here now, verbatim,
# so the file stays generated end to end. Edit it HERE. Its counts were taken by hand on
# the date it names; this script does not recompute them.
CLEAN_ROOM_LINE = """
**Where the clean-room line actually falls, counted 2026-09-07.** The rule (`CLAUDE.md` iron rule 5,
memory `feedback-goal-standard`) is that decoding a binary yourself is fair game and *somebody else's
interpretation* is not. Asked of this file rather than assumed:

| source kind | rows |
|---|---|
| machine output — DiStella, `cmd/dissect` | **58** |
| **someone else's ANNOTATED source** | **1** |

The one is `circus-atari-source` ("Commented Source Code for Circus Atari"), and its `feeds` column
routes it to `reference/disassemblies / cmd/dissect` — **the thread is recorded, and the route is our
own disassembler.** So the line holds where it is supposed to, and it holds 58 to 1. ★Worth counting
rather than believing: the rule is easy to state and easy to erode one convenient row at a time, and
until this it had never been asked of the digest. Raised by the mailing-list distillation (helper-1).
"""


def emit(rows, forum):
    by = {c: [] for c in CATORDER}
    sources = {}
    for r in rows:
        cat, feed = classify(r["slug"])
        title, src = thread_title(r, forum)
        sources[src] = sources.get(src, 0) + 1
        by.setdefault(cat, [])
        by[cat].append((r["topic_id"], r["slug"], title.replace("|", "/"), feeds_en(feed), r["url"]))
    # ★2026-08-15: 出力は下で `for c in CATORDER` を回すだけなので、setdefault で受け入れた
    # のに CATORDER に無いカテゴリは **一行も出ず、警告も出ない**＝採掘したスレが黙って消える。
    # 落とす件数を数えて印字する（0でも「0件落とした」と分かるように出す）。
    dropped = {c: len(v) for c, v in by.items() if c not in CATORDER and v}
    if dropped:
        print("WARNING: %d thread(s) in %d category/ies are NOT in CATORDER and will be DROPPED "
              "from the digest: %s" % (sum(dropped.values()), len(dropped),
                                       ", ".join("%s(%d)" % (k, v) for k, v in sorted(dropped.items()))),
              file=sys.stderr)
    L = []
    L.append("# Mining Digest — AtariAge mined threads, indexed to principles & checks\n")
    L.append("Distilled index of the mined AtariAge threads. Raw thread captures stay in the umbrella "
             "`reference/atariage/<topic>/` (provenance); this is the citable, self-contained takeaway map "
             "in the harness. Each row → the design-principles section / `pkg/design` function / technique "
             "candidate it feeds. Generated by `scripts/gen_mining_digest.py` from `reference/atariage/MINED.csv`.\n")
    L.append("Source of record: `reference/atariage/MINED.csv` (%d rows); details in each thread's "
             "`notes.ja.md`. Where MINED.csv's title is a Japanese summary, the thread column gives the forum's "
             "own title (`reference/atariage/index-forum*.csv`) instead.\n"
             % len(rows))
    L.append(CLEAN_ROOM_LINE)
    for c in CATORDER:
        items = sorted(by.get(c, []), key=lambda x: x[1])
        if not items:
            continue
        L.append("\n## %s — %s\n" % (c, CATNAME[c]))
        L.append("| topic | technique | thread | feeds |\n|---|---|---|---|\n")
        for tid, slug, title, feed, url in items:
            L.append("| [%s](%s) | `%s` | %s | %s |\n" % (tid, url, slug, title, feed))
    return "".join(L), sources


BLOGS = os.path.normpath(os.path.join(HARNESS, "..", "reference", "atariage", "blogs"))


def blog_title(name, note_heading):
    """English title for a dev-blog entry: the note's heading up to its first em dash when that has no
    Japanese in it (what the digest printed before), else the page title from entry.txt, else the
    English part of the heading, else the slug with spaces. Returns (title, source)."""
    h = note_heading.split("—")[0].strip()
    if h and not JA.search(h):
        return h, "note heading"
    entry = os.path.join(BLOGS, name, "entry.txt")
    if os.path.isfile(entry):
        with open(entry, encoding="utf-8", errors="replace") as f:
            for line in f:
                if line.startswith("TITLE:"):
                    t = re.sub(r"\s+-\s+AtariAge Forums\s*$", "", line[len("TITLE:"):]).strip()
                    if t and not JA.search(t):
                        return t, "entry.txt"
                    break
    h = note_heading.split("（")[0].strip()
    if h and not JA.search(h):
        return h, "note heading"
    h = h.split("—")[0].strip()
    if h and not JA.search(h):
        return h, "note heading"
    return humanize(name.split("-", 1)[-1]), "slug"


def blog_section():
    """Generate the Dev-blogs index section from reference/atariage/blogs/*/notes.ja.md (source URL + title)."""
    rows = []
    sources = {}
    if os.path.isdir(BLOGS):
        for name in sorted(os.listdir(BLOGS)):
            note = os.path.join(BLOGS, name, "notes.ja.md")
            if not os.path.isfile(note):
                continue
            heading, url = "", "https://forums.atariage.com/blogs/entry/%s/" % name
            with open(note, encoding="utf-8") as f:
                for line in f.readlines()[:6]:
                    if not heading and line.startswith("# "):
                        heading = line[2:].strip()
                    m = re.search(r"(forums\.atariage\.com/blogs/entry/[\w\-]+)", line)
                    if m:
                        url = "https://" + m.group(1) + "/"
            title, src = blog_title(name, heading)
            sources[src] = sources.get(src, 0) + 1
            rows.append((name, title, url))
    if not rows:
        return "", sources
    out = ["\n## Dev-blogs — AtariAge development blogs (SpiceWare's Collect / Stay Frosty series and others)\n",
           "No manual browser fetch needed: CDX listing → Wayback fetch → distillation. Details in "
           "`reference/atariage/blogs/<id>-<slug>/notes.ja.md`. The valuable parts have been absorbed into "
           "design-principles / known-traps / technique-candidates.\n",
           "| entry | title | source |\n|---|---|---|\n"]
    for name, title, url in rows:
        out.append("| `%s` | %s | [link](%s) |\n" % (name.split("-")[0], title.replace("|", "/"), url))
    return "".join(out) + "\n(%d dev-blog entries)\n" % len(rows), sources


def main():
    check_sections()
    with open(MINED, encoding="utf-8") as f:
        rows = list(csv.DictReader(f))
    body, tsrc = emit(rows, forum_titles())
    blog, bsrc = blog_section()
    text = body + blog
    left = [(n, l) for n, l in enumerate(text.splitlines(), 1) if JA.search(l)]
    if left:
        fail("%d output line(s) still carry Japanese; not writing docs/mining-digest.md. First: line %d: %s"
             % (len(left), left[0][0], left[0][1][:160]))
    open(os.path.join(DOCS, "mining-digest.md"), "w", encoding="utf-8").write(text)
    nblog = blog.count("| `") if blog else 0
    print("mining-digest: %d threads + %d dev-blogs -> docs/mining-digest.md" % (len(rows), nblog))
    print("  thread titles from: %s" % ", ".join("%s %d" % kv for kv in sorted(tsrc.items())))
    print("  dev-blog titles from: %s" % ", ".join("%s %d" % kv for kv in sorted(bsrc.items())))


if __name__ == "__main__":
    main()
