#!/usr/bin/env python3
"""Is any scenario claim taken on ONE phase of an alternation?

A scenario that asserts `field == v` at a single frame cannot tell a constant from a
value that alternates every frame. If the field alternates, the assertion is pinned to
whichever phase the author happened to look at, and the other phase is unmeasured --
the same defect as measuring a value in one state and calling it a constant.

    python3 scripts/phase_probe.py             # sweep every scenario (slow: emulates each)
    python3 scripts/phase_probe.py --selftest  # controls only (fast)

This is NOT a gate. It emulates 129 ROMs and takes minutes; putting that in CI is how
`internal/emu` blew the per-package timeout in 2026-09-06. Run it when scenarios change
shape, not on every commit.

Measured 2026-09-07: 129 scenarios readable, **0** fields alternating on a single-phase
assertion. The zero is guarded by the positive control below -- text24 asserts
`tia.player0.hmoved_pixel` at frames 10 AND 11 (39 and 87, a genuine two-phase kernel);
delete one of the two and the probe fires. A zero from a probe that cannot fire is
indistinguishable from a probe that was never pointed anywhere.

★The first version of this probe reported **62** such fields and every one was wrong.
It paired output lines to plan entries BY POSITION, and the runner emits frame-major
while the plan was built field-major. The give-away was `bank.number` reported as
alternating 0/177 -- bank numbers are 0..3, so 177 was another field's value. The field
name was printed on every output line the whole time and the pairing threw it away.
This version reads the field name from the line and encodes the phase in the sentinel
value, so a misalignment cannot survive.
"""
import collections
import json
import os
import re
import subprocess
import sys

HARNESS = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OFFSETS = (-2, -1, 0, 1, 2)
SENTINEL = 9000  # -(SENTINEL + i) encodes which offset a reading came from
LINE = re.compile(r"(?:FAIL|ng)\s+(\S+)\s+==\s+-(\d{4})\s+\(got (-?\d+)\)")


def build_runner(workdir):
    exe = os.path.join(workdir, "scen")
    subprocess.run(["go", "build", "-o", exe, "./cmd/scenario"], cwd=HARNESS, check=True)
    return exe


def single_frame_fields(scenario):
    """Fields this scenario asserts with == at exactly one frame."""
    seen = collections.defaultdict(set)
    for a in scenario.get("asserts") or []:
        if "at_frame" in a and "field" in a and a.get("op") == "==":
            seen[a["field"]].add(a["at_frame"])
    return {k: sorted(v)[0] for k, v in seen.items() if len(v) == 1}


def read_around(exe, workdir, path, scenario, fields):
    """Read each field at frame-2 .. frame+2 by asserting an impossible value."""
    probe = dict(scenario)
    probe["checks"] = {}
    probe["asserts"] = [
        {"at_frame": fr + off, "field": fld, "op": "==", "value": -(SENTINEL + i)}
        for fld, fr in fields.items()
        for i, off in enumerate(OFFSETS)
        if fr + off >= 0
    ]
    p = os.path.join(workdir, "probe.json")
    with open(p, "w") as f:
        json.dump(probe, f)
    r = subprocess.run([exe, p], cwd=HARNESS, capture_output=True, text=True, timeout=300)
    got = collections.defaultdict(dict)
    for line in (r.stdout + r.stderr).split("\n"):
        m = LINE.search(line)
        if m:
            got[m.group(1)][int(m.group(2)) - SENTINEL] = int(m.group(3))
    return got


def alternates(vals):
    """True when the readings repeat with period two and the two phases differ."""
    return (
        len(vals) >= 4
        and all(vals[i] == vals[i + 2] for i in range(len(vals) - 2))
        and vals[0] != vals[1]
    )


def scan(exe, workdir, path):
    with open(path) as f:
        scenario = json.load(f)
    fields = single_frame_fields(scenario)
    if not fields:
        return []
    got = read_around(exe, workdir, path, scenario, fields)
    out = []
    for fld, fr in fields.items():
        vals = [got[fld][k] for k in sorted(got.get(fld, {}))]
        if alternates(vals):
            out.append((path, fld, fr, vals))
    return out


def selftest(workdir):
    """The probe must fire on a known alternation and stay silent on a constant."""
    exe = build_runner(workdir)
    src = os.path.join(HARNESS, "roms/techniques/scenarios/text24.json")
    with open(src) as f:
        base = json.load(f)
    field = "tia.player0.hmoved_pixel"
    frames = sorted(a["at_frame"] for a in base["asserts"] if a["field"] == field)
    if len(frames) < 2:
        print("SELFTEST FAIL — text24 no longer asserts %s at two frames, so the positive "
              "control is gone. Point this at another two-phase scenario." % field)
        sys.exit(1)

    # Positive: strip one of the two phases and the probe must notice.
    hurt = dict(base)
    hurt["checks"] = {}
    hurt["asserts"] = [a for a in base["asserts"]
                       if not (a["field"] == field and a["at_frame"] != frames[0])]
    p = os.path.join(workdir, "positive.json")
    with open(p, "w") as f:
        json.dump(hurt, f)
    hits = scan(exe, workdir, p)
    if not any(h[1] == field for h in hits):
        print("SELFTEST FAIL — the probe did not fire on a field known to alternate "
              "(%s in text24, 39/87). A zero from this probe would mean nothing." % field)
        sys.exit(1)
    print("selftest OK — fires on %s: %s" % (field, [h[3] for h in hits if h[1] == field][0]))

    # Negative: text24 unmodified asserts both phases, so nothing may fire.
    p = os.path.join(workdir, "negative.json")
    with open(p, "w") as f:
        json.dump(base, f)
    if scan(exe, workdir, p):
        print("SELFTEST FAIL — fired on text24 as written, which pins BOTH phases")
        sys.exit(1)
    print("selftest OK — silent on the same scenario with both phases asserted")


def main():
    import glob
    import tempfile
    with tempfile.TemporaryDirectory() as workdir:
        if "--selftest" in sys.argv:
            selftest(workdir)
            return
        exe = build_runner(workdir)
        files = sorted(glob.glob(os.path.join(HARNESS, "roms/*/scenarios/*.json")) +
                       glob.glob(os.path.join(HARNESS, "../roms/*/scenarios/*.json")))
        hits, read = [], 0
        for path in files:
            try:
                found = scan(exe, workdir, path)
            except Exception as e:
                print("  unreadable: %s (%s)" % (path, e))
                continue
            read += 1
            hits += found
        for path, fld, fr, vals in hits:
            print("ONE PHASE  %s  %s @frame %d  readings=%s"
                  % (os.path.relpath(path, HARNESS), fld, fr, vals))
        print("\n%d scenarios read, %d single-phase alternating field(s)" % (read, len(hits)))
        sys.exit(1 if hits else 0)


if __name__ == "__main__":
    main()
