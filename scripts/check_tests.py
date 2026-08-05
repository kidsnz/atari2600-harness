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

RULE 1 — a way to fail. A `func TestXxx(t *testing.T)` body must either call a
failure method on `t`, or hand `t` to something else (a table runner, a shared helper —
delegation is normal and the helper does the asserting). Anything else runs code and
draws no conclusion. `t.Skip` does NOT count: skipping is how a test declines to say
anything, so a body whose only `t.` call is a Skip fails this rule.

RULE 2 — an assertion about the SUBJECT (added 2026-08-04). Rule 1 was satisfied by
the wrong kind of failure. `internal/emu/zz_dyn_test.go` counted the frames that were
not 262 scanlines and PRINTED them; `bad` could reach 28 and the test still passed,
because the only `t.Fatal` in the function was the error check on `LoadROM`. This gate
saw that `t.Fatal` and was satisfied — without asking whether the assertion was about
the thing under test or about the plumbing that set it up.

So: a test whose failure paths are ALL bare `if err != nil` checks is asserting that
its fixtures loaded, and nothing else. `if err == nil { t.Fatal(...) }` is the opposite
— a test that an error IS produced is a statement about behaviour — and counts.

Audited when rule 2 landed: 427 test functions under internal/** and pkg/**, of which
1 asserted only its setup and 4 more computed a real measurement and printed it. Rule 2
catches the shape; it cannot catch a test that prints its verdict while asserting some
OTHER measured fact (a premise check like "the corpus is non-empty" satisfies it), so
it narrows the hole rather than closing it. Read a new test's failure paths yourself.

Usage:  python3 scripts/check_tests.py [--selftest]     (run from harness/)
"""
import os
import re
import sys

HARNESS = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# A failure path: t.Error/Fatal/Fail in any form, or a testify-style assertion.
# t.Skip is deliberately NOT here — declining to run is not a way to fail.
FAILS = re.compile(r"\bt\.(Error|Errorf|Fatal|Fatalf|Fail|FailNow)\b"
                   r"|\b(require|assert)\.\w+\(")
# Delegation: t handed to another function, or a subtest that carries its own *testing.T.
DELEGATES = re.compile(r"\bt\.Run\b|\w+\(\s*t\s*[,)]|\(\s*t\s*,")
# The name of a function `t` is handed to, so its body can be read too.
DELEGATE_NAME = re.compile(r"\b(\w+)\(\s*t\s*[,)]")
NOT_A_HELPER = {"Run", "Helper", "Error", "Errorf", "Fatal", "Fatalf", "Fail", "FailNow",
                "Log", "Logf", "Skip", "Skipf", "Cleanup", "Setenv", "Chdir", "TempDir"}

# `err != nil` — the setup check. Any identifier ending in err/Err counts.
ERR_NE_NIL = re.compile(r"\b\w*[eE]rr\w*\s*!=\s*nil")
# `err == nil` — asserting an error IS returned, which is about behaviour.
ERR_EQ_NIL = re.compile(r"\b\w*[eE]rr\w*\s*==\s*nil")
# A guard that opens a block: `if x {`, `} else if x {`, `for ... {`, `case x:`.
GUARD = re.compile(r"^(if|for|case|switch|}\s*else)\b")

SKIP_DIRS = ("Gopher2600", ".git", ".claude", "node_modules", "third_party")


def test_bodies(src):
    """Yield (name, body) for each top-level func TestXxx in one file."""
    parts = re.split(r"\nfunc (Test\w+)\(", src)
    for i in range(1, len(parts), 2):
        name, rest = parts[i], parts[i + 1]
        end = rest.find("\nfunc ")
        yield name, (rest[:end] if end > 0 else rest)


def package_funcs(src):
    """name -> body for every top-level func in one file (helpers included)."""
    out = {}
    parts = re.split(r"\nfunc (?:\([^)]*\)\s*)?(\w+)\(", src)
    for i in range(1, len(parts), 2):
        name, rest = parts[i], parts[i + 1]
        end = rest.find("\nfunc ")
        out[name] = rest[:end] if end > 0 else rest
    return out


def asserts_beyond_setup(body):
    """True if any failure path is guarded by something other than `err != nil`.

    Walks back from each failure call to the `if`/`for`/`case` that guards it. A guard
    naming only `err != nil` is a fixture check; anything else — a comparison, a length,
    a boolean, `err == nil`, or no guard at all — is a claim about the subject.
    """
    lines = body.split("\n")
    for i, line in enumerate(lines):
        if not FAILS.search(line):
            continue
        guard = ""
        for j in range(i, max(-1, i - 15), -1):
            s = lines[j].strip()
            if GUARD.match(s) and (s.endswith("{") or s.endswith(":") or "{" in s):
                guard = s
                break
        if not guard:
            return True  # an unguarded t.Fatal is a claim ("we should never get here")
        if ERR_EQ_NIL.search(guard):
            return True  # "this must FAIL" is a statement about behaviour
        if not ERR_NE_NIL.search(guard):
            return True
    return False


def scan(root):
    """Return (rule-1 offenders, rule-2 offenders, total tests)."""
    pkg_funcs, test_files = {}, []
    for dirpath, dirnames, files in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        for fn in files:
            if not fn.endswith(".go"):
                continue
            path = os.path.join(dirpath, fn)
            with open(path, encoding="utf-8", errors="replace") as fh:
                src = fh.read()
            pkg_funcs.setdefault(dirpath, {}).update(package_funcs(src))
            if fn.endswith("_test.go"):
                test_files.append((path, dirpath, src))

    cannot_fail, setup_only, total = [], [], 0
    for path, dirpath, src in sorted(test_files):
        for name, body in test_bodies(src):
            total += 1
            where = os.path.relpath(path, root) + ":" + name
            delegates = DELEGATES.search(body)
            if not FAILS.search(body) and not delegates:
                cannot_fail.append(where)
                continue
            if asserts_beyond_setup(body):
                continue
            # Every failure path here is `if err != nil`. Before calling it setup-only,
            # read the helpers `t` was handed to: a table-driven test asserts through a
            # shared function, and that function is where the claim lives.
            helped = False
            for m in DELEGATE_NAME.finditer(body):
                h = m.group(1)
                if h in NOT_A_HELPER or h == name:
                    continue
                hbody = pkg_funcs.get(dirpath, {}).get(h)
                if hbody is None:
                    helped = True  # helper is elsewhere; do not guess — stay quiet
                    break
                if asserts_beyond_setup(hbody):
                    helped = True
                    break
            if not helped:
                setup_only.append(where)
    return cannot_fail, setup_only, total


def selftest():
    """The detector must fire on bait and stay quiet on tests that do assert.

    Without this the check could silently match nothing — the same defect it exists to
    catch, one level up.
    """
    ok = True

    def body_of(src):
        return next(test_bodies(src))[1]

    # RULE 1 — no failure path at all.
    for label, src in (
        ("printing bait", "\nfunc TestBait(t *testing.T) {\n\tx := compute()\n\tfmt.Println(x)\n}\n"),
        ("skip-only bait", "\nfunc TestSkip(t *testing.T) {\n\tif n == 0 {\n\t\tt.Skip(\"nothing\")\n\t}\n}\n"),
    ):
        b = body_of(src)
        if FAILS.search(b) or DELEGATES.search(b):
            print(f"selftest FAILED: the {label} was not detected by rule 1")
            ok = False
    for label, src in (
        ("delegating", "\nfunc TestFine(t *testing.T) {\n\tfor _, c := range cases {\n\t\thelper(t, c)\n\t}\n}\n"),
        ("asserting", "\nfunc TestAlsoFine(t *testing.T) {\n\tif got != want {\n\t\tt.Fatalf(\"no\")\n\t}\n}\n"),
    ):
        b = body_of(src)
        if not (FAILS.search(b) or DELEGATES.search(b)):
            print(f"selftest FAILED: a {label} test was reported as unable to fail")
            ok = False

    # RULE 2 — failure paths that only check the setup.
    setup_only_bait = (
        "\nfunc TestBaitB(t *testing.T) {\n"
        "\te, err := New()\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n"
        "\tif err := e.LoadROM(rom); err != nil {\n\t\tt.Fatalf(\"load: %v\", err)\n\t}\n"
        "\tbad := e.Count()\n\tt.Logf(\"bad=%d\", bad)\n}\n")
    if asserts_beyond_setup(body_of(setup_only_bait)):
        print("selftest FAILED: a test whose only failure paths are err-checks passed rule 2")
        ok = False
    for label, src in (
        ("value comparison", "\nfunc TestC(t *testing.T) {\n\tif err != nil {\n\t\tt.Fatal(err)\n\t}\n"
                             "\tif got != 262 {\n\t\tt.Errorf(\"got %d\", got)\n\t}\n}\n"),
        ("negative error test", "\nfunc TestD(t *testing.T) {\n\tif _, err := Parse(bad); err == nil {\n"
                                "\t\tt.Fatal(\"expected a rejection\")\n\t}\n}\n"),
        ("unguarded fatal", "\nfunc TestE(t *testing.T) {\n\tswitch v := f(); {\n\t}\n\tt.Fatal(\"unreachable\")\n}\n"),
    ):
        if not asserts_beyond_setup(body_of(src)):
            print(f"selftest FAILED: a {label} test was reported as asserting only its setup")
            ok = False

    if ok:
        print("selftest OK — rule 1 fires on printing/skip-only bait, rule 2 fires on a body whose "
              "only failure paths are err-checks, and both clear tests that assert.")
    return 0 if ok else 1


def main():
    if "--selftest" in sys.argv:
        return selftest()
    cannot_fail, setup_only, total = scan(HARNESS)
    rc = 0
    if cannot_fail:
        print("A test with no failure path is a green tick that means nothing:")
        for o in cannot_fail:
            print("  ", o)
        print("\nGive it an assertion, hand `t` to a helper that has one, or delete it.")
        rc = 1
    if setup_only:
        print("A test whose only failure paths are `err != nil` asserts that its fixtures loaded,")
        print("not that the code under test is right:")
        for o in setup_only:
            print("  ", o)
        print("\nAssert the measured fact the test is named after — a printed number is not a check.")
        rc = 1
    if rc == 0:
        print(f"tests OK — all {total} test functions can fail, and none of them fail only when "
              f"their setup does.")
    return rc


if __name__ == "__main__":
    sys.exit(main())
