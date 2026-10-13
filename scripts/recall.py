#!/usr/bin/env python3
"""recall.py -- ask the harness's documents a question, get the passages that answer it.

The knowledge here is about 20,000 lines of documents and 77 technique pages. Nobody reads
that in the middle of writing a kernel, and measured over the first work made after it was
written down, nobody did: it was consulted once, at the plan, and not again in two days --
seven of the problems met afterwards were already answered in it. This is the cheap way in:

    python3 scripts/recall.py "the scene change frame is one scanline too long"
    python3 scripts/recall.py -n 10 "save bytes, the ROM is full"
    python3 scripts/recall.py -work ../roms/261011_edo "plucked notes, volume per frame"

Every passage comes with the file, the line it starts on, and its own first words (quote
those, not the line number, when you write the reference down). Open the file and read the
whole passage before acting on it: what is printed is the start of it.

How it finds them: every table row, list item and paragraph of docs/**/*.md and CLAUDE.md
is one passage; passages are ranked by BM25 on the words of the question, words in a
passage's heading or bold lead counting double, and the pages written to be acted on
(traps, techniques, the protocol) ahead of the audits written to be read once. A short
table of 2600 synonyms widens the question ("too long" also looks for "scanline"). No
model, no index file: it reads the documents as they are now, in about a second.

-work DIR also appends the question and what was returned to DIR/build/recall.log, which
is the record of what the knowledge was asked while that work was made.

    python3 scripts/recall.py --selftest     known-answer test on a corpus the test builds
    python3 scripts/recall.py --exam [FILE]  how often the right passage comes back, for the
                                             questions in scripts/recall_exam.tsv: a measurement
                                             of this tool against these documents, not a gate

What it scores (2026-10-13). On the 18 questions it was written against -- the problems of one
work, written down before the tool was -- the answer is in the first 5 for 14 and in the first
10 for 16; a plain count of shared words gets 10 and 15; without the synonym table, 12 and 13.
The synonym table was written AFTER those questions, by the same author, so that last pair is
the honest floor. Three questions of a different kind were added the same day ("which command
does X") and found nothing in the first 10; making each entry of CLAUDE.md's tool roster its
own passage did not move them, because the roster says what a tool is and the question says
what one wants. Run --exam for today's numbers; never reword a question to make it pass.
"""
import json, math, os, re, sys, time
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
STOP = set("a an and are as at be by do does for from has have how i in is it its my of on or so that the then this to was what when where which with you your into over after before not no can will would should it's there their them they than too also only more most some any each every one two".split())
# what a question's word also means on this machine; kept short and written down, not learned
ALSO = {
    "long": ["scanline", "line"], "lines": ["scanline"], "frame": ["scanline"], "roll": ["scanline", "sync"], "rolls": ["scanline", "sync"],
    "timer": ["intim", "tim64t"], "wait": ["intim"], "music": ["audf", "audc", "audv", "tune"], "melody": ["tune", "note", "audf"],
    "tune": ["music", "note", "pitch"], "sound": ["audio", "audv", "audc", "audf"], "volume": ["audv", "envelope"], "pitch": ["audf", "tuning"],
    "stack": ["jsr", "depth"], "random": ["lfsr", "rng"], "generator": ["lfsr", "rng"], "bytes": ["byte", "rom", "size"], "full": ["space", "byte"],
    "save": ["saving", "smaller", "shorter"], "playfield": ["pf0", "pf1", "pf2", "pf"], "late": ["deadline", "early", "beam"],
    "sprite": ["player", "grp0", "grp1", "object"], "position": ["respx", "resp0", "hmove", "placement"], "horizontal": ["hmove", "respx"],
    "colour": ["color", "colup0", "colubk"], "color": ["colour", "colup0", "colubk"], "band": ["zone"], "bands": ["zone"],
    "setting": ["state", "configuration"], "settings": ["state", "configuration"], "checked": ["tested", "measured"],
    "plucked": ["pluck", "envelope", "decay"], "golden": ["hash", "update"], "registers": ["register"], "nest": ["depth", "jsr"],
    "skip": ["bit"], "overwrites": ["clobber", "collide"], "key": ["keyfit", "tonic"],
}
WEIGHT = [("known-traps.md", 1.25), ("techniques/", 1.2), ("authoring-protocol.md", 1.15), ("cookbook.md", 1.15), ("scenarios.md", 1.1),
          ("mcp-tools.md", 1.1), ("CLAUDE.md", 1.0), ("capability-gap-audit.md", 0.7), ("fundamentals-audit.md", 0.8), ("mining-digest.md", 0.85)]


