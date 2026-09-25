#!/usr/bin/env python3
"""Compare committed v0.7.20 fixture catalogs with candidate catalogs, per claim id."""
import json, subprocess, collections, sys
REPO = "/home/claude/dossierx/.claude/worktrees/agent-ad7950c8c1896b8de"
FIX = ["fixture-basic", "fixture-conformance-v1", "fixture-graph-demo", "fixture-portability", "fixture-theme-flat"]

def load(rev, f):
    raw = subprocess.run(["git", "-C", REPO, "show", f"{rev}:testdata/{f}/build/catalog/catalog.json"], capture_output=True, check=True).stdout
    return json.loads(raw)

def summ(c):
    r = c.get("readiness") or {}
    kinds = collections.Counter()
    for k in ("conditions", "review_causes", "dependency_conditions", "causes"):
        for rec in r.get(k) or []:
            kinds[f"{k}:{rec.get('kind')}/{rec.get('source_kind','')}"] += 1
    return {"status": c.get("status"), "facet": c.get("facet"),
            "b": (r.get("local_approved"), r.get("dependency_ready"), r.get("review_pending"), r.get("ready")),
            "policy": r.get("policy_version"), "kinds": dict(kinds), "edges": c.get("edges")}

for f in FIX:
    old = {c["id"]: c for c in load("v0.7.20", f)["claims"]}
    new = {c["id"]: c for c in load("20072b7", f)["claims"]}
    shared = sorted(set(old) & set(new))
    only_old = sorted(set(old) - set(new))
    only_new = sorted(set(new) - set(old))
    same_b = diff_b = 0
    notes = []
    for i in shared:
        a, b = summ(old[i]), summ(new[i])
        if a["b"] == b["b"]:
            same_b += 1
        else:
            diff_b += 1
            notes.append(f"    {i}: v0.7.20 {a['status']} {a['b']} {a['kinds']} edges={a['edges']}\n      cand   {b['status']} {b['b']} {b['kinds']} edges={b['edges']}")
    print(f"{f}: v0.7.20 claims={len(old)} candidate exported={len(new)} shared={len(shared)} booleans equal={same_b} differ={diff_b}")
    pol = collections.Counter(str(summ(c)['policy']) for c in old.values())
    print(f"  v0.7.20 policy_version values: {dict(pol)}; candidate: {dict(collections.Counter(str(summ(c)['policy']) for c in new.values()))}")
    if only_old:
        facets = collections.Counter(old[i].get('facet') for i in only_old)
        print(f"  only in v0.7.20: {len(only_old)} by facet {dict(facets)}")
    if only_new:
        print(f"  only in candidate: {only_new}")
    for n in notes:
        print(n)
