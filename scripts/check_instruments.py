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
    calls = {}
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
            for fn in re.findall(r"\b([A-Z]\w*)\(", body):
                calls.setdefault(fn, []).append(
                    "%s:%s" % (os.path.relpath(path, ROOT), m.group(1)))
    return calls


def main():
    inst = instruments()
    cal = calibrations()
    listing = "--list" in sys.argv

    missing = []
    for name, path in sorted(inst.items()):
        if name in EXEMPT:
            if listing:
                print("  exempt   %-24s %s  (%s)" % (name, path, EXEMPT[name]))
            continue
        where = cal.get(name)
        if listing:
            print("  %-8s %-24s %s%s" % ("ok" if where else "MISSING", name, path,
                                         "  <- " + where[0] if where else ""))
        if not where:
            missing.append((name, path))

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
        return 1

    print("instruments OK — all %d sample-reading measurements have a calibration that "
          "builds its own input (%d exempt, with reasons)" % (len(inst) - len(EXEMPT), len(EXEMPT)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
