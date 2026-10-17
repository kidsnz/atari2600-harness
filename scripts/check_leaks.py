#!/usr/bin/env python3
"""check_leaks.py — keep what must stay private out of this PUBLIC repository.

WHY IT EXISTS (measured 2026-10-15). This repository is public, and the works it is used on are
not. Nothing stopped a work from leaking in through the text written ABOUT it: an unreleased
title and the commissioning artist's name sat in six tracked lines for two months (first added
2026-08-09), and work folder names in about twenty files. None of them was a file copied out of a
work. Every one was a sentence in a tool header, a doc or the CHANGELOG saying where something had
been learned. So the check looks at every line added, and at commit messages too, because those
are published with the code.

WHAT IS CHECKED
  - gitleaks with its default rules (real credentials: API keys, tokens, private keys), plus the
    public rules in .gitleaks.toml (an absolute home path such as /Users/<name>/, a session's
    temporary folder);
  - the PRIVATE words. Each work keeps its own list in ITS OWN folder, in the private works
    repository beside this one: ../roms/<work>/private-words.txt, one word or phrase per line,
    `#` starts a comment. Every dated work folder name (`260816_xxx`) is added automatically.
    The lists never enter this repository; the rules built from them live in a temporary file.
    Without the works repository (a plain clone, CI) only the public rules run, and it says so.

MODES
  --staged          the lines a commit is about to add (pre-commit hook)
  --message FILE    a commit message (commit-msg hook)
  --range A..B      the commits a push carries: their added lines AND their messages (pre-push)
  --tree            every tracked file as it is now (an audit; reports, never needed for a commit)
  --selftest        a throwaway repository: a private word on an added line, in a message and in a
                    pushed commit must each be stopped, the same text without it must pass, and an
                    old line that already holds the word must not stop an unrelated change. Shown
                    to go red 2026-10-15 with the private words emptied (4 of 8 cases), with push
                    messages unread (1 of 8) and with the public config missing (gitleaks refuses).
                    2026-10-16, after an independent check found variants getting through: red
                    again with words matched literally (2 of 12), file names unread (1 of 12), and
                    the Land-Key allowance removed (1 of 12).
                    2026-10-17, after 261009_next's measurement: red again with the old hand-made
                    list of skipped types, no unpacking and no merge reading (5 of 25)

gitleaks' own `git` mode does not read commit messages (measured 2026-10-15 on a throwaway
repository: a commit whose only private word was in its message reported "no leaks found"), which
is why --range pipes `git log --format=%B` through `gitleaks stdin` as well.

Requires gitleaks (`brew install gitleaks`). A missing gitleaks FAILS: a leak gate that quietly
does nothing is the failure this file exists to prevent.
"""
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.dirname(HERE)
PUBLIC_CONFIG = os.path.join(REPO, ".gitleaks.toml")
WORKS = os.environ.get("LEAK_WORKS_DIR", os.path.join(os.path.dirname(REPO), "roms"))
DATED = re.compile(r"^\d{6}_[A-Za-z0-9._-]+$")


def private_words(works_dir):
    """[(work, word)] from every work folder's name and private-words.txt; None without the repo."""
    if not os.path.isdir(works_dir):
        return None
    out = []
    for name in sorted(os.listdir(works_dir)):
        path = os.path.join(works_dir, name)
        if not os.path.isdir(path) or not DATED.match(name):
            continue
        out.append((name, name))
        lst = os.path.join(path, "private-words.txt")
        if os.path.isfile(lst):
            with open(lst, encoding="utf-8") as f:
                for line in f:
                    word = line.split("#", 1)[0].strip()
                    if word:
                        out.append((name, word))
    return out


SEP = r"[\s_*`~.\-]*"   # what may sit between two parts of a word: spaces, -, _, markdown


def word_regex(word):
    """A private word as a regex that also matches its spacing, joining and emphasis variants:
    "Blue Harbor" matches "blue  harbor", "blue-harbor", "blue_harbor", "BlueHarbor", "**Blue** Harbor".
    Measured need (2026-10-15, the independent check of this gate): each of those got through when
    the words were matched literally. Lookalike letters and zero-width characters still get
    through; they are deliberate, and this gate is for the accidental kind."""
    parts = [p for p in re.split(r"[\s_\-]+|(?<=[a-z])(?=[A-Z])", word) if p]
    return SEP.join(re.escape(p) for p in parts) if parts else re.escape(word)


