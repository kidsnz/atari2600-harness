#!/usr/bin/env python3
"""verify_claims.py — re-run the command behind a number before believing the number.

WHY THIS IS A SCRIPT AND NOT A RESOLUTION. Twice in one session a subagent reported a
measurement that did not survive being reproduced:

  "proved on 73 of 76 regions"  -- the pf_deadlines check had never been executed at all;
                                   the 76 was a count of regions, and 73 came from a
                                   different question.
  "four points agree"           -- one point had been measured. The other three were
                                   asserted from the shape of the first.

Both were caught the same way: the number was reached again by a second route. Neither was
caught by reading the report carefully, and there is no reason to expect the third one will
be. Care does not scale with the number of claims; a command that either reproduces a
figure or does not, does.

WHAT THIS CATCHES, stated narrowly so the green line is not over-read:

  1. The command was never run. This is the pf_deadlines failure exactly, and it is the
     common one -- a report that describes the run it INTENDED to do.
  2. The number is not in the output. Transcription, rounding, a figure carried over from
     an earlier run, a denominator invented to go with a numerator.
  3. The command no longer produces it. A claim that was true when made and has since
     rotted.

WHAT IT DOES NOT CATCH, and this is the important half: re-execution is not independence.
If the instrument is wrong, the agent and this script run the same wrong instrument and
agree. That is the defect class that cost this project a year (audio.MeasurePeriod) and
fifteen failures in one session, and nothing here touches it. For a DERIVED number --
anything computed from a measurement rather than printed by one -- a second, structurally
different route is still required, and that is a judgement this script cannot make.
`scripts/check_instruments.py` is the mechanical half of that problem; this is not.

FORMAT. A claims file is a JSON list. Each entry:

    {
      "id":      "litmus-pf-late-is-late",       # short, unique
      "claim":   "pf_late misses 3 of its 10 playfield deadlines",
      "command": "go run ./cmd/cyclebound -asm roms/litmus/pf_late.asm -budget 76",
      "expect":  ["\"certified\": true"],        # literal substrings the output MUST hold
      "reject":  ["REFUSED"]                     # optional: substrings it must NOT hold
    }

THE EXAMPLE ABOVE WAS BROKEN FOR A DAY, WHICH IS THE POINT. The first version of this docstring
showed `cyclebound -pf roms/technojacket/cover.bin`. `cmd/cyclebound` has no `-pf` flag (only
`-asm` and `-budget`), it takes a .asm rather than a .bin, `roms/technojacket/cover.bin` does not
exist, and roms/ is a sibling repository that this script's ROOT cannot reach anyway. Four errors
in one line -- inside the script written because reported numbers had never been run. The example
here is now one that runs from the harness root; run it before trusting it, which is the whole
thesis.

`expect` holds LITERALS, not patterns, on purpose. A regex is a place to hide a claim that
matches anything -- `\\d+/\\d+` would have passed the 73/76 report -- and the whole point is
that the exact figure quoted in prose has to appear in the exact output.

An entry with no `command` is reported UNVERIFIABLE and fails the run. It is the state the
two failures above were actually in, and silently skipping it would reproduce them.

    python3 scripts/verify_claims.py claims.json [--timeout SEC] [--keep-going]
    python3 scripts/verify_claims.py --selftest
"""
import json
import os
import subprocess
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def run_one(c, timeout):
    """Return (verdict, detail). Verdict is one of OK / MISMATCH / UNVERIFIABLE / ERROR."""
    cid = c.get("id") or "<no id>"
    if not c.get("claim"):
        return "UNVERIFIABLE", "%s has no `claim` text — a command with nothing to prove" % cid
    cmd = c.get("command")
    if not cmd:
        return "UNVERIFIABLE", ("%s: no `command`. This is the state the '73 of 76' report was "
                                "in — a figure with no way to reach it again." % cid)
    expect = c.get("expect") or []
    if not expect:
        return "UNVERIFIABLE", ("%s: no `expect`. A command that runs and is checked against "
                                "nothing is the vacuous-test defect one level up." % cid)
    try:
        p = subprocess.run(cmd, shell=True, cwd=ROOT, timeout=timeout,
                           capture_output=True, text=True)
    except subprocess.TimeoutExpired:
        return "ERROR", "%s: timed out after %ss" % (cid, timeout)
    except OSError as e:
        return "ERROR", "%s: could not run: %s" % (cid, e)

    out = p.stdout + p.stderr
    missing = [s for s in expect if s not in out]
    present = [s for s in (c.get("reject") or []) if s in out]
    if missing or present:
        lines = []
        if missing:
            lines.append("expected but absent: %s" % ", ".join(repr(s) for s in missing))
        if present:
            lines.append("rejected but present: %s" % ", ".join(repr(s) for s in present))
        tail = "\n".join(out.strip().splitlines()[-12:])
        return "MISMATCH", "%s: %s\n    exit=%d, last lines of output:\n%s" % (
            cid, "; ".join(lines), p.returncode,
            "\n".join("      " + l for l in tail.splitlines()))
    return "OK", "%s: reproduced (%s)" % (cid, ", ".join(repr(s) for s in expect))


