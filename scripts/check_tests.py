#!/usr/bin/env python3
"""A test that cannot fail is not verification — find the ones that can't.

Why this exists. Repeatedly this project has found checks that pass while covering
nothing: a linter reading 0 of 133 instructions, 38 of 95 scenarios run by no gate, a
smoothness gate that passes on a sprite which never moves, a soundness grading aimed at
the wrong bank. The extreme case of that family is a test function containing no way to
fail at all — it contributes a green tick that means precisely nothing, and nothing in
`go test` will ever say so.

Measured when this was written: 344 test functions in the tree, exactly ONE with no
failure path — `TestZZProbe`, a scratch probe swept into a docs commit that printed to
stdout and asserted nothing. One is a number worth keeping at zero.

The rule. A `func TestXxx(t *testing.T)` body must either call a failure/skip method on
`t`, or hand `t` to something else (a table runner, a shared helper — delegation is
normal and the helper does the asserting). Anything else is a test that runs code and
draws no conclusion.

Usage:  python3 scripts/check_tests.py [--selftest]     (run from harness/)
"""
import os
import re
import sys

HARNESS = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# A failure path: t.Error/Fatal/Fail/Skip in any form, or a testify-style assertion.
FAILS = re.compile(r"\bt\.(Error|Errorf|Fatal|Fatalf|Fail|FailNow|Skip|Skipf|Skipped)\b"
                   r"|\b(require|assert)\.\w+\(")
# Delegation: t handed to another function, or a subtest that carries its own *testing.T.
DELEGATES = re.compile(r"\bt\.Run\b|\w+\(\s*t\s*[,)]|\(\s*t\s*,")

SKIP_DIRS = ("Gopher2600", ".git", "node_modules", "third_party")


def test_bodies(src):
    """Yield (name, body) for each top-level func TestXxx in one file."""
    parts = re.split(r"\nfunc (Test\w+)\(", src)
    for i in range(1, len(parts), 2):
        name, rest = parts[i], parts[i + 1]
        end = rest.find("\nfunc ")
        yield name, (rest[:end] if end > 0 else rest)


def scan(root):
    offenders, total = [], 0
    for dirpath, dirnames, files in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        for fn in files:
            if not fn.endswith("_test.go"):
                continue
            path = os.path.join(dirpath, fn)
            with open(path, encoding="utf-8", errors="replace") as fh:
                src = fh.read()
            for name, body in test_bodies(src):
                total += 1
                if not FAILS.search(body) and not DELEGATES.search(body):
                    offenders.append(os.path.relpath(path, root) + ":" + name)
    return offenders, total


def selftest():
    """The detector must fire on bait and stay quiet on a delegating test.

    Without this the check could silently match nothing — the same defect it exists to
    catch, one level up.
    """
    bait = "\nfunc TestBait(t *testing.T) {\n\tx := compute()\n\tfmt.Println(x)\n}\n"
    delegating = "\nfunc TestFine(t *testing.T) {\n\tfor _, c := range cases {\n\t\thelper(t, c)\n\t}\n}\n"
    asserting = "\nfunc TestAlsoFine(t *testing.T) {\n\tif got != want {\n\t\tt.Fatalf(\"no\")\n\t}\n}\n"
    ok = True
    for name, body in test_bodies(bait):
        if FAILS.search(body) or DELEGATES.search(body):
            print("selftest FAILED: the bait test was not detected")
            ok = False
    for src, label in ((delegating, "delegating"), (asserting, "asserting")):
        for name, body in test_bodies(src):
            if not (FAILS.search(body) or DELEGATES.search(body)):
                print(f"selftest FAILED: a {label} test was reported as unable to fail")
                ok = False
    if ok:
        print("selftest OK — the detector fires on the bait and clears delegating/asserting tests")
    return 0 if ok else 1


def main():
    if "--selftest" in sys.argv:
        return selftest()
    offenders, total = scan(HARNESS)
    if offenders:
        print("A test with no failure path is a green tick that means nothing:")
        for o in offenders:
            print("  ", o)
        print("\nGive it an assertion, hand `t` to a helper that has one, or delete it.")
        return 1
    print(f"tests OK — all {total} test functions can fail (assert directly or via a helper).")
    return 0


if __name__ == "__main__":
    sys.exit(main())