def toml_str(s):
    return "'''" + s + "'''"


def build_config(words, tmpdir, credentials=True):
    """A gitleaks config: the public one, plus one rule per work. Written only to tmpdir.
    credentials=False leaves out gitleaks' default credential rules, for the files gitleaks itself
    skips by path: those rules are why it skips them (a go.sum module hash reads as an API key,
    measured 2026-10-17), and what is looked for there is the private words and the paths."""
    if credentials:
        lines = ["[extend]", "path = %s" % toml_str(PUBLIC_CONFIG), ""]
    else:
        with open(PUBLIC_CONFIG, encoding="utf-8") as f:
            public = f.read()
        lines = [re.sub(r"(?ms)^\[extend\]\s*\nuseDefault\s*=\s*true\s*\n", "", public), ""]
    by_work = {}
    for work, word in words or []:
        by_work.setdefault(work, []).append(word_regex(word))
    for work, pats in sorted(by_work.items()):
        lines += ["[[rules]]",
                  "id = %s" % toml_str("private-word:" + work),
                  "description = %s" % toml_str("a word kept in the works repository: " + work +
                                                 "/private-words.txt or the folder name"),
                  "regex = %s" % toml_str("(?i)(" + "|".join(sorted(set(pats))) + ")"),
                  ""]
    path = os.path.join(tmpdir, "leaks.toml")
    with open(path, "w", encoding="utf-8") as f:
        f.write("\n".join(lines))
    return path


def gitleaks(args, cfg, tmpdir, stdin=None, cwd=REPO):
    """Run gitleaks; return its findings (a list of dicts). Raises on a run that did not finish."""
    report = os.path.join(tmpdir, "report-%d.json" % len(os.listdir(tmpdir)))
    cmd = ["gitleaks"] + args + ["--config", cfg, "--no-banner", "--log-level", "error",
                                 "--report-format", "json", "--report-path", report,
                                 "--exit-code", "0"]
    r = subprocess.run(cmd, input=stdin, cwd=cwd, capture_output=True, text=True)
    if r.returncode != 0 or not os.path.exists(report):
        raise RuntimeError("gitleaks did not finish (%s): %s" % (" ".join(args), r.stderr.strip()))
    with open(report, encoding="utf-8") as f:
        return json.load(f) or []


def describe(f, where):
    rule = f.get("RuleID", "?")
    loc = f.get("File") or where
    if f.get("StartLine"):
        loc = "%s:%s" % (loc, f["StartLine"])
    if f.get("Commit"):
        loc = "%s (commit %s)" % (loc, f["Commit"][:8])
    return "  %s  %s\n      %s" % (loc, rule, (f.get("Match") or "").strip()[:160])