def selftest():
    """A detector that silently matches nothing is the defect one level up (check_traps' rule).

    Four cases, one per verdict, so a run that has stopped detecting anything cannot look
    like a run that found nothing wrong.
    """
    cases = [
        ({"id": "t-ok", "claim": "echo prints 73/76", "command": "echo 73/76",
          "expect": ["73/76"]}, "OK"),
        ({"id": "t-mismatch", "claim": "echo prints 73/76", "command": "echo 12/76",
          "expect": ["73/76"]}, "MISMATCH"),
        ({"id": "t-reject", "claim": "no refusal", "command": "echo REFUSED",
          "expect": ["REFUSED"], "reject": ["REFUSED"]}, "MISMATCH"),
        ({"id": "t-unverifiable", "claim": "73 of 76 regions"}, "UNVERIFIABLE"),
    ]
    bad = 0
    for c, want in cases:
        got, detail = run_one(c, 30)
        mark = "ok " if got == want else "BAD"
        if got != want:
            bad += 1
        print("  %s %-16s want %-12s got %s" % (mark, c["id"], want, got))
    if bad:
        print("\nselftest FAILED: %d case(s) did not produce the expected verdict" % bad,
              file=sys.stderr)
        return 1
    print("selftest OK — all four verdicts fire (OK / MISMATCH ×2 / UNVERIFIABLE)")
    return 0


def main():
    argv = sys.argv[1:]
    if "--selftest" in argv:
        return selftest()
    timeout = 300
    if "--timeout" in argv:
        i = argv.index("--timeout")
        timeout = float(argv[i + 1])
        del argv[i:i + 2]
    keep_going = "--keep-going" in argv
    argv = [a for a in argv if not a.startswith("--")]
    if len(argv) != 1:
        print(__doc__.strip().splitlines()[-3], file=sys.stderr)
        return 2

    try:
        claims = json.load(open(argv[0], encoding="utf-8"))
    except (OSError, ValueError) as e:
        print("cannot read claims file: %s" % e, file=sys.stderr)
        return 2
    if not isinstance(claims, list) or not claims:
        print("claims file must be a non-empty JSON list", file=sys.stderr)
        return 2

    tally = {}
    for c in claims:
        verdict, detail = run_one(c, timeout)
        tally[verdict] = tally.get(verdict, 0) + 1
        print("%-13s %s" % (verdict, detail))
        if verdict != "OK" and not keep_going:
            print("\nStopping at the first claim that did not reproduce. Re-run with "
                  "--keep-going to see them all.", file=sys.stderr)
            break

    ok = tally.get("OK", 0)
    print("\n%d/%d claim(s) reproduced%s" % (
        ok, len(claims),
        "" if ok == len(claims) else " — " + ", ".join(
            "%d %s" % (v, k) for k, v in sorted(tally.items()) if k != "OK")))
    if ok != len(claims):
        print("A claim that does not reproduce is not a weaker claim, it is an unmade "
              "measurement. Re-run the work, do not soften the wording.", file=sys.stderr)
        return 1
    print("Reproduced is not independent: if the instrument is wrong, this agrees with it. "
          "For a DERIVED number, reach it again by a different route.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
