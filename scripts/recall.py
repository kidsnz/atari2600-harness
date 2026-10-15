#!/usr/bin/env python3
"""recall.py -- ask the harness's documents a question, get the passages that answer it.

The knowledge here is about 20,000 lines of documents and 77 technique pages. Nobody reads
that in the middle of writing a kernel, and measured over the first work made after it was
written down, nobody did: it was consulted once, at the plan, and not again in two days --
seven of the problems met afterwards were already answered in it. This is the cheap way in:

    python3 scripts/recall.py "the scene change frame is one scanline too long"
    python3 scripts/recall.py -n 10 "save bytes, the ROM is full"
    python3 scripts/recall.py -work ../roms/<work> "plucked notes, volume per frame"

Every passage comes with the file, the line it starts on, and its own first words (quote
those, not the line number, when you write the reference down). Open the file and read the
whole passage before acting on it: what is printed is the start of it.

How it finds them: every table row, list item and paragraph of docs/**/*.md and CLAUDE.md
is one passage; passages are ranked by BM25 on the words of the question, words in a
passage's heading or bold lead counting double, and the pages written to be acted on
(traps, techniques, the protocol) ahead of the audits written to be read once. A short
table of 2600 synonyms widens the question ("too long" also looks for "scanline"). No
model, no index file: it reads the documents as they are now, in about a second.

Under the passages it lists the commands and MCP tools that SAY they do what was asked -- from
the `// Answers:` line every cmd/*/main.go carries -- or says that none does. That half exists
because the first half cannot do it: the roster describes a tool in its builder's words, and of
twelve tools one work wrote for itself, six had a command here that came near and was not found.

-work DIR also appends the question and what was returned to DIR/build/recall.log, which
is the record of what the knowledge was asked while that work was made.

    python3 scripts/recall.py --detect frame_lines_stable "262x699 263x1"

--detect CHECK is for the moment a scenario check has just failed. It does not search by
likeness: it returns the passages that NAME that check -- first the rows of known-traps.md,
which say what the check catches, then what scenarios.md says the check does, then the rest --
and puts them in the order of what the failure said, if anything was passed after the name. A
check no document names gets told so, and the ordinary search of what the failure said.
-full prints whole passages instead of their first 420 characters.

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
The tools half is scored on scripts/recall_exam_tools.tsv (13 questions frozen before any
command had an Answers: line): 9 of 13 on 2026-10-13. Later the same day cmd/sound was
written for one of the four it missed; that question's key gained the new command and one
question was added, as the file's header says, and it read 11 of 14.
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
            if len(w) > len(end) + 3 and w.endswith(end) and not (end == "s" and w.endswith("ss")):
                w = w[:-len(end)]                             # "presses" -> "press"; "press" stays "press"
                break
        out.append(w)
    return out


def passages(root):
    """(file, line, title, text) for every table row, list item and paragraph."""
    # *.ja.md are local reading copies, git-ignored: a clone has none, and where they exist they
    # answered in place of the page they copy (two of the first four passages for one check).
    files = [f for f in sorted((root / "docs").rglob("*.md")) if not f.name.endswith(".ja.md")] + [root / "CLAUDE.md"]
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


def tools(root):
    """(name, the question it says it answers, the start of its description) for every command
    and every MCP tool. A command's question is the `// Answers:` line of cmd/<name>/main.go --
    kept in the source, next to the thing it describes, and required by check_wiring.py. An MCP
    tool's is the description the server itself gives (the `Description:` of its `mcp.Tool` in
    cmd/harness/*.go), with its entry in CLAUDE.md's roster after it when it has one. Until
    2026-10-13 only the roster was read, and the roster names 27 of the 43 tools in bold:
    read_row and decompose_row, which a work then rebuilt for itself, were not among them."""
    out = []
    for main in sorted((root / "cmd").glob("*/main.go")):
        head = main.read_text(errors="replace").split("\npackage main", 1)[0]
        lines = [l[2:].strip() for l in head.split("\n") if l.startswith("//")]
        answers = " ".join(l[len("Answers:"):].strip() for l in lines if l.startswith("Answers:"))
        doc = " ".join(l for l in lines if l and not l.startswith("Answers:"))
        out.append(("cmd/" + main.parent.name, answers, doc[:700]))
    roster = {}
    claude = root / "CLAUDE.md"
    if claude.exists():
        for m in re.finditer(r"\*\*`([a-z_]+)`\*\* \((.*?)\)(?= /|\n\n|\n\s*\*\*`|$)", claude.read_text(errors="replace"), flags=re.S):
            roster[m.group(1)] = " ".join(m.group(2).split())
    served = {}
    lit = r'"(?:[^"\\]|\\.)*"'
    for src in sorted((root / "cmd" / "harness").glob("*.go")):
        if src.name.endswith("_test.go"):
            continue
        for m in re.finditer(r'mcp\.Tool\{\s*Name:\s*"([a-z_]+)",\s*Description:\s*((?:' + lit + r'\s*\+?\s*)+)', src.read_text(errors="replace")):
            try:
                served[m.group(1)] = "".join(json.loads(x) for x in re.findall(lit, m.group(2)))
            except ValueError:
                served[m.group(1)] = ""
    for name in sorted(set(served) | set(roster)):
        out.append((name, "", (served.get(name, "") + " " + roster.get(name, "")).strip()[:900]))
    return out


