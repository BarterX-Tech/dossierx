#!/usr/bin/env python3
"""Carry-over proof B (candidate only): a store written by the candidate with
approvals, baselines, receipts, a flag and live review causes is rewritten to
record retired policy 0 (and, separately, to lack the field). Readiness before
and after must be identical; reads must not write; the next write may add only
the requested claim's records and the three policy stamp fields."""
import json, os, shutil, subprocess, sys, hashlib

S = "/tmp/claude-0/-home-claude-dossierx/af1a85f0-622e-5ae6-be49-0e33bb124333/scratchpad"
NEW = f"{S}/gs/dx-cand"
D = f"{S}/r0721/carryB"
STORE = f"{D}/build/ledger/lock-store.json"
LOG = open(f"{S}/r0721/carryB.log", "w")
GRAPH = {"a": [], "b": ["a"], "c": ["b"], "d": ["c"], "e": ["b", "c"], "f": ["e"]}
def cid(x): return f"widget.contract.{x}"

def run(*args, ok=(0,)):
    p = subprocess.run([NEW, *args], cwd=D, capture_output=True, text=True)
    LOG.write(f"$ dx-cand {' '.join(args)}\nrc={p.returncode}\n{p.stdout[-3000:]}\n{p.stderr[-1000:]}\n")
    if ok is not None and p.returncode not in ok:
        print(f"UNEXPECTED rc={p.returncode}: {args}\n{p.stdout[-1500:]}"); sys.exit(1)
    return p

def env(p): return json.loads(p.stdout)
def sha(path): return hashlib.sha256(open(path, "rb").read()).hexdigest()
def write(rel, text):
    path = f"{D}/{rel}"; os.makedirs(os.path.dirname(path), exist_ok=True); open(path, "w").write(text)

def lock(x, reason="carry-over fixture approval"):
    tok = env(run("claim", "lock", cid(x), "--dry-run"))["data"]["snapshot"]
    run("claim", "lock", cid(x), "--reason", reason, "--proposal", tok)

def readiness_all():
    out = {}
    for x in GRAPH:
        out[x] = env(run("claim", "show", cid(x)))["data"]["readiness"]
    cat = json.load(open(f"{D}/build/catalog/catalog.json"))
    return out, cat

shutil.rmtree(D, ignore_errors=True); os.makedirs(D)
write("project.config.yaml", "schema_version: 1\nfacets:\n  - contract\n  - internals\nmodules:\n  - widget\nclaims_dir: claims\n")
write("constitution.yaml", "status: draft\ninvariants:\n  - slug: one-roof\n    title: One roof\n    body: The carry-over fixture has one roof.\n")
write("claims/widget/manifest.yaml", "summary: carry-over fixture module.\nprovides:\n" + "".join(f"  - {cid(x)}\n" for x in GRAPH) + "depends_on: []\n")
for x, deps in GRAPH.items():
    s = f"id: {cid(x)}\nsummary: Claim {x} of the carry-over fixture.\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: |\n  Claim {x} states one fact.\n"
    s += ("rests_on:\n" + "".join(f"  - {cid(y)}\n" for y in deps)) if deps else "rests_on:\n  none: true\n  reason: root of the fixture\n"
    write(f"claims/{x}.yaml", s)

run("constitution", "lock", "--reason", "fixture roof")
for x in ["a", "b", "c", "d", "e"]:
    lock(x)
# f locks against a DRAFT parent? No: e is locked. Leave f draft.
# causes: honest unlock/edit/relock of a (drift for b), flag on d.
run("claim", "unlock", cid("a"), "--reason", "edit a")
t = open(f"{D}/claims/a.yaml").read().replace("Claim a states one fact.", "Claim a states one revised fact.")
open(f"{D}/claims/a.yaml", "w").write(t)
lock("a", reason="re-approve a")
run("claim", "flag", cid("d"), "--claim-says", "d says x", "--now-does", "d does y", "--reason", "fixture flag")
run("check", ok=None)
v1_store_raw = open(STORE).read()
v1_store = json.loads(v1_store_raw)
print("candidate-written store policy_version:", v1_store.get("policy_version"), "keys:", sorted(v1_store))
r_v1, cat_v1 = readiness_all()

results = {}
for variant in ("policy0", "absent"):
    st = json.loads(v1_store_raw)
    st.pop("policy_migrated_at", None); st.pop("policy_migration_reason", None)
    if variant == "policy0":
        st["policy_version"] = 0
    else:
        st.pop("policy_version", None)
    open(STORE, "w").write(json.dumps(st, indent=2))
    before = sha(STORE)
    run("check", ok=None)  # plain check: catalog + viewer regenerate
    r_old, cat_old = readiness_all()
    run("check", "--validate", ok=None); run("claim", "list")
    reads_unchanged = sha(STORE) == before
    same_show = r_old == r_v1
    same_cat = [c.get("readiness") for c in cat_old["claims"]] == [c.get("readiness") for c in cat_v1["claims"]]
    # next write: lock f (its dependency e is locked; b..e carry causes)
    lock("f", reason="first write after carry-over")
    after = json.load(open(STORE))
    diffs = {}
    for k in sorted(set(st) | set(after)):
        if st.get(k) != after.get(k):
            if isinstance(after.get(k), dict) and isinstance(st.get(k), dict):
                changed = sorted(i for i in set(st[k]) | set(after[k]) if st[k].get(i) != after[k].get(i))
                diffs[k] = changed
            else:
                diffs[k] = after.get(k)
    r_after, _ = readiness_all()
    others_same = all(r_after[x] == r_old[x] for x in GRAPH if x != "f")
    print(f"[{variant}] reads left store unchanged={reads_unchanged}; claim show readiness identical to v1 store={same_show}; catalog readiness identical={same_cat}")
    print(f"[{variant}] store diff after lock f: {json.dumps(diffs)[:600]}")
    print(f"[{variant}] readiness of a..e unchanged by the write={others_same}")
    for x in GRAPH:
        r = r_after[x]
        kinds = sorted({(c.get('source_kind')) for c in r.get('causes') or []} | {c.get('kind') for c in r.get('conditions') or []})
        print(f"   {x}: policy={r['policy_version']} local={r['local_approved']} dep_ready={r['dependency_ready']} pending={r['review_pending']} ready={r['ready']} kinds={kinds}")
    # reset f for the next variant
    run("claim", "unlock", cid("f"), "--reason", "reset")
    open(STORE, "w").write(v1_store_raw)
    t = open(f"{D}/claims/f.yaml").read().replace("status: locked", "status: draft")
    open(f"{D}/claims/f.yaml", "w").write(t)
