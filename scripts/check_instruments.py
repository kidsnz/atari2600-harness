#!/usr/bin/env python3
"""A MEASUREMENT that has never been checked against a known answer is not a measurement.

Why this exists, and it is the most expensive lesson this project has learned twice.

`check_tests.py` already forbids a test that cannot fail. This is the same rule one level
down: a FUNCTION that turns machine data into a number must have been fed an input whose
answer is known independently, and asserted against it. Without that it is not an
instrument, it is an opinion that returns a float.

`audio.MeasurePeriod` went a year without one. It takes the mean interval between
transitions and doubles it, which is the period only when there are exactly two transitions
per cycle -- true of AUDC 4 and 12 and of nothing else the TIA has. On the polynomial
waveforms it returns a clean FRACTION of the period wearing the face of an ordinary number:
4x low for saw, 8x for rumble/pitfall/buzz, 64x for engine. It was "verified" by four spot
checks that happened, by luck, to be three squares and the one polynomial waveform whose
transition count coincides with its period. Every one of them passed. Sweeping the whole
512-point table with it reported 145 pairs off by up to 7200 cents, and every one of those
was the instrument rather than the hardware.

A single calibration case -- a synthetic square of known period in, that period out -- would
have caught it on the day it was written.

WHAT COUNTS AS AN INSTRUMENT here, stated mechanically so the rule cannot drift: an
exported function whose first parameter is a slice of samples (`[]uint8`, `[]float64`,
`[]int`) and which returns at least one numeric value. That is the shape of every reader
this project has been burned by, and it is checkable without understanding the code.

WHAT COUNTS AS CALIBRATION: a test that calls it with an input the test itself CONSTRUCTS
-- not one read back from the emulator -- and asserts against a literal. Feeding an
instrument the machine's own output and checking it agrees with the machine is not
calibration; it is the loop that hid MeasurePeriod for a year.

SECOND AXIS -- THE STATE. One calibration point is not a calibration, and this project
has now paid for that lesson in three separate places in one session: a Video Olympics
reading taken only in the attract screen, a frame measurement taken only at frame 1, and
a pitch table checked at a single AUDF. Each was true where it was measured and wrong
about everything else. An instrument exercised at ONE input cannot be told apart from a
constant that happens to be right there, so a calibration must exercise it at TWO OR MORE
constructed inputs.

Measured before this rule shipped, so it is neither vacuous nor a flood: 9 of the 11
instruments already had two or more, and the 2 that did not were both hiding a live
mutant.

  DominantFreq -- calibrated only at 8192 samples, a power of two. Its bin width is
    sampleRate/nextPow2(len), and the only difference between that and sampleRate/len is
    a length that is NOT a power of two. Production is `cmd/audiospec`, which passes an
    emulator capture: 30,955 samples on a 60-frame run, and never a power of two. The
    mutant `float64(n)` -> `float64(len(samples))` is EXACT at the calibrated state and
    reports 263.7 Hz for 249.1 Hz at the production state -- 100 cents -- while passing
    every test in the package.
  BeatPhase -- calibrated at one click track, with an assertion that fails only inside
    the band 0.08 < ph < 0.42. `return 0` sits outside that band, so a BeatPhase that
    never searches at all passes. Verified by mutation: the whole package stays green.
    (It also has zero production callers -- `cmd/audioingest`'s phase check is
    `census.go`. Recorded here rather than fixed, because deleting it is a separate
    decision.)

HOW "TWO STATES" IS COUNTED, mechanically, so the rule cannot drift: call sites of the
instrument across ALL of its calibrations, where a call site inside a `for ... range`
body counts as many. Two separate single-call tests are two states -- EstimateTempo is
calibrated once on a click track and once on noise, which is the shape this rule wants.

    python3 scripts/check_instruments.py [--list]
"""
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SAMPLE_PARAM = re.compile(r"^\s*\w+\s+\[\](uint8|float64|int)\b")
NUMERIC_RET = re.compile(r"\)\s*(\(?[^)]*\b(int|float64|uint8|int16)\b)")

# Instruments that are exempt, each with the reason. An exemption is a claim in itself and
# is meant to be argued with, not accumulated.
EXEMPT = {
    # Reconstructs its input rather than measuring it: given a period it checks that the
    # samples repeat, which is an equality on the data and has no scale to be wrong about.
    "IsPeriodic": "a predicate on exact repetition, not a scale that can be mis-derived",
}