# gitleaks' default config skips whole files by path (measured 2026-10-16 by the second
# independent check: a private word in an .svg passed every mode). Those files are read here and
# their text passed through `gitleaks stdin`, so the same rules apply. The paths are gitleaks' OWN
# list, copied verbatim from the `[allowlist] paths` of config/gitleaks.toml at the pinned v8.30.1
# (https://github.com/gitleaks/gitleaks/blob/v8.30.1/config/gitleaks.toml, MIT): a hand-made list
# of extensions missed 15 of 26 probed types (measured 2026-10-17 by 261009_next's measurer).
# When the pinned gitleaks version changes, copy the list again.
GITLEAKS_SKIPPED_PATHS = [re.compile(p) for p in (
    r'''gitleaks\.toml''',
    r'''(?i)\.(?:bmp|gif|jpe?g|png|svg|tiff?)$''',
    r'''(?i)\.(?:eot|[ot]tf|woff2?)$''',
    r'''(?i)\.(?:docx?|xlsx?|pdf|bin|socket|vsidx|v2|suo|wsuo|.dll|pdb|exe|gltf)$''',
    r'''go\.(?:mod|sum|work(?:\.sum)?)$''',
    r'''(?:^|/)vendor/modules\.txt$''',
    r'''(?:^|/)vendor/(?:github\.com|golang\.org/x|google\.golang\.org|gopkg\.in|istio\.io|k8s\.io|sigs\.k8s\.io)(?:/.*)?$''',
    r'''(?:^|/)gradlew(?:\.bat)?$''',
    r'''(?:^|/)gradle\.lockfile$''',
    r'''(?:^|/)mvnw(?:\.cmd)?$''',
    r'''(?:^|/)\.mvn/wrapper/MavenWrapperDownloader\.java$''',
    r'''(?:^|/)node_modules(?:/.*)?$''',
    r'''(?:^|/)(?:deno\.lock|npm-shrinkwrap\.json|package-lock\.json|pnpm-lock\.yaml|yarn\.lock)$''',
    r'''(?:^|/)bower_components(?:/.*)?$''',
    r'''(?:^|/)(?:angular|bootstrap|jquery(?:-?ui)?|plotly|swagger-?ui)[a-zA-Z0-9.-]*(?:\.min)?\.js(?:\.map)?$''',
    r'''(?:^|/)javascript\.json$''',
    r'''(?:^|/)(?:Pipfile|poetry)\.lock$''',
    r'''(?i)(?:^|/)(?:v?env|virtualenv)/lib(?:64)?(?:/.*)?$''',
    r'''(?i)(?:^|/)(?:lib(?:64)?/python[23](?:\.\d{1,2})+|python/[23](?:\.\d{1,2})+/lib(?:64)?)(?:/.*)?$''',
    r'''(?i)(?:^|/)[a-z0-9_.]+-[0-9.]+\.dist-info(?:/.+)?$''',
    r'''(?:^|/)vendor/(?:bundle|ruby)(?:/.*?)?$''',
    r'''\.gem$''',
    r'''verification-metadata\.xml''',
    r'''Database.refactorlog''',
    r'''(?:^|/)\.git$''',
)]
# Compressed or differently encoded files are read too: gzip by its magic bytes, UTF-16 by a BOM or
# by NUL bytes, zlib streams inside a PDF or a PNG (measured: all four passed before).
ZLIB_STREAM = re.compile(rb"stream\r?\n(.*?)\r?\nendstream", re.S)


def gitleaks_skips(name):
    return any(r.search(name) for r in GITLEAKS_SKIPPED_PATHS)


def decoded_texts(blob):
    """Every readable form of a file's bytes: as UTF-8, and whatever unpacks out of it."""
    import gzip, zlib, struct
    out = [blob.decode("utf-8", "ignore")]
    if blob[:2] == b"\x1f\x8b":
        try:
            out.append(gzip.decompress(blob).decode("utf-8", "ignore"))
        except (OSError, EOFError, zlib.error):
            pass
    if blob[:2] in (b"\xff\xfe", b"\xfe\xff") or (len(blob) > 8 and blob.count(b"\0") * 4 >= len(blob)):
        for enc in ("utf-16", "utf-16-le", "utf-16-be"):
            try:
                out.append(blob.decode(enc))
                break
            except UnicodeDecodeError:
                continue
    for m in ZLIB_STREAM.finditer(blob):                         # PDF streams
        try:
            out.append(zlib.decompress(m.group(1)).decode("utf-8", "ignore"))
        except zlib.error:
            pass
    if blob[:8] == b"\x89PNG\r\n\x1a\n":                         # PNG text chunks
        i = 8
        while i + 8 <= len(blob):
            n, kind = struct.unpack(">I4s", blob[i:i + 8])
            data = blob[i + 8:i + 8 + n]
            if kind == b"tEXt":
                out.append(data.decode("latin-1"))
            elif kind in (b"zTXt", b"iTXt"):
                key, _, rest = data.partition(b"\0")
                try:
                    out.append(zlib.decompress(rest[1:] if kind == b"zTXt" else rest.split(b"\0", 3)[-1]).decode("utf-8", "ignore"))
                except zlib.error:
                    out.append(rest.decode("utf-8", "ignore"))
            i += 12 + n
    return "\n".join(out)