def ask_tools(root, question, n=3):
    """The commands and tools whose own words answer the question, best first; [] when none does.
    A tool qualifies only if at least two of the question's words are in its name or its Answers:
    line (or in the opening of an MCP tool's entry) -- otherwise the honest answer is "nothing
    here says it does that", and a list of three unrelated names would hide it."""
    q = set(words(question))
    extra = set(words(" ".join(x for w in re.findall(r"[a-z0-9_$]+", question.lower()) for x in ALSO.get(w, []))))
    items = tools(root)
    docs = [(set(words(name.replace("/", " ").replace("_", " ") + " " + (ans or doc[:240]))), set(words(doc))) for name, ans, doc in items]
    df = {}
    for strong, weak in docs:
        for w in strong | weak:
            df[w] = df.get(w, 0) + 1
    N = len(items)
    scored = []
    for (name, ans, doc), (strong, weak) in zip(items, docs):
        hit = q & strong
        if len(hit) < min(2, len(q)):
            continue
        s = sum(math.log(1 + N / df[w]) * 3 for w in hit) + sum(math.log(1 + N / df[w]) for w in (q & weak) - hit)
        s += 0.4 * sum(math.log(1 + N / df[w]) for w in (extra & (strong | weak)) - q)
        scored.append((s, name, ans or doc[:160]))
    scored.sort(reverse=True)
    return scored[:n]


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

    def scored(self, question):
        """[(score, index)] of every passage that shares a word with the question, best first."""
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
        return scored

    def ask(self, question, n=6):
        out, per_file = [], {}
        for s, i in self.scored(question):
            rel = self.items[i][0]
            if per_file.get(rel, 0) >= 3:                     # no more than three passages of one file
                continue
            per_file[rel] = per_file.get(rel, 0) + 1
            out.append((s, *self.items[i]))
            if len(out) == n:
                break
        return out


DETECT_ORDER = ("known-traps.md", "scenarios.md", "authoring-protocol.md", "techniques/", "cookbook.md", "testing-playbook.md")