def words(text):
    out = []
    for w in re.findall(r"[a-z0-9_$]+", text.lower()):
        if w in STOP or len(w) < 2:
            continue
        for end in ("ing", "ed", "es", "s"):
            if len(w) > len(end) + 3 and w.endswith(end):
                w = w[:-len(end)]
                break
        out.append(w)
    return out


def passages(root):
    """(file, line, title, text) for every table row, list item and paragraph."""
    files = sorted((root / "docs").rglob("*.md")) + [root / "CLAUDE.md"]
    out = []
    for path in files:
        if not path.exists():
            continue
        rel = str(path.relative_to(root))
        heading, block, start = "", [], 0
        first = len(out)                                      # the page's opening passage stands for the page

        def flush():
            nonlocal block
            if block:
                text = " ".join(block).strip()
                if len(text) > 40:
                    lead = re.match(r"[|\-*\s]*\*\*(.+?)\*\*", text)
                    tool = re.match(r"\**((?:`[A-Za-z0-9_./-]+`(?:\s*[+/&,]\s*)?)+)", text)      # a roster entry: `cmd/x` + `cmd/y` (what it is for)
                    title = lead.group(1) if lead else tool.group(1).replace("`", " ") if tool else heading
                    out.append((rel, start, title[:160], text))
            block = []

        for n, line in enumerate(path.read_text(errors="replace").split("\n"), 1):
            s = line.strip()
            if s.startswith("#"):
                flush(); heading = s.lstrip("# ").strip(); continue
            if not s or re.fullmatch(r"\|[\s:|-]+\|?", s):
                flush(); continue
            new = s.startswith("|") or re.match(r"([-*]|\d+\.)\s", s) or re.match(r"\**`(cmd|internal|pkg|scripts)/", s) or re.match(r"\*\*`[a-z_]+`\*\*", s)
            if new or not block:
                flush(); start = n
            block.append(s)
        flush()
        if len(out) > first:
            rel_, line_, title_, text_ = out[first]
            out[first] = (rel_, line_, "PAGE " + path.stem.replace("-", " ") + " " + title_, text_)
    return out


class Index:
    def __init__(self, root=ROOT):
        self.items = passages(root)
        self.toks = []
        self.df = {}
        for rel, line, title, text in self.items:
            t = words(text)
            self.toks.append((t, set(words(title)), set(words(Path(rel).stem.replace("-", " "))), title.startswith("PAGE ")))
            for w in set(t):
                self.df[w] = self.df.get(w, 0) + 1
        self.avg = sum(len(x[0]) for x in self.toks) / max(1, len(self.toks))

    def ask(self, question, n=6):
        q = words(question)
        extra = [x for w in re.findall(r"[a-z0-9_$]+", question.lower()) for x in ALSO.get(w, [])]
        terms = {w: 1.0 for w in q}
        for w in words(" ".join(extra)):
            terms.setdefault(w, 0.4)
        N = len(self.items)
        scored = []
        for i, (t, title, stem, page) in enumerate(self.toks):
            if not t:
                continue
            tf = {}
            for w in t:
                if w in terms:
                    tf[w] = tf.get(w, 0) + 1
            if not tf:
                continue
            s = 0.0
            for w, f in tf.items():
                idf = math.log(1 + (N - self.df[w] + 0.5) / (self.df[w] + 0.5))
                s += terms[w] * idf * (f * 2.4) / (f + 1.4 * (0.4 + 0.6 * len(t) / self.avg)) * (2.0 if w in title else 1.0)
            s *= 1 + 0.25 * (sum(1 for w in q if w in tf) / max(1, len(set(q))))            # how much of the question it covers
            rel = self.items[i][0]
            s *= next((k for frag, k in WEIGHT if frag in rel), 1.0)
            named = sum(1 for w in q if w in stem)            # the question uses the page's own name
            if named:
                s *= 1.0 + 0.2 * named + (0.4 if page else 0.0)
            scored.append((s, i))
        scored.sort(reverse=True)
        out, per_file = [], {}
        for s, i in scored:
            rel = self.items[i][0]
            if per_file.get(rel, 0) >= 3:                     # no more than three passages of one file
                continue
            per_file[rel] = per_file.get(rel, 0) + 1
            out.append((s, *self.items[i]))
            if len(out) == n:
                break
        return out


