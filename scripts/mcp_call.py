#!/usr/bin/env python3
"""mcp_call.py -- the MCP server's tools, from a script or a shell.

The server (cmd/harness) holds most of what this repository can measure, and its answers
reach whoever is in the conversation as text. A script could not ask: one work transcribed
240 frames of read_audio_trace by hand (three of six series came out the wrong length), and
the works after it wrote Go programs of their own against internal/emu for what a tool here
already returns. This is the way in for a script:

    python3 scripts/mcp_call.py load_rom '{"path":"roms/litmus/smoke.bin"}' \\
                                step_frame '{"count":30}' read_tia '{}'

Tool names and JSON arguments in pairs. ONE server runs them in order, so the state carries
from call to call, and each answer is printed as one line of JSON (the tool's structured
content). A call the server refuses stops the run with exit code 1 and the server's words.

    python3 scripts/mcp_call.py -              calls from standard input, one a line: ["tool", {...}]
    python3 scripts/mcp_call.py --list [word]  the tools, with the first sentence of what each is for
    python3 scripts/mcp_call.py --describe T   all of tool T's description, and its arguments
    python3 scripts/mcp_call.py --selftest

From Python, for a loop a command line cannot write:

    sys.path.insert(0, "<harness>/scripts"); from mcp_call import Harness
    with Harness() as h:
        h.call("load_rom", path=rom)
        for frame in range(600):
            h.call("step_frame", count=1)
            rows.append(h.call("read_row", y=100))

WHICH SERVER. Not bin/harness: that file is what the editor's MCP connection runs, it is
shared by every session, and it is only as new as its last rebuild. Each run here builds the
server from the source as it is now into a directory of its own and removes it afterwards
(about two seconds once Go's build cache is warm), so a script is never answered by an old
binary. --bin PATH runs a given binary instead (scripts/mcp_smoke.py does, to test bin/harness).

Paths in arguments are the server's to resolve, and it runs in the directory you called from.
The picture some tools save (ATARI2600_SCREEN_PATH) goes to the run's own directory and is
removed with it, so that a script does not overwrite the preview someone is looking at;
--screen FILE keeps it.
"""
import json, os, shutil, subprocess, sys, tempfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent


class ToolError(Exception):
    """The server refused a call, or the tool answered with an error."""


class Harness:
    def __init__(self, binary=None, screen=None, cwd=None):
        self._tmp = tempfile.mkdtemp(prefix="harness-mcp-")
        self.p = None
        if binary is None:
            binary = os.path.join(self._tmp, "harness")
            r = subprocess.run(["go", "build", "-o", binary, "./cmd/harness"], cwd=ROOT, capture_output=True, text=True)
            if r.returncode:
                shutil.rmtree(self._tmp, ignore_errors=True)
                raise SystemExit("mcp_call: the server does not build from this source:\n" + (r.stderr or r.stdout)[-2000:])
        env = dict(os.environ, ATARI2600_SCREEN_PATH=os.path.abspath(screen) if screen else os.path.join(self._tmp, "screen.png"))
        self.p = subprocess.Popen([binary], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                                  text=True, cwd=cwd, env=env)
        self._id = 0
        info = self._request("initialize", {"protocolVersion": "2024-11-05", "capabilities": {},
                                            "clientInfo": {"name": "mcp_call", "version": "0"}})
        self.version = info["serverInfo"]["version"]
        self._send({"jsonrpc": "2.0", "method": "notifications/initialized"})

    def _send(self, msg):
        self.p.stdin.write(json.dumps(msg) + "\n")
        self.p.stdin.flush()

    def _request(self, method, params):
        # one request at a time: the server answers concurrently, so wait for this id
        self._id += 1
        self._send({"jsonrpc": "2.0", "id": self._id, "method": method, "params": params})
        while True:
            line = self.p.stdout.readline()
            if not line:
                raise ToolError("the server went away during %s" % method)
            try:
                d = json.loads(line)
            except ValueError:
                continue
            if d.get("id") != self._id:
                continue
            if "error" in d:
                raise ToolError("%s: %s" % (params.get("name", method), d["error"].get("message", d["error"])))
            return d["result"]

    def tools(self):
        out, cursor = [], None
        while True:
            r = self._request("tools/list", {"cursor": cursor} if cursor else {})
            out += r["tools"]
            cursor = r.get("nextCursor")
            if not cursor:
                return out

    def call(self, name, args=None, **kw):
        """Run one tool. Returns its structured content, or its text when it has none."""
        r = self._request("tools/call", {"name": name, "arguments": dict(args or {}, **kw)})
        text = "\n".join(c.get("text", "") for c in r.get("content", []) if c.get("type") == "text")
        if r.get("isError"):
            raise ToolError("%s: %s" % (name, text or "error"))
        return r["structuredContent"] if "structuredContent" in r else text

    def close(self):
        if self.p:
            self.p.terminate()
            try:
                self.p.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self.p.kill()
            self.p = None
        shutil.rmtree(self._tmp, ignore_errors=True)

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        self.close()


