#!/usr/bin/env python3
"""Carry-over proof A: a lock store genuinely written by the v0.7.20 binary
under retired policy 0, then read and written by the candidate binary."""
import json, os, shutil, subprocess, sys, hashlib

S = "/tmp/claude-0/-home-claude-dossierx/af1a85f0-622e-5ae6-be49-0e33bb124333/scratchpad"
OLD = f"{S}/gs/dx-0720"
NEW = f"{S}/gs/dx-cand"
D = f"{S}/r0721/carryA"
STORE = f"{D}/build/ledger/lock-store.json"
LOG = open(f"{S}/r0721/carryA.log", "w")

def run(binary, *args, ok=(0,)):
    p = subprocess.run([binary, *args], cwd=D, capture_output=True, text=True)
    LOG.write(f"$ {os.path.basename(binary)} {' '.join(args)}\nrc={p.returncode}\n{p.stdout[-3000:]}\n{p.stderr[-2000:]}\n")
    if ok is not None and p.returncode not in ok:
        print(f"UNEXPECTED rc={p.returncode}: {binary} {args}\n{p.stdout[-2000:]}\n{p.stderr[-1000:]}")
        sys.exit(1)
    return p

def env(p):
    try:
        return json.loads(p.stdout)
    except Exception:
        return None

def sha(path):
    return hashlib.sha256(open(path, "rb").read()).hexdigest()

def write(rel, text):
    path = f"{D}/{rel}"
    os.makedirs(os.path.dirname(path), exist_ok=True)
    open(path, "w").write(text)

# id -> rests_on
GRAPH = {"a": [], "b": ["a"], "c": ["b"], "d": ["c"], "e": ["b", "c"]}
def cid(x): return f"widget.contract.{x}"

def claim_old(x, body):
    s = f"id: {cid(x)}\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: |\n  {body}\n"
    if GRAPH[x]:
        s += "rests_on:\n" + "".join(f"  - {cid(y)}\n" for y in GRAPH[x])
    s += "governed_by:\n  type: none\n  reason: carry-over fixture\n"
    return s

def lock(binary, x, reason="carry-over fixture approval"):
    p = run(binary, "claim", "lock", cid(x), "--dry-run")
    tok = env(p)["data"].get("snapshot")
    if not tok:
        print("no proposal token", p.stdout[:2000]); sys.exit(1)
    run(binary, "claim", "lock", cid(x), "--reason", reason, "--proposal", tok)

def set_status(x, status, body=None):
    path = f"{D}/claims/{x}.yaml"
    t = open(path).read()
    t = t.replace("status: draft", f"status: {status}") if status == "locked" else t.replace("status: locked", "status: draft")
    if body:
        lines = t.split("\n")
        i = lines.index("body: |")
        lines[i + 1] = f"  {body}"
        t = "\n".join(lines)
    open(path, "w").write(t)

shutil.rmtree(D, ignore_errors=True)
os.makedirs(D)
write("project.config.yaml", "schema_version: 1\nfacets:\n  - contract\n  - internals\nmodules:\n  - widget\nclaims_dir: claims\n")
for x in GRAPH:
    write(f"claims/{x}.yaml", claim_old(x, f"Claim {x} states one fact."))

# 1. v0.7.20 creates the store with its first approval; then the store is made
#    to predate the policy field (what a pre-v1 store looks like on disk),
#    keeping the schema version v0.7.20 wrote.
lock(OLD, "a")
st = json.load(open(STORE))
LOG.write(f"v0.7.20 store after first lock: keys={sorted(st)} policy_version={st.get('policy_version')}\n")
print("v0.7.20 store after first lock: version", st.get("version"), "policy_version", st.get("policy_version", "<absent>"))
st.pop("policy_version", None); st.pop("policy_migrated_at", None); st.pop("policy_migration_reason", None)
json.dump(st, open(STORE, "w"), indent=2)

# 2. v0.7.20 locks in dependency order under policy 0; a policy-0 refusal of a
#    dependent before its dependency is also recorded.
p = run(OLD, "claim", "lock", cid("c"), "--dry-run", ok=None)
e = env(p) or {}; dd = e.get("data") or {}
print("v0.7.20 policy-0 dry-run of c before b: rc", p.returncode, "blocked", dd.get("blocked"), "policy", (dd.get("evaluation") or {}).get("policy_version"), "refusals", [v.get("refusals") for v in (dd.get("evaluation") or {}).get("verdicts") or []])
for x in ["b", "c", "d", "e"]:
    lock(OLD, x)