def changed_names(cwd, *diff_args):
    """Names of added, copied, modified or renamed files, read NUL-separated so a space or a
    non-ASCII letter in a name survives (a plain split on whitespace cut such names, and git quotes
    non-ASCII names otherwise; both found by the third independent check). Modified files count
    too: an existing file edited to hold a word is the same leak as a new one."""
    raw = subprocess.run(["git", "diff", "-z", "--name-only", "--diff-filter=ACMR"] + list(diff_args),
                         cwd=cwd, capture_output=True, check=True).stdout
    return [n for n in raw.decode("utf-8", "surrogateescape").split("\0") if n]


def skipped_text(cwd, rev_paths):
    """Text of the files gitleaks would skip, as (label, text) pairs; binary bytes are dropped."""
    out = []
    for label, spec in rev_paths:
        blob = subprocess.run(["git", "show", spec], cwd=cwd, capture_output=True).stdout
        if gitleaks_skips(label):
            out.append((label, decoded_texts(blob)))
        else:
            # a file gitleaks does read can still hide words it cannot see: compressed or UTF-16
            # (measured: a gzipped .svgz passed). Only what unpacks out of it is added here.
            unpacked = decoded_texts(blob)[len(blob.decode("utf-8", "ignore")):]
            if unpacked.strip():
                out.append((label, unpacked))
    return out


def merge_added_text(cwd, sha):
    """The lines a merge commit adds against its first parent: a conflict resolution lives only
    there, and gitleaks' git mode does not read merge diffs (measured 2026-10-17)."""
    parents = subprocess.run(["git", "rev-list", "--parents", "-n", "1", sha], cwd=cwd,
                             capture_output=True, text=True).stdout.split()[1:]
    if len(parents) < 2:
        return ""
    diff = subprocess.run(["git", "diff", "--no-color", "-U0", parents[0], sha], cwd=cwd,
                          capture_output=True).stdout.decode("utf-8", "surrogateescape")
    return "\n".join(l[1:] for l in diff.splitlines() if l.startswith("+") and not l.startswith("+++"))


