#!/usr/bin/env python3
"""Lock-store carry-over and brief one-way proof for v0.7.22.

A lock store genuinely written by the v0.7.21 binary is read, folded and
re-locked by the candidate; then a brief is locked over it (version 4), its
rests_on claim moves, and the claim graph is compared with and without the
brief; finally a store from a newer binary is refused without a write.

Usage: carry.py <dx-v0.7.21> <dx-candidate> <v0.7.21 tree> <scratch dir>
Every step asserts; any surprise exits non-zero.
"""
import hashlib, json, os, re, shutil, subprocess, sys

OLD, NEW, BASETREE, SCRATCH = sys.argv[1:5]
D = os.path.join(SCRATCH, "carry")
STORE = os.path.join(D, "build/ledger/lock-store.json")
LOG = open(os.path.join(SCRATCH, "carry.log"), "w")
A, B, C = "widget.contract.overview", "widget.internals.fields", "widget.contract.c"
checks = 0

def run(binary, *args, env_extra=None):
    env = dict(os.environ, DOSSIERX_ACTOR="proof", **(env_extra or {}))
    p = subprocess.run([binary, *args], cwd=D, capture_output=True, text=True, env=env)
    LOG.write(f"$ {os.path.basename(binary)} {' '.join(args)}\nrc={p.returncode}\n{p.stdout[-4000:]}\n{p.stderr[-2000:]}\n")
    return p

def js(p):
    return json.loads(p.stdout)

def sha(path):
    return hashlib.sha256(open(path, "rb").read()).hexdigest()

def ok(cond, what):
    global checks
    checks += 1
    print(("PASS " if cond else "FAIL ") + what)
    if not cond:
        sys.exit(1)

def lock(binary, cid, reason):
    p = run(binary, "claim", "lock", cid, "--dry-run")
    tok = js(p)["data"].get("snapshot")
    ok(bool(tok), f"{os.path.basename(binary)} claim lock {cid} --dry-run gives a proposal")
    p = run(binary, "claim", "lock", cid, "--reason", reason, "--proposal", tok)
    ok(p.returncode == 0, f"{os.path.basename(binary)} claim lock {cid} rc=0")

def readiness(cid):
    p = run(NEW, "claim", "show", cid)
    ok(p.returncode == 0, f"candidate claim show {cid} rc=0")
    return js(p)["data"]["readiness"]

def kinds(r):
    return sorted({c.get("source_kind") or c.get("kind") for c in (r.get("causes") or [])} |
                  {c.get("kind") for c in (r.get("dependency_conditions") or [])})

def ledger_rules(p):
    d = js(p)
    return sorted({(f.get("rule"), f.get("claim_id")) for f in (d.get("data") or {}).get("ledger_findings") or []})

# --- 1. v0.7.21 writes the store -------------------------------------------
shutil.rmtree(D, ignore_errors=True)
shutil.copytree(os.path.join(BASETREE, "testdata/fixture-basic"), D)
shutil.rmtree(os.path.join(D, "build/viewer"), ignore_errors=True)
fields = open(f"{D}/claims/widget-fields.yaml").read()
fields = fields.replace("facet: internals\n", "facet: internals\nmigrated_from: old/spec.md#fields\n")
open(f"{D}/claims/widget-fields.yaml", "w").write(fields)
open(f"{D}/claims/widget-c.yaml", "w").write(
    "id: widget.contract.c\nsummary: A second contract claim for the carry-over proof.\n"
    "facet: contract\nmodule: widget\nstatus: draft\nlayout: card\n"
    "migrated_from: free-text note naming nothing\nbody: |\n  C rests on the overview.\n"
    "rests_on:\n  - widget.contract.overview\n")
for cid, why in ((A, "approve a"), (B, "approve b"), (C, "approve c")):
    lock(OLD, cid, why)
st = json.load(open(STORE))
ok(st["version"] == 3 and "briefs" not in st, f"v0.7.21 store is version {st['version']} with no briefs key")
ok(sorted(st["ledger"]) == sorted([A, B, C]), "v0.7.21 ledger holds the three approvals")
snap = json.dumps(st["ledger"][B])
ok('"MigratedFrom"' in snap, "v0.7.21 ledger snapshot carries the retired MigratedFrom key")
shutil.copy(STORE, os.path.join(SCRATCH, "carry-lock-store-written-by-v0.7.21.json"))
h0 = sha(STORE)

# --- 2. the candidate on the unfolded corpus refuses at load, writes nothing --
p = run(NEW, "check", "--validate")
d = js(p)
ok(p.returncode != 0 and d["error"]["code"] == "invalid_claim" and "migrated_from" in p.stdout,
   "unfolded corpus: check --validate refuses invalid_claim naming migrated_from")
ok(sha(STORE) == h0, "unfolded refusal left the store byte-identical")

# --- 3. fold; candidate read-only commands write nothing ---------------------
for f in ("widget-fields.yaml", "widget-c.yaml"):
    t = open(f"{D}/claims/{f}").read()
    open(f"{D}/claims/{f}", "w").write(re.sub(r"^migrated_from:.*\n", "", t, flags=re.M))
p = run(NEW, "check", "--validate")
rules = ledger_rules(p)
ok(rules == sorted([("lock-content-drift", A), ("lock-content-drift", B), ("lock-content-drift", C)]),
   f"folded: ledger findings are exactly lock-content-drift x3 (got {rules})")
ok("lock-ledger-unreadable" not in p.stdout, "no lock-ledger-unreadable: the v0.7.21 snapshots decode")
run(NEW, "claim", "list")
for cid in (A, B, C):
    r = readiness(cid)
    ok(r["local_approved"] is False and "approval_content_drift" in kinds(r),
       f"{cid}: local_approved false with approval_content_drift")
    ok("direct_dependency_change" not in kinds(r), f"{cid}: no dependency baseline moved (ContentHash unchanged)")
