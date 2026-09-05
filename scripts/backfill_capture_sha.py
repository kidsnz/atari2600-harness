#!/usr/bin/env python3
"""Bind every Stella capture to the BYTES of the ROM it captured.

Why this exists (2026-09-04). A capture recorded `# rom: roms/litmus/litmus_framephase.bin` and
nothing else. That is a FILE NAME. When a completely different program was written to that path,
the oracle went on grading a month-old capture against the new ROM and reported the mismatch as
**"Gopher2600 and Stella disagree on the write-only TIA registers"**. The two emulators had never
run the same program. The test was not wrong about the numbers; it was wrong about what they meant.

`scripts/stella_oracle.sh` now writes `# binsha256:` at capture time. This script fills it in for
the captures taken before that field existed.

★What a backfilled hash does and does not prove.
    It proves the .bin will not change from now on without the grader noticing.
    It does NOT prove this .bin is the one Stella actually ran back in August.
    The evidence for THAT is separate and must be checked before backfilling: the corpus has to
    grade clean right now. A capture that is already stale would be blessed by this script, which
    is why it refuses to run unless the grader is green, and why the field it writes says
    `backfilled-<date>` rather than `captured`. Anything backfilled is weaker evidence than
    anything captured, and the header says which it is.
"""
import hashlib, os, re, subprocess, sys, datetime

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
CAPDIR = os.path.join(ROOT, "internal/oracle/testdata/stella_tia")
TODAY = datetime.date.today().isoformat()


def sha(path):
    with open(path, "rb") as f:
        return hashlib.sha256(f.read()).hexdigest()


def main():
    force = "--force" in sys.argv
    caps = sorted(p for p in os.listdir(CAPDIR) if p.endswith(".txt"))
    if not caps:
        sys.exit("no captures")

    # ★The precondition. Backfilling a stale capture would freeze the lie in place, so ask the
    #   grader first — but ask it with the sha check DISABLED, because right now it fails on
    #   `unbound` by construction. What must be green is the *disagreement* count.
    if not force:
        r = subprocess.run(["go", "test", "./internal/oracle/", "-run", "TestStellaAgrees",
                            "-count=1"], cwd=ROOT, capture_output=True, text=True)
        bad = [ln for ln in (r.stdout + r.stderr).splitlines()
               if "disagree on the write-only" in ln or "changed since capture" in ln]
        if bad:
            print("✋ refusing to backfill — the grader reports a real disagreement first:")
            for ln in bad:
                print("   " + ln.strip())
            print("   Fix that, or re-run with --force if you know the capture is the right one.")
            sys.exit(1)

    wrote = skipped = missing = 0
    for name in caps:
        p = os.path.join(CAPDIR, name)
        s = open(p, encoding="utf-8", errors="replace").read()
        if re.search(r"(?m)^# binsha256:", s):
            skipped += 1
            continue
        m = re.search(r"(?m)^# rom: *(.+?) *$", s)
        if not m:
            print(f"   ★{name}: no `# rom:` header — left alone")
            missing += 1
            continue
        rom = os.path.join(ROOT, m.group(1))
        if not os.path.exists(rom):
            print(f"   ★{name}: {m.group(1)} does not exist — left alone")
            missing += 1
            continue
        hdr = f"# binsha256: {sha(rom)}\n# binsha256-source: backfilled-{TODAY}\n"
        # place it directly after the `# rom:` line so provenance stays in one block
        s = s[:m.end()] + "\n" + hdr.rstrip("\n") + s[m.end():]
        open(p, "w", encoding="utf-8").write(s)
        wrote += 1

    print(f"backfilled {wrote}, already bound {skipped}, left alone {missing} "
          f"(of {len(caps)} captures)")
    if missing:
        print("★the ones left alone will keep failing the `unbound` check until they are "
              "re-captured or deleted — that is the point, not a bug")


if __name__ == "__main__":
    main()