def scan(mode, arg, works_dir=None, cwd=REPO):
    """Return (findings as text lines, a note about coverage)."""
    words = private_words(works_dir or WORKS)
    note = ("private words: %d from %s" % (len(words), works_dir or WORKS) if words is not None else
            "private words: none (no works repository at %s) — only the public rules ran"
            % (works_dir or WORKS))
    found = []
    with tempfile.TemporaryDirectory(prefix="leaks.") as tmp:
        cfg = build_config(words, tmp)
        os.makedirs(os.path.join(tmp, "plain"))
        cfg_plain = build_config(words, os.path.join(tmp, "plain"), credentials=False)   # skipped types only
        if mode == "staged":
            for f in gitleaks(["git", "--pre-commit", "--staged", "."], cfg, tmp, cwd=cwd):
                found.append(describe(f, "staged"))
            names = changed_names(cwd, "--cached")
            for f in gitleaks(["stdin"], cfg, tmp, stdin="\n".join(names) + "\n", cwd=cwd):
                found.append(describe(f, "a staged FILE NAME"))
            for label, text in skipped_text(cwd, [(n, ":" + n) for n in names]):
                for f in gitleaks(["stdin"], cfg_plain, tmp, stdin=text, cwd=cwd):
                    found.append(describe(f, label + " (a type gitleaks skips)"))
        elif mode == "message":
            # Every line, '#' ones too: with -F or -m (this project's commit form) git keeps them in
            # the published message. Only the editor's own template comments would be stripped, and
            # scanning those as well costs nothing.
            with open(arg, encoding="utf-8") as fh:
                text = fh.read()
            for f in gitleaks(["stdin"], cfg, tmp, stdin=text, cwd=cwd):
                found.append(describe(f, "commit message"))
        elif mode == "range":
            for f in gitleaks(["git", "--log-opts", arg, "."], cfg, tmp, cwd=cwd):
                found.append(describe(f, "pushed lines"))
            shas = subprocess.run(["git", "rev-list", arg], cwd=cwd, capture_output=True,
                                  text=True, check=True).stdout.split()
            # Every commit of the push, not only its tip: a name renamed away or a file deleted
            # before the tip is still published in the commit that had it.
            for sha in shas:
                # against the FIRST parent: `sha^!` names nothing for a merge (measured 2026-10-17)
                names = changed_names(cwd, sha + "^1", sha) if subprocess.run(
                    ["git", "rev-parse", "-q", "--verify", sha + "^"], cwd=cwd, capture_output=True).returncode == 0 \
                    else [n for n in subprocess.run(["git", "show", "-z", "--name-only", "--format=", sha], cwd=cwd,
                          capture_output=True, check=True).stdout.decode("utf-8", "surrogateescape").split("\0") if n]
                for f in gitleaks(["stdin"], cfg, tmp, stdin="\n".join(names) + "\n", cwd=cwd):
                    found.append(describe(f, "a FILE NAME in %s" % sha[:8]))
                for label, text in skipped_text(cwd, [(n, sha + ":" + n) for n in names]):
                    for f in gitleaks(["stdin"], cfg_plain, tmp, stdin=text, cwd=cwd):
                        found.append(describe(f, "%s in %s (a type gitleaks skips)" % (label, sha[:8])))
                merged = merge_added_text(cwd, sha)
                if merged:
                    for f in gitleaks(["stdin"], cfg, tmp, stdin=merged, cwd=cwd):
                        found.append(describe(f, "a line the merge %s adds" % sha[:8]))
            for sha in shas:
                msg = subprocess.run(["git", "log", "-1", "--format=%B", sha], cwd=cwd,
                                     capture_output=True, text=True, check=True).stdout
                for f in gitleaks(["stdin"], cfg, tmp, stdin=msg, cwd=cwd):
                    found.append(describe(f, "message of %s" % sha[:8]))
        elif mode == "tree":
            files = subprocess.run(["git", "ls-files", "-z"], cwd=cwd, capture_output=True,
                                   check=True).stdout.decode().split("\0")
            snap = os.path.join(tmp, "tree")
            for rel in filter(None, files):
                src = os.path.join(cwd, rel)
                if os.path.islink(src) or not os.path.isfile(src):
                    continue
                dst = os.path.join(snap, rel)
                os.makedirs(os.path.dirname(dst), exist_ok=True)
                shutil.copyfile(src, dst)
            for f in gitleaks(["dir", snap], cfg, tmp, cwd=cwd):
                f["File"] = os.path.relpath(f.get("File", ""), snap)
                found.append(describe(f, "tree"))
            # the files gitleaks skips by path, and anything compressed or UTF-16, read here too
            for label, text in skipped_text(cwd, [(rel, "HEAD:" + rel) for rel in filter(None, files)]):
                for f in gitleaks(["stdin"], cfg_plain, tmp, stdin=text, cwd=cwd):
                    found.append(describe(f, label + " (a type gitleaks skips, or packed content)"))
    return found, note