ok(sha(STORE) == h0, "check --validate, claim list and claim show x3 on the folded corpus left the store byte-identical")

# --- 4. unlock and re-lock each once -----------------------------------------
for cid in (A, B, C):
    p = run(NEW, "claim", "unlock", cid, "--reason", "v0.7.22 re-lock")
    ok(p.returncode == 0, f"candidate claim unlock {cid} rc=0")
for cid in (A, B, C):
    lock(NEW, cid, "re-lock after the v0.7.22 fold")
p = run(NEW, "check")
ok(p.returncode == 0, "after re-lock: plain check rc=0")
for cid in (A, B, C):
    r = readiness(cid)
    ok(r["ready"] is True, f"{cid}: ready after re-lock")
st = json.load(open(STORE))
ok(st["version"] == 3 and "briefs" not in st, "a store with no brief record stays version 3")
cat_nobrief = open(f"{D}/build/catalog/catalog.json").read()

# --- 5. a brief rests on a claim; locking it earns version 4 -----------------
os.makedirs(f"{D}/briefs/notes", exist_ok=True)
open(f"{D}/briefs/notes/why.md", "w").write(
    "---\nsummary: Why the widget exists.\nrests_on:\n  - widget.contract.overview\n---\n# Why\n\nBecause.\n")
p = run(NEW, "brief", "lock", "briefs/notes/why.md", "--reason", "approve the brief")
ok(p.returncode == 0, "candidate brief lock rc=0")
st = json.load(open(STORE))
ok(st["version"] == 4 and list(st["briefs"]) == ["notes.why"], "first brief record raises the store to version 4")
p = run(NEW, "check")
ok(p.returncode == 0, "check rc=0 with the locked brief")
cat_brief = open(f"{D}/build/catalog/catalog.json").read()
ok(re.sub(r'"generated_at": "[^"]*"', "", cat_brief) == re.sub(r'"generated_at": "[^"]*"', "", cat_nobrief),
   "catalog.json (readiness included) is identical with and without the locked brief")

# --- 6. the claim moves: the brief goes review_pending, the claim graph does not learn of it
p = run(NEW, "claim", "unlock", A, "--reason", "edit a")
ok(p.returncode == 0, "unlock overview")
t = open(f"{D}/claims/widget-overview.yaml").read()
open(f"{D}/claims/widget-overview.yaml", "w").write(t.replace("smallest unit", "smallest documented unit"))
lock(NEW, A, "approve the edit")
for cid in (B, C):
    r = readiness(cid)
    ok(r["review_pending"] is True and "direct_dependency_change" in kinds(r),
       f"{cid}: review_pending via direct_dependency_change (claim propagation unchanged)")
p = run(NEW, "brief", "show", "notes.why")
bd = js(p)["data"]
ok(bd.get("review_pending") is True and [c.get("id") or c.get("claim_id") for c in bd.get("changed_claims") or []] == [A],
   "brief review_pending with exactly the moved claim in changed_claims")
p = run(NEW, "check", "--validate")
lint = js(p)["data"].get("lint_findings") or []
ok(any((f.get("lint") or f.get("rule")) == "brief-dependency-drift" for f in lint), "check reports brief-dependency-drift")
reads_with = {cid: readiness(cid) for cid in (A, B, C)}
# same corpus with no brief at all
bak = os.path.join(SCRATCH, "carry-briefs.bak")
shutil.rmtree(bak, ignore_errors=True)
shutil.move(f"{D}/briefs", bak)
st = json.load(open(STORE)); st_saved = json.dumps(st)
del st["briefs"]; st["version"] = 3
json.dump(st, open(STORE, "w"), indent=2)
reads_without = {cid: readiness(cid) for cid in (A, B, C)}
ok(reads_with == reads_without, "claim show readiness of all three claims identical with and without the brief")
shutil.move(bak, f"{D}/briefs")
open(STORE, "w").write(json.dumps(json.loads(st_saved), indent=2))
p = run(NEW, "brief", "reaudit", "notes.why", "--confirm", "--reason", "seen")
ok(p.returncode == 0, "brief reaudit --confirm rc=0")
ok(js(run(NEW, "brief", "show", "notes.why"))["data"].get("review_pending") is False, "reaudit clears the brief only")
for cid in (B, C):
    ok(readiness(cid)["review_pending"] is True, f"{cid}: still review_pending (the brief's reaudit clears no claim cause)")

# --- 7. a store from a newer binary is refused, never rewritten -------------
st = json.load(open(STORE)); st["version"] = 5
json.dump(st, open(STORE, "w"), indent=2)
h5 = sha(STORE)
p = run(NEW, "check", "--validate")
ok("lock-ledger-unreadable" in p.stdout and "pgrade" in p.stdout, "version-5 store: check names lock-ledger-unreadable with an upgrade hint")
p = run(NEW, "claim", "unlock", B, "--reason", "x")
ok(p.returncode != 0 and js(p)["error"]["code"] == "store_too_new", "version-5 store: claim unlock refused store_too_new")
r = readiness(A)
ok(r["ready"] is False and "approval_unknown" in kinds(r), "version-5 store: claim show fails closed (approval_unknown, not ready)")
p = run(NEW, "brief", "unlock", "notes.why", "--reason", "x")
ok(p.returncode != 0, "version-5 store: brief unlock refused")
ok(sha(STORE) == h5, "version-5 store left byte-identical by every refused command")
print(f"carry.py: {checks} checks passed")