def selftest():
    import tempfile
    with tempfile.TemporaryDirectory() as d:
        root = Path(d); (root / "docs" / "techniques").mkdir(parents=True)
        (root / "CLAUDE.md").write_text("# entry\n\nNothing about the subject asked for.\n")
        (root / "docs" / "known-traps.md").write_text(
            "# traps\n\n| trap | what happens |\n|---|---|\n"
            "| **a wait for the timer that leaves a line late** | the frame grows by one scanline when the code in front of the wait gets longer |\n"
            "| **a sprite drawn one line low** | VDEL shifts the copy by a line |\n")
        (root / "docs" / "techniques" / "music.md").write_text("# music driver\n\nA plucked note is a volume table that dies away, one entry a frame.\n\n- unrelated: horizontal motion is HMOVE.\n")
        ix = Index(root)
        a = ix.ask("the frame is one scanline too long after the timer wait")
        b = ix.ask("plucked note volume every frame")
        c = ix.ask("zebra quantum marmalade")
        ok = (a and "leaves a line late" in a[0][3] and b and b[0][1].endswith("music.md") and "plucked" in b[0][4] and not c)
        # the same questions against a corpus with the answers taken out must NOT return them
        (root / "docs" / "known-traps.md").write_text("# traps\n\n| trap | what happens |\n|---|---|\n| **a sprite drawn one line low** | VDEL shifts the copy by a line |\n")
        a2 = Index(root).ask("the frame is one scanline too long after the timer wait")
        ok = ok and not any("leaves a line late" in x[3] for x in a2)
    print("recall selftest:", "ok" if ok else "FAILED")
    return 0 if ok else 1


def exam(path):
    qs = [l.rstrip("\n").split("\t") for l in open(path) if l.strip() and not l.startswith("#")]
    ix = Index()

    def rank(hits, files, phrases):
        for k, h in enumerate(hits, 1):
            if any(f in h[1] for f in files.split("|")) and (not phrases or any(p.lower() in h[4].lower() for p in phrases.split("|"))):
                return k
        return None

    def plain(q, n):                                          # the baseline: how many of the question's words a passage has
        qw = set(words(q))
        sc = sorted(((len(qw & set(x[0])), -i, i) for i, x in enumerate(ix.toks)), reverse=True)
        return [(c, *ix.items[i]) for c, _, i in sc[:n] if c]

    saved = dict(ALSO)

    def bare(q, n):
        ALSO.clear()
        r = ix.ask(q, n)
        ALSO.update(saved)
        return r

    print(f"{len(ix.items)} passages from {len({i[0] for i in ix.items})} files; {len(qs)} questions")
    for name, f in (("recall", ix.ask), ("recall without the synonym table", bare), ("a plain count of shared words", plain)):
        ranks = [rank(f(q[0], 10), q[1], q[2] if len(q) > 2 else "") for q in qs]
        print(f"{name:34s} first: {sum(r == 1 for r in ranks):2d}   in the first 5: {sum(bool(r) and r <= 5 for r in ranks):2d}   in the first 10: {sum(bool(r) for r in ranks):2d}   of {len(qs)}")
        if name == "recall":
            for q, r in zip(qs, ranks):
                print(f"    {str(r or '-'):>2s}  {q[0]}")
    return 0


def main():
    args = sys.argv[1:]
    if args == ["--selftest"]:
        return selftest()
    if args and args[0] == "--exam":
        return exam(args[1] if len(args) > 1 else Path(__file__).with_name("recall_exam.tsv"))
    n, work = 6, None
    while args and args[0] in ("-n", "-work"):
        if args[0] == "-n":
            n = int(args[1])
        else:
            work = Path(args[1])
        args = args[2:]
    if not args:
        sys.exit(__doc__)
    question = " ".join(args)
    hits = Index().ask(question, n)
    if not hits:
        print("nothing in the documents shares a word with that. Say it another way; if it still finds nothing, this is knowledge the harness does not have -- write it down when you have it.")
    for k, (s, rel, line, title, text) in enumerate(hits, 1):
        title = title[5:] if title.startswith("PAGE ") else title
        print(f"{k}. {rel}:{line}  [{title}]")
        print("   " + text[:420] + (" ..." if len(text) > 420 else ""))
    if work:
        log = work / "build" / "recall.log"
        log.parent.mkdir(parents=True, exist_ok=True)
        with open(log, "a") as f:
            f.write(json.dumps({"at": time.strftime("%Y-%m-%dT%H:%M:%S"), "asked": question,
                                "got": [f"{rel}: {title[5:] if title.startswith('PAGE ') else title}" for _, rel, line, title, text in hits]}, ensure_ascii=False) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