# 3. Create review causes: honest unlock of a, edit, re-lock (v0.7.20).
run(OLD, "claim", "unlock", cid("a"), "--reason", "carry-over fixture edit")
set_status("a", "draft", body="Claim a states one fact, now revised.")
lock(OLD, "a", reason="re-approve revised a")
run(OLD, "check", ok=None)
old_store = json.load(open(STORE))
print("v0.7.20 store policy_version:", old_store.get("policy_version", "<absent>"))
old_cat = json.load(open(f"{D}/build/catalog/catalog.json"))
shutil.copy(STORE, f"{S}/r0721/carryA-store-v0720.json")

def idents(r):
    out = set()
    for k in ("conditions", "causes"):
        for rec in r.get(k) or []:
            out.add(f"{k}|{rec.get('source_kind', rec.get('kind'))}|{rec.get('dependency_id', '')}|{'>'.join(rec.get('path') or [])}")
    return out
old_r = {c["id"]: c["readiness"] for c in old_cat["claims"]}
for i, r in sorted(old_r.items()):
    print(f"v0.7.20 {i}: policy={r['policy_version']} local={r['local_approved']} dep_ready={r['dependency_ready']} pending={r['review_pending']} ready={r['ready']} ids={sorted(idents(r))}")

# 4. Fold the corpus by hand per the upgrading skill: drop governed_by, add
#    summary, rests_on none on the root, manifest, constitution. Status stays locked.
for x in GRAPH:
    path = f"{D}/claims/{x}.yaml"
    t = open(path).read()
    t = t.split("governed_by:")[0]
    t = t.replace(f"id: {cid(x)}\n", f"id: {cid(x)}\nsummary: Claim {x} of the carry-over fixture.\n")
    if not GRAPH[x]:
        t += "rests_on:\n  none: true\n  reason: root of the carry-over fixture\n"
    open(path, "w").write(t)
write("claims/widget/manifest.yaml", "summary: carry-over fixture module.\nprovides:\n" + "".join(f"  - {cid(x)}\n" for x in GRAPH) + "depends_on: []\n")
write("constitution.yaml", "status: draft\ninvariants:\n  - slug: one-roof\n    title: One roof\n    body: The carry-over fixture has one roof.\n")

before = sha(STORE)
# 5. Candidate read-only surfaces must not write the carried-over store.
reads = [("check", "--validate"), ("claim", "list"), ("manifest", "show", "widget")] + [("claim", "show", cid(x)) for x in GRAPH]
new_r = {}
for args in reads:
    p = run(NEW, *args, ok=None)
    if args[:2] == ("claim", "show"):
        e = env(p)
        new_r[args[2]] = e["data"].get("readiness") if e and e.get("data") else None
after_reads = sha(STORE)
print("candidate read-only commands left store bytes unchanged:", before == after_reads, f"({len(reads)} commands)")

for i in sorted(old_r):
    r = new_r.get(i)
    if not r:
        print("MISSING candidate readiness for", i); continue
    lost = idents(old_r[i]) - idents(r)
    added = idents(r) - idents(old_r[i])
    print(f"cand {i}: policy={r['policy_version']} local={r['local_approved']} dep_ready={r['dependency_ready']} pending={r['review_pending']} ready={r['ready']} lost={sorted(lost)} added={len(added)}")

# 6. First candidate write: constitution lock (the upgrade's step 4).
def brief(p):
    e = env(p) or {}
    return f"rc={p.returncode} ok={e.get('ok')} code={(e.get('error') or {}).get('code')} msg={((e.get('error') or {}).get('message') or '')[:260]}"
p = run(NEW, "check", "--validate", ok=None)
print("candidate check --validate:", brief(p), "rules:", sorted({f.get('rule') for f in ((env(p) or {}).get('data') or {}).get('ledger_findings') or []}))
p = run(NEW, "constitution", "lock", "--reason", "carry-over fixture roof approval", ok=None)
print("candidate constitution lock:", brief(p))
p = run(NEW, "claim", "unlock", cid("e"), "--reason", "upgrade re-lock", ok=None)
print("candidate claim unlock e:", brief(p))
p = run(NEW, "claim", "lock", cid("e"), "--dry-run", ok=None)
print("candidate claim lock e --dry-run:", brief(p))
print("store bytes after candidate commands unchanged:", sha(STORE) == before)
if sha(STORE) == before:
    sys.exit(0)
new_store = json.load(open(STORE))
shutil.copy(STORE, f"{S}/r0721/carryA-store-after-candidate-write.json")
keys = sorted(set(old_store) | set(new_store))
for k in keys:
    same = old_store.get(k) == new_store.get(k)
    print(f"store key {k}: {'unchanged' if same else 'CHANGED'}" + ("" if same else f" -> {json.dumps(new_store.get(k))[:200]}"))