# Instruments exempt from the SECOND-STATE rule only (they still need a calibration).
# Same idiom as EXEMPT: an exemption is a claim, meant to be argued with, not accumulated.
EXEMPT_STATE = {}


def go_files(root):
    for base, dirs, files in os.walk(root):
        dirs[:] = [d for d in dirs if d not in ("Gopher2600", ".git", "reference", "build", ".claude")]
        for f in files:
            if f.endswith(".go"):
                yield os.path.join(base, f)


def instruments():
    """Exported funcs taking a sample slice first and returning a number."""
    out = {}
    for path in go_files(ROOT):
        if path.endswith("_test.go"):
            continue
        try:
            src = open(path, encoding="utf-8", errors="replace").read()
        except OSError:
            continue
        for m in re.finditer(r"^func ([A-Z]\w*)\(([^)]*)\)([^{]*)\{", src, re.M):
            name, params, ret = m.group(1), m.group(2), m.group(3)
            first = params.split(",")[0] if params else ""
            if not SAMPLE_PARAM.match(" " + first.strip()):
                continue
            if not re.search(r"\b(int|float64|uint8|int16)\b", ret):
                continue
            out[name] = os.path.relpath(path, ROOT)
    return out


def calibrations():
    """Test functions that construct their own input and assert against a literal.

    "Constructs its own input" is approximated by: the test body does NOT call LoadROM.
    A test that loads a cartridge is measuring the machine, and an instrument agreeing
    with the machine it was derived from proves consistency, not correctness.
    """
    calls, bodies = {}, {}
    for path in go_files(ROOT):
        if not path.endswith("_test.go"):
            continue
        try:
            src = open(path, encoding="utf-8", errors="replace").read()
        except OSError:
            continue
        for m in re.finditer(r"^func (Test\w+)\(t \*testing\.T\) \{", src, re.M):
            start = m.end()
            depth, i = 1, start
            while i < len(src) and depth:
                if src[i] == "{":
                    depth += 1
                elif src[i] == "}":
                    depth -= 1
                i += 1
            body = src[start:i]
            if "LoadROM" in body:
                continue                      # measures the machine, not the instrument
            if not re.search(r"t\.(Error|Fatal)", body):
                continue                      # check_tests.py owns this, but be safe
            where = "%s:%s" % (os.path.relpath(path, ROOT), m.group(1))
            bodies[where] = body
            for fn in set(re.findall(r"\b([A-Z]\w*)\(", body)):
                calls.setdefault(fn, []).append(where)
    return calls, bodies


def range_loop_spans(body):
    """Byte spans of every `for ... range ...{ ... }` block body in `body`.

    The block's opening brace is found by scanning, not by regex. The obvious pattern
    (`for[^{]*range[^{]*\\{`) cannot see a composite literal in the range expression, and
    the first table-driven calibration written against this rule was exactly that:

        for _, offSec := range []float64{0, 0.05, 0.125} {

    -- where the first `{` belongs to `[]float64`, not to the loop. The gate reported that
    test as single-state and demanded a second state it already had. Scanning forward and
    counting bracket depth gets the right brace regardless of what is in the header.
    """
    spans = []
    for m in re.finditer(r"\bfor\b[^\n]*?\brange\b", body):
        eol = body.find("\n", m.end())
        if eol < 0:
            eol = len(body)
        header = body[m.end():eol]
        # The block's brace is the LAST `{` on the header line. A composite literal in the
        # range expression always closes before it, so nothing else can be last -- whereas
        # the FIRST `{` is the literal's whenever one is present, which is what the two
        # earlier versions of this function each got wrong in a different way.
        brace = header.rfind("{")
        if brace < 0:
            continue                    # a header spilling over lines: not handled, skipped
        start, depth = m.end() + brace + 1, 1
        j = start
        while j < len(body) and depth:
            if body[j] == "{":
                depth += 1
            elif body[j] == "}":
                depth -= 1
            j += 1
        spans.append((start, j))
    return spans


def states(name, where, bodies):
    """How many distinct constructed inputs the calibrations feed this instrument.

    A call site inside a `for ... range` body is counted as 2 ("many") rather than 1: the
    table it iterates is the multi-state case this rule is asking for, and counting the
    rows would mean parsing Go literals. Capped contributions, not exact arithmetic --
    the question here is only "one, or more than one".
    """
    total = 0
    for w in where:
        body = bodies.get(w, "")
        spans = range_loop_spans(body)
        for m in re.finditer(r"\b%s\(" % re.escape(name), body):
            total += 2 if any(s <= m.start() < e for s, e in spans) else 1
    return total


