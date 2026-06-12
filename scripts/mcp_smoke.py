#!/usr/bin/env python3
"""bin/harness の逐次スモークテスト（go-sdk は並行処理なので応答を待ってから次を送ること）。
使い方: python3 scripts/mcp_smoke.py  （harness/ から実行）"""
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
r=call(3,"step_frame",{"count":5})
print("smoke OK")
p.terminate()
