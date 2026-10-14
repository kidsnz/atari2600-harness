#!/usr/bin/env python3
"""Sequential smoke test of bin/harness -- the binary the editor's MCP connection runs.
Usage: python3 scripts/mcp_smoke.py [path/to/harness]   (run from harness/; default ./bin/harness)

The client is scripts/mcp_call.py's (one request at a time: the go-sdk answers concurrently).
A call the server refuses raises there, so a line below that only calls a tool is a check
that the tool answered."""
import os, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from mcp_call import Harness
h = Harness(binary=sys.argv[1] if len(sys.argv) > 1 else "./bin/harness")
print("initialize OK", h.version)
def call(_i, name, args):
    return {"result": {"structuredContent": h.call(name, args)}}
r=call(2,"load_rom",{"path":"roms/litmus/smoke.bin"})
r=call(3,"step_frame",{"count":30})
r=call(4,"analyze_screen",{})
r=call(5,"watch_ram",{"addr":0x80,"max_frames":2})
r=call(6,"trace_clocks",{"max_instructions":4})
r=call(7,"run_scenario",{"paths":["roms/litmus/scenarios/hmove_mid.json"]})
assert r["result"]["structuredContent"]["all_pass"], r
# U-M9: srcmap — via assemble_and_load, a budget overrun's `at` carries the source location
r=call(8,"assemble_and_load",{"asm_path":"roms/litmus/litmus_overrun.asm","bin_path":"/tmp/smoke_overrun.bin"})
assert r["result"]["structuredContent"]["ok"], r
r=call(9,"assert_line_budget",{"budget":76,"max_frames":2})
sc=r["result"]["structuredContent"]
assert sc["over"] and "litmus_overrun.asm:" in sc.get("at",""), sc
r=call(10,"trace_clocks",{"max_instructions":3})
assert "at" in r["result"]["structuredContent"]["trace"][0], r
# AT-2/3/4: authoring aids (beamtrace timeline / beam_race advisory / spritepos solver)
r=call(11,"load_rom",{"path":"roms/litmus/smoke.bin"})
r=call(12,"beamtrace",{"frames":1})
sc=r["result"]["structuredContent"]
assert sc["frames"] and sc["frames"][0]["rows"], str(sc)[:300]  # one entry a frame since ab201fe (2026-07-30); this line asked for the older shape until 2026-10-13
r=call(13,"beam_race",{"frames":1})
r=call(14,"spritepos",{"x":96,"object":"P0"})
sc=r["result"]["structuredContent"]
assert sc["solution"]["exact"] and sc["solution"]["achieved_x"]==96, sc
# SD-1/SD-2: static def-use and the proven beam window. Both are forall answers,
# so they are checked against a ROM whose behaviour is known: the stack-trick
# litmus writes COLUBK with a PHA at one exact beam position, and nothing but a
# tool that resolves the push can see that write at all.
r=call(15,"defuse",{"asm_path":"roms/litmus/litmus_stack_trick.asm"})
sc=r["result"]["structuredContent"]["report"]
assert sc["converged"], sc
assert any(w.get("exact") and w.get("addrs")==[265] for w in sc["writes"].values()), \
    "defuse did not resolve the PHA to $0109 (page-1 mirror of COLUBK)"
r=call(16,"beam_intervals",{"asm_path":"roms/litmus/litmus_stack_trick.asm"})
sc=r["result"]["structuredContent"]["report"]
found=[w for reg in sc["regions"] if reg.get("bounded") for w in (reg.get("writes") or []) if w["reg"]=="COLUBK"]
assert found and found[0]["exact"] and found[0]["min_clock"]==-11, found
# The uninitialised-read pair: fires on the bait, silent on the twin.
r=call(17,"defuse",{"asm_path":"roms/litmus/litmus_uninit_read.asm"})
assert len(r["result"]["structuredContent"]["report"].get("uninit_reads") or [])==1, r
r=call(18,"defuse",{"asm_path":"roms/litmus/litmus_uninit_clean.asm"})
assert not (r["result"]["structuredContent"]["report"].get("uninit_reads") or []), r

print("smoke OK (load/step/analyze_screen/watch_ram/trace_clocks/run_scenario/srcmap/beamtrace/beam_race/spritepos/defuse/beam_intervals)")
h.close()