def detect(ix, check, said="", n=6):
    """Passages that name a scenario check: what it catches first, then what it does, then the rest."""
    if "_" in check:
        pat = re.compile(r"(?<![A-Za-z0-9_])" + re.escape(check) + r"(?![A-Za-z0-9_])")
    else:                                                     # a check whose name is an ordinary word (`motion`) counts only when quoted as a name
        pat = re.compile(r"[`\"]" + re.escape(check) + r"[`\"]")
    like = dict((i, s) for s, i in ix.scored(said)) if said.strip() else {}
    named = []
    for i, (rel, line, title, text) in enumerate(ix.items):
        if pat.search(text):
            group = next((k for k, frag in enumerate(DETECT_ORDER) if frag in rel), len(DETECT_ORDER))
            named.append((group, -like.get(i, 0.0), i))
    named.sort()
    out, per_file = [], {}
    for group, neg, i in named:
        rel = ix.items[i][0]
        cap = 4 if "known-traps.md" in rel else 2
        if per_file.get(rel, 0) >= cap:
            continue
        per_file[rel] = per_file.get(rel, 0) + 1
        out.append((-neg, *ix.items[i]))
        if len(out) == n:
            break
    return out, len(named)


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
        # --detect: by the check's name, not by likeness; the row that says what it catches comes first
        (root / "docs" / "known-traps.md").write_text(
            "# traps\n\n| trap | what happens | detect |\n|---|---|---|\n"
            "| **a frame of 263 lines** | the picture rolls | runtime: `frame_lines_stable` |\n"
            "| **a frame that is long, stable or not** | words alike, another check | runtime: `ntsc_frame_lines` |\n"
            "| **a sprite that moves** | the word motion is not the check | manual |\n")
        (root / "docs" / "scenarios.md").write_text("# scenarios\n\n- `frame_lines_stable` counts the lines of every frame of a window.\n- `\"motion\"` pins how far an object moves.\n")
        ix = Index(root)
        d, total = detect(ix, "frame_lines_stable", "262x699 263x1")
        ok = ok and total == 2 and [x[1] for x in d] == ["docs/known-traps.md", "docs/scenarios.md"] and "263 lines" in d[0][4]
        m, total = detect(ix, "motion")
        ok = ok and total == 1 and "pins how far" in m[0][4]                  # the row that only uses the word is not returned
        ok = ok and detect(ix, "no_such_check")[1] == 0
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
    tq = Path(__file__).with_name("recall_exam_tools.tsv")
    if tq.exists():
        tqs = [l.rstrip("\n").split("\t") for l in open(tq) if l.strip() and not l.startswith("#")]
        right, rows = 0, []
        for q, want in tqs:
            got = [name.split("/")[-1] for _, name, _ in ask_tools(ROOT, q)]
            ok = (not got) if want == "none" else any(w in got for w in want.split("|"))
            right += ok
            rows.append(f"    {'ok ' if ok else 'NO '} wanted {want:28s} got {', '.join(got) or 'none':34s} {q[:70]}")
        print(f"tools: the right command or tool in the first 3 (or 'none' when none is right) for {right} of {len(tqs)}")
        print("\n".join(rows))
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
    n, work, check, full = 6, None, None, False
    while args and args[0] in ("-n", "-work", "--detect", "-full"):
        if args[0] == "-full":
            full = True
            args = args[1:]
            continue
        if len(args) < 2:
            sys.exit(__doc__)
        if args[0] == "-n":
            n = int(args[1])
        elif args[0] == "--detect":
            check = args[1]
        else:
            work = Path(args[1])
        args = args[2:]
    if not args and not check:
        sys.exit(__doc__)
    question = " ".join(args)
    ix = Index()
    if check:
        hits, total = detect(ix, check, question, n)
        if hits:
            print(f"{total} passage(s) name `{check}`" + (f"; the first {len(hits)}, what it catches first:" if total > len(hits) else ":"))
        else:
            print(f"No passage of the documents names `{check}`." + (" What the failure said, searched the ordinary way:" if question else " Say what the failure said after the name, or ask in your own words."))
            hits = ix.ask(question, n) if question else []
        question = f"{check} fails: {question}".strip()
    else:
        hits = ix.ask(question, n)
        if not hits:
            print("nothing in the documents shares a word with that. Say it another way; if it still finds nothing, this is knowledge the harness does not have -- write it down when you have it.")
    for k, (s, rel, line, title, text) in enumerate(hits, 1):
        title = title[5:] if title.startswith("PAGE ") else title
        print(f"{k}. {rel}:{line}  [{title}]")
        print("   " + (text if full or len(text) <= 420 else text[:420] + " ..."))
    found = ask_tools(ROOT, question)
    print()
    if found:
        print("Commands and tools that say they do this (read the command's own header before writing a new one):")
        for s, name, what in found:
            print(f"   {name} -- {what}")
        if any(not name.startswith("cmd/") for s, name, what in found):
            print("   (a name without cmd/ is an MCP tool; from a script or a shell: python3 scripts/mcp_call.py --describe <tool>)")
    else:
        print("No command or MCP tool here says it does this. If you write one for a work, say in its first lines what was missing here.")
    if work:
        log = work / "build" / "recall.log"
        log.parent.mkdir(parents=True, exist_ok=True)
        with open(log, "a") as f:
            f.write(json.dumps({"at": time.strftime("%Y-%m-%dT%H:%M:%S"), "asked": question,
                                "got": [f"{rel}: {title[5:] if title.startswith('PAGE ') else title}" for _, rel, line, title, text in hits]}, ensure_ascii=False) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