def first_sentence(text):
    text = " ".join((text or "").split())
    for i, ch in enumerate(text):
        if ch == "." and i > 30 and text[i + 1:i + 2] in ("", " "):
            return text[:i + 1]
    return text[:200]


def selftest():
    rom = str(ROOT / "roms" / "litmus" / "smoke.bin")
    bad = []
    with Harness() as h:
        names = [t["name"] for t in h.tools()]
        if not {"load_rom", "step_frame", "read_ram", "read_audio_trace"} <= set(names):
            bad.append("the tool list lacks load_rom, step_frame, read_ram or read_audio_trace: %d tools" % len(names))
        h.call("load_rom", path=rom)
        a = h.call("step_frame", count=3)
        b = h.call("step_frame", {"count": 4})                # the state carries: 3 frames, then 4 more
        if (a["coords"]["frame"], b["coords"]["frame"]) != (3, 7):
            bad.append("step_frame 3 then 4 did not read frame 3 then 7: %r then %r" % (a, b))
        for what, name, args in (("a tool that does not exist", "no_such_tool", {}),
                                 ("a ROM that does not exist", "load_rom", {"path": rom + ".missing"})):
            try:
                h.call(name, args)
                bad.append("%s was not refused" % what)
            except ToolError:
                pass
        tmp = h._tmp
    if os.path.exists(tmp):
        bad.append("the run's directory %s was left behind" % tmp)
    for b_ in bad:
        print("  ✗", b_)
    print("mcp_call selftest:", "FAILED" if bad else "ok (%d tools)" % len(names))
    return 1 if bad else 0


def main():
    args = sys.argv[1:]
    if not args or args[0] in ("-h", "--help"):
        sys.exit(__doc__)
    if args == ["--selftest"]:
        return selftest()
    binary = screen = None
    while args and args[0] in ("--bin", "--screen"):
        if len(args) < 2:
            sys.exit(args[0] + " needs a path")
        if args[0] == "--bin":
            binary = args[1]
        else:
            screen = args[1]
        args = args[2:]
    if args and args[0] in ("--list", "--describe"):
        with Harness(binary, screen) as h:
            tools = h.tools()
        if args[0] == "--describe":
            want = [t for t in tools if t["name"] in args[1:]]
            if not want:
                sys.exit("no tool of that name; --list shows them")
            for t in want:
                print(t["name"] + "\n\n" + (t.get("description") or "") + "\n\narguments: " + json.dumps(t.get("inputSchema", {}).get("properties", {}), indent=1))
            return 0
        words = [w.lower() for w in args[1:]]
        for t in sorted(tools, key=lambda t: t["name"]):
            if all(w in (t["name"] + " " + (t.get("description") or "")).lower() for w in words):
                print("%-26s %s" % (t["name"], first_sentence(t.get("description"))))
        return 0
    if args == ["-"]:
        calls = [json.loads(line) for line in sys.stdin if line.strip()]
    else:
        if len(args) % 2:
            sys.exit("tool names and JSON arguments come in pairs (use '{}' for none): " + " ".join(args))
        calls = []
        for name, raw in zip(args[0::2], args[1::2]):
            try:
                calls.append([name, json.loads(raw)])
            except ValueError as e:
                sys.exit("the arguments of %s are not JSON (%s): %s" % (name, e, raw))
    for c in calls:
        if not (isinstance(c, list) and len(c) == 2 and isinstance(c[0], str) and isinstance(c[1], dict)):
            sys.exit('a call is ["tool", {arguments}]: %r' % (c,))
    with Harness(binary, screen) as h:
        for name, a in calls:
            try:
                print(json.dumps(h.call(name, a), ensure_ascii=False))
            except ToolError as e:
                print("mcp_call:", e, file=sys.stderr)
                return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