def selftest():
    """Prove the state counter still discriminates, on snippets with known answers.

    This gate's state axis has now been written three times: the first two versions each
    mis-parsed a `for ... range` header containing a composite literal, and both reported a
    four-case table-driven calibration as single-state. Neither was caught by running the
    gate -- it printed a plausible complaint about a real function. A counter that silently
    stops finding loops turns this whole axis into "call it twice on one line", which is the
    defect one level up, so the parse is pinned here rather than trusted.
    """
    cases = [
        ("one call, no loop", "got := Meas(x)\nif got != 1 { t.Error() }", 1),
        ("two calls", "a := Meas(x)\nb := Meas(y)\nif a != b { t.Error() }", 2),
        ("range over a slice of ints",
         "for _, n := range sizes {\n\tif Meas(n) != 0 { t.Error() }\n}", 2),
        ("range over a composite literal -- the case that broke two versions",
         "for _, n := range []int{1, 2, 3} {\n\tif Meas(n) != 0 { t.Error() }\n}", 2),
        ("range over a struct table",
         "for _, c := range []struct{ n int }{{1}, {2}} {\n\tif Meas(c.n) != 0 { t.Error() }\n}", 2),
        ("call AFTER a range loop is not in it",
         "for i := range xs {\n\t_ = i\n}\nif Meas(x) != 0 { t.Error() }", 1),
        ("a plain for loop is not a range loop",
         "for i := 0; i < 3; i++ {\n\tif Meas(i) != 0 { t.Error() }\n}", 1),
    ]
    bad = 0
    for name, body, want in cases:
        got = states("Meas", ["w"], {"w": body})
        ok = got == want
        if not ok:
            bad += 1
        print("  %s %-58s want %d got %d" % ("ok " if ok else "BAD", name, want, got))
    if bad:
        print("\nselftest FAILED: %d snippet(s) counted wrong" % bad, file=sys.stderr)
        return 1
    print("selftest OK — the state counter separates one-state from many, and sees a call "
          "inside a range loop over a composite literal")
    return 0


def main():
    if "--selftest" in sys.argv:
        return selftest()
    inst = instruments()
    cal, bodies = calibrations()
    listing = "--list" in sys.argv

    missing, single = [], []
    for name, path in sorted(inst.items()):
        if name in EXEMPT:
            if listing:
                print("  exempt   %-24s %s  (%s)" % (name, path, EXEMPT[name]))
            continue
        where = cal.get(name)
        n = states(name, where, bodies) if where else 0
        if not where:
            verdict = "MISSING"
            missing.append((name, path))
        elif n < 2 and name not in EXEMPT_STATE:
            verdict = "1-STATE"
            single.append((name, path, where))
        else:
            verdict = "ok"
        if listing:
            print("  %-8s %-24s %-42s states=%s%s" % (
                verdict, name, path,
                "exempt" if name in EXEMPT_STATE else ("%d+" % n if n >= 2 else n),
                "  <- " + where[0] if where else ""))

    if missing:
        print("An instrument with no calibration is an opinion that returns a number:",
              file=sys.stderr)
        for name, path in missing:
            print("  %s (%s) is never called from a test that builds its own input and "
                  "asserts a literal answer" % (name, path), file=sys.stderr)
        print("\nFeed it something whose answer is known WITHOUT the emulator -- a synthetic "
              "waveform of known period, a hand-built array with a known mean -- and assert "
              "that. Checking an instrument against the machine it reads proves they agree, "
              "which is what hid audio.MeasurePeriod for a year.", file=sys.stderr)
    if single:
        print("\nAn instrument measured in ONE state cannot be told apart from a constant:",
              file=sys.stderr)
        for name, path, where in single:
            print("  %s (%s) is exercised at a single constructed input, in %s"
                  % (name, path, ", ".join(sorted(set(where)))), file=sys.stderr)
        print("\nCall it again with a DIFFERENT constructed input and assert the different "
              "answer -- or put the call inside a `for ... range` over a table of cases. "
              "Pick the second state where production actually lives: DominantFreq was "
              "calibrated at 8192 samples and called at 30,955, and the mutant that divides "
              "by len instead of nextPow2(len) is exact at the first and 100 cents wrong at "
              "the second.", file=sys.stderr)
    if missing or single:
        return 1

    print("instruments OK — all %d sample-reading measurements have a calibration that "
          "builds its own input, at two or more states (%d exempt, with reasons)"
          % (len(inst) - len(EXEMPT), len(EXEMPT) + len(EXEMPT_STATE)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