def selftest():
    """Each mode must stop the word, pass the same text without it, and ignore an old line."""
    word = "Quaxolotl Records"
    fails = []
    ran = []
    with tempfile.TemporaryDirectory(prefix="leaks-selftest.") as tmp:
        works = os.path.join(tmp, "works")
        os.makedirs(os.path.join(works, "260101_fixture"))
        with open(os.path.join(works, "260101_fixture", "private-words.txt"), "w") as f:
            f.write("# a fixture\n%s\n" % word)
        repo = os.path.join(tmp, "repo")
        os.makedirs(repo)
        env = dict(os.environ, GIT_AUTHOR_NAME="t", GIT_AUTHOR_EMAIL="t@t",
                   GIT_COMMITTER_NAME="t", GIT_COMMITTER_EMAIL="t@t")

        def git(*a):
            subprocess.run(["git"] + list(a), cwd=repo, env=env, check=True, capture_output=True)

        def write(text, mode="a"):
            with open(os.path.join(repo, "notes.txt"), mode) as f:
                f.write(text + "\n")

        def expect(label, mode, arg, want):
            got = scan(mode, arg, works_dir=works, cwd=repo)[0]
            ran.append(label)
            if bool(got) != want:
                fails.append("%s: expected %s, got %d finding(s)" % (
                    label, "a finding" if want else "none", len(got)))

        git("init", "-q")
        write("an old line naming %s, from before the rule" % word, "w")
        git("add", "notes.txt")
        git("commit", "-q", "-m", "start")
        base = subprocess.run(["git", "rev-parse", "HEAD"], cwd=repo, capture_output=True,
                              text=True, check=True).stdout.strip()

        write("an unrelated line")
        git("add", "notes.txt")
        expect("old line, unrelated change staged", "staged", None, False)
        write("a new line about the quaxolotl records sleeve")
        git("add", "notes.txt")
        expect("private word on a staged line (any case)", "staged", None, True)
        git("reset", "-q", "--hard")
        write("see the Quaxolotl-Records_sleeve and **Quaxolotl** Records")
        git("add", "notes.txt")
        expect("private word joined by - or _, or with markdown emphasis", "staged", None, True)
        git("reset", "-q", "--hard")
        with open(os.path.join(repo, "quaxolotl_records.txt"), "w") as f:
            f.write("nothing here\n")
        git("add", "quaxolotl_records.txt")
        expect("private word in a FILE NAME only", "staged", None, True)
        git("rm", "-q", "--cached", "quaxolotl_records.txt")
        os.remove(os.path.join(repo, "quaxolotl_records.txt"))
        write("see roms/260101_fixture/tools for the measurement")
        git("add", "notes.txt")
        expect("the work's folder name on a staged line", "staged", None, True)
        git("reset", "-q", "--hard")

        msg = os.path.join(tmp, "msg")
        with open(msg, "w") as f:
            f.write("Measured on the %s work\n# a comment line naming 260101_fixture is dropped\n" % word)
        expect("private word in a commit message", "message", msg, True)
        with open(msg, "w") as f:
            f.write("Measured on one of the works\n# 260101_fixture on a '#' line\n")
        expect("a '#' line of a message is scanned (git -F keeps it)", "message", msg, True)
        with open(msg, "w") as f:
            f.write("Measured on one of the works\n\nLand-Key: " + "c6e8910b427d\n")
        expect("a clean message with a Land-Key line passes", "message", msg, False)
        with open(msg, "w") as f:
            f.write("Measured\n\nLand-Key: " + "c6e8910b427d api_key=" + "Zq8Kp2Lm9Xv4Rt7Wn3Hs6Jd1\n")
        expect("a key on a Land-Key line is still caught", "message", msg, True)

        write("plain text")
        git("add", "notes.txt")
        git("commit", "-q", "-m", "named the 260101_fixture work")
        expect("pushed commit whose MESSAGE holds a folder name", "range", base + "..HEAD", True)
        git("commit", "-q", "--amend", "-m", "named nothing")
        expect("pushed commit with a clean message and line", "range", base + "..HEAD", False)
        with open(os.path.join(repo, "260101_fixture-notes.txt"), "w") as f:
            f.write("nothing\n")
        git("add", "260101_fixture-notes.txt")
        git("commit", "-q", "--no-verify", "-m", "plain")
        expect("a pushed FILE NAME holding a folder name", "range", base + "..HEAD", True)
        git("reset", "-q", "--hard", "HEAD~1")
        with open(os.path.join(repo, "logo.svg"), "w") as f:
            f.write("<svg><text>quaxolotl records</text></svg>\n")
        git("add", "logo.svg")
        expect("a private word inside an .svg (a type gitleaks skips)", "staged", None, True)
        git("commit", "-q", "--no-verify", "-m", "plain")
        expect("the same .svg in a push", "range", base + "..HEAD", True)
        git("reset", "-q", "--hard", "HEAD~1")
        # the types and encodings found getting through on 2026-10-17
        import gzip
        for fname, data, label in (
                ("notes.docx", b"quaxolotl records", "a .docx (on gitleaks' own skip list, not on the old one)"),
                ("package-lock.json", b'{"name": "quaxolotl records"}', "a lock file gitleaks skips by name"),
                ("logo.svgz", gzip.compress(b"<svg>quaxolotl records</svg>"), "a gzipped file"),
                ("u16.svg", "<svg>quaxolotl records</svg>".encode("utf-16"), "a UTF-16 file")):
            with open(os.path.join(repo, fname), "wb") as f:
                f.write(data)
            git("add", fname)
            expect(label, "staged", None, True)
            git("rm", "-q", "--cached", fname)
            os.remove(os.path.join(repo, fname))
        # a word that only a merge's conflict resolution adds
        git("checkout", "-q", "-b", "side")
        write("side line")
        git("commit", "-q", "-am", "side")
        git("checkout", "-q", "-")
        write("main line")
        git("commit", "-q", "-am", "main")
        subprocess.run(["git", "merge", "-q", "side"], cwd=repo, env=env, capture_output=True)
        with open(os.path.join(repo, "notes.txt"), "w") as f:
            f.write("resolved: quaxolotl records\n")
        git("add", "notes.txt")
        git("commit", "-q", "--no-verify", "-m", "merge")
        expect("a word only a merge's resolution adds", "range", base + "..HEAD", True)
        git("reset", "-q", "--hard", base)
        with open(os.path.join(repo, "my logo.svg"), "w") as f:
            f.write("<svg>clean</svg>\n")
        git("add", "my logo.svg")
        git("commit", "-q", "--no-verify", "-m", "plain")
        with open(os.path.join(repo, "my logo.svg"), "w") as f:
            f.write("<svg><text>quaxolotl records</text></svg>\n")
        git("add", "my logo.svg")
        expect("an EXISTING .svg with a space in its name, edited to hold the word", "staged", None, True)
        git("commit", "-q", "--no-verify", "-m", "plain")
        expect("the same edit in a push", "range", base + "..HEAD", True)
        git("reset", "-q", "--hard", base)
        with open(os.path.join(repo, "260101_fixture.txt"), "w") as f:
            f.write("x\n")
        git("add", "260101_fixture.txt")
        git("commit", "-q", "--no-verify", "-m", "plain")
        git("mv", "260101_fixture.txt", "renamed.txt")
        git("commit", "-q", "--no-verify", "-m", "plain")
        expect("a private FILE NAME renamed away before the tip of a push", "range", base + "..HEAD", True)
        git("reset", "-q", "--hard", base)
        msg2 = os.path.join(tmp, "msg2")
        for label, text in (("a /home path", "/" + "ho" + "me/alice/x"), ("a /tmp/claude path", "/" + "tm" + "p/claude-501/x")):
            with open(msg2, "w") as f:
                f.write("see %s\n" % text)
            expect(label + " in a message (gitleaks' own allowlist used to drop it)", "message", msg2, True)
        # Built at run time: written out whole, this fixture is itself the thing the gate stops
        # (it stopped the commit that added it, 2026-10-15).
        write("/" + "Use" + "rs/someone/Documents/x")
        git("add", "notes.txt")
        expect("an absolute home path (public rule)", "staged", None, True)
    return fails, len(ran)


