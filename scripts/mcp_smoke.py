#!/usr/bin/env python3
"""Sequential smoke test of bin/harness (the go-sdk is concurrent, so wait for each response before sending the next).
Usage: python3 scripts/mcp_smoke.py  (run from harness/)"""
import subprocess, json, sys
p = subprocess.Popen(["./bin/harness"], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True)
def call(i, name, args):
    p.stdin.write(json.dumps({"jsonrpc":"2.0","id":i,"method":"tools/call","params":{"name":name,"arguments":args}})+"\n"); p.stdin.flush()
    while True:
        line = p.stdout.readline()
        if not line: sys.exit("server died")
        try: d=json.loads(line)
        except: continue
        if d.get("id")==i: return d
p.stdin.write(json.dumps({"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"0"}}})+"\n")
p.stdin.write(json.dumps({"jsonrpc":"2.0","method":"notifications/initialized"})+"\n"); p.stdin.flush()
while True:
    d=json.loads(p.stdout.readline())
    if d.get("id")==1: print("initialize OK", d["result"]["serverInfo"]["version"]); break
r=call(2,"load_rom",{"path":"roms/litmus/smoke.bin"})
assert not r["result"].get("isError"), r
r=call(3,"step_frame",{"count":30})
r=call(4,"analyze_screen",{})
assert not r["result"].get("isError"), r
r=call(5,"watch_ram",{"addr":0x80,"max_frames":2})
assert not r["result"].get("isError"), r
r=call(6,"trace_clocks",{"max_instructions":4})
assert not r["result"].get("isError"), r
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
assert not r["result"].get("isError") and "rows" in r["result"]["structuredContent"], r
r=call(13,"beam_race",{"frames":1})
assert not r["result"].get("isError"), r
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
p.terminate()
