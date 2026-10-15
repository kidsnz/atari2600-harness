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
                    messages unread (1 of 8) and with the public config missing (gitleaks refuses)

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


def toml_str(s):
    return "'''" + s + "'''"


def build_config(words, tmpdir):
    """A gitleaks config: the public one, plus one rule per work. Written only to tmpdir."""
    lines = ["[extend]", "path = %s" % toml_str(PUBLIC_CONFIG), ""]
    by_work = {}
    for work, word in words or []:
        by_work.setdefault(work, []).append(re.escape(word))
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


def scan(mode, arg, works_dir=None, cwd=REPO):
    """Return (findings as text lines, a note about coverage)."""
    words = private_words(works_dir or WORKS)
    note = ("private words: %d from %s" % (len(words), works_dir or WORKS) if words is not None else
            "private words: none (no works repository at %s) — only the public rules ran"
            % (works_dir or WORKS))
    found = []
    with tempfile.TemporaryDirectory(prefix="leaks.") as tmp:
        cfg = build_config(words, tmp)
        if mode == "staged":
            for f in gitleaks(["git", "--pre-commit", "--staged", "."], cfg, tmp, cwd=cwd):
                found.append(describe(f, "staged"))
        elif mode == "message":
            with open(arg, encoding="utf-8") as fh:
                text = "".join(l for l in fh if not l.startswith("#"))
            for f in gitleaks(["stdin"], cfg, tmp, stdin=text, cwd=cwd):
                found.append(describe(f, "commit message"))
        elif mode == "range":
            for f in gitleaks(["git", "--log-opts", arg, "."], cfg, tmp, cwd=cwd):
                found.append(describe(f, "pushed lines"))
            shas = subprocess.run(["git", "rev-list", arg], cwd=cwd, capture_output=True,
                                  text=True, check=True).stdout.split()
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
        write("see roms/260101_fixture/tools for the measurement")
        git("add", "notes.txt")
        expect("the work's folder name on a staged line", "staged", None, True)
        git("reset", "-q", "--hard")

        msg = os.path.join(tmp, "msg")
        with open(msg, "w") as f:
            f.write("Measured on the %s work\n# a comment line naming 260101_fixture is dropped\n" % word)
        expect("private word in a commit message", "message", msg, True)
        with open(msg, "w") as f:
            f.write("Measured on one of the works\n# 260101_fixture in a comment is not published\n")
        expect("clean message (the word only in a # comment)", "message", msg, False)

        write("plain text")
        git("add", "notes.txt")
        git("commit", "-q", "-m", "named the 260101_fixture work")
        expect("pushed commit whose MESSAGE holds a folder name", "range", base + "..HEAD", True)
        git("commit", "-q", "--amend", "-m", "named nothing")
        expect("pushed commit with a clean message and line", "range", base + "..HEAD", False)
        # Built at run time: written out whole, this fixture is itself the thing the gate stops
        # (it stopped the commit that added it, 2026-10-15).
        write("/" + "Users/someone/Documents/x")
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