def main(argv):
    if shutil.which("gitleaks") is None:
        print("check_leaks: gitleaks is not installed (brew install gitleaks). Refusing to pass "
              "a leak gate that cannot look.", file=sys.stderr)
        return 1
    if not argv or argv[0] in ("-h", "--help"):
        print(__doc__)
        return 0
    if argv[0] == "--selftest":
        fails, n = selftest()
        for f in fails:
            print("  ✗ " + f)
        if fails:
            print("selftest FAILED — %d of %d case(s)" % (len(fails), n))
            return 1
        print("selftest OK — %d cases: a private word or a work folder name is stopped on an added "
              "line, in a message and in a pushed commit; clean text and old lines pass" % n)
        return 0
    modes = {"--staged": ("staged", 0), "--message": ("message", 1), "--range": ("range", 1),
             "--tree": ("tree", 0)}
    if argv[0] not in modes or len(argv) != 1 + modes[argv[0]][1]:
        print(__doc__, file=sys.stderr)
        return 2
    mode, n = modes[argv[0]]
    if os.environ.get("LEAK_WORKS_DIR") and not os.path.isdir(WORKS):
        # Asked for a works repository that is not there: passing on the public rules alone would
        # look like a pass on the private words too.
        print("✗ check_leaks: LEAK_WORKS_DIR=%s is not a directory." % WORKS)
        return 1
    found, note = scan(mode, argv[1] if n else None)
    if found:
        print("✗ check_leaks: %d finding(s) — this repository is PUBLIC." % len(found))
        print("\n".join(found))
        print("  Write 'one of the works' / 'an in-house work' instead of its name; keep the "
              "detail in the work's own folder.\n  (%s)" % note)
        return 1
    print("check_leaks OK (%s, %s)" % (mode, note))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
