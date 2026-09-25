#!/usr/bin/env python3
"""Live serve smoke on the carry-over B corpus: initial render carries the
readiness facts, and an upstream change (flag on e) reaches the served page
for its dependent f after the watcher rebuild."""
import subprocess, time, urllib.request, re, json, os, signal, shutil
S = "/tmp/claude-0/-home-claude-dossierx/af1a85f0-622e-5ae6-be49-0e33bb124333/scratchpad"
D = f"{S}/r0721/serveB"
shutil.rmtree(D, ignore_errors=True)
shutil.copytree(f"{S}/r0721/carryB", D)
NEW = f"{S}/gs/dx-cand"
port = 18777
p = subprocess.Popen([NEW, "serve", "--port", str(port)], cwd=D, stdout=subprocess.PIPE, stderr=subprocess.STDOUT)
def get(path):
    for _ in range(100):
        try:
            return urllib.request.urlopen(f"http://127.0.0.1:{port}{path}", timeout=5).read().decode()
        except Exception:
            time.sleep(0.2)
    raise SystemExit("serve did not answer")
def facts(html):
    return {"own_flag": html.count('"source_kind":"own_flag"'), "direct_dependency_change": html.count('"source_kind":"direct_dependency_change"'),
            "f_mentions_e_flag": len(re.findall(r'"source_kind":"own_flag"[^}]*"path":\["widget\.contract\.f","widget\.contract\.e"\]', html))}
try:
    h0 = get("/")
    f0 = facts(h0)
    st0 = json.loads(get("/api/status"))
    print("initial:", f0, "bytes", len(h0), "status ok keys", sorted(st0)[:6])
    r = subprocess.run([NEW, "claim", "flag", "widget.contract.e", "--claim-says", "e says x", "--now-does", "e does y", "--reason", "upstream change"], cwd=D, capture_output=True, text=True)
    print("flag e rc", r.returncode)
    f1 = f0
    for _ in range(50):
        time.sleep(0.3)
        h1 = get("/")
        f1 = facts(h1)
        if f1 != f0:
            break
    print("after upstream flag:", f1)
    cli = json.loads(subprocess.run([NEW, "claim", "show", "widget.contract.f"], cwd=D, capture_output=True, text=True).stdout)["data"]["readiness"]
    print("claim show f agrees: own_flag via e =", any(c.get("source_kind") == "own_flag" and c.get("path") == ["widget.contract.f", "widget.contract.e"] for c in cli.get("causes") or []))
finally:
    p.send_signal(signal.SIGINT)
    try:
        p.wait(timeout=5)
    except Exception:
        p.kill()
