# Graph-safety evidence: briefs, the read side (NIT-204, v0.7.22)

Scope: a second authored tree beside the claims and everything that reads it.

- `internal/briefs`: discovery (`Load` over the working tree, `FromFiles` for
  both sources), frontmatter, the four caps, and the two claim-aware rules
  (`brief-rests-on-unknown`, `brief-rests-on-duplicate`).
- `internal/render`: `Extras`, `RenderWith` and `RenderBoundedWith`, and the
  `dossierx-briefs` payload on both shell paths — eager (the embedded shell,
  charged to the output budget) and lazy (a project shell override's
  `{{.BriefsPayload}}`, charged to the intermediate budget).
- `internal/serve`: the watcher's second tree (`briefs_dir`) and the re-read
  of the briefs on every render.
- `internal/check`: `lintFindings` (brief findings join `lint_findings`) and
  `stagedBriefs` (the index's briefs tree for `check --staged`).
- `cmd/dossierx`: `brief list` and `brief show`.

A brief is viewer data, which is why this note exists (AGENTS.md: viewer data
is a graph-safety trigger). Briefs add no claim edge, cause, condition,
baseline or traversal. `rests_on` on a brief is a list of claim ids the brief
reads; nothing follows it, and no claim learns it exists. Nothing in this
change writes a file: no store, no sentinel, no approval.

- Baseline: `4944032861cc2c6f72ec66deefc96f7c35782666` (release/v0.7.22 after
  PR #131). Candidate: the head of
  `work/nit-204-engine-192a-briefs-read-side-discovery-caps-listshow-render`
  carrying this note; the figures below were measured on the commit that adds
  it, with the commands shown.
- Environment: go1.26.5 darwin/arm64 (Apple M4 Pro).

## Preserved invariants

- **A claim never learns about briefs.** The claim-side packages do not import
  `internal/briefs`, directly or through a neighbour, and none of them changed:

      go list -deps ./internal/lock ./internal/readiness ./internal/catalog \
        ./internal/model ./internal/lint ./internal/manifest ./internal/loader \
        ./internal/reaudit | grep -c internal/briefs          # 0
      git diff --stat 4944032861cc..HEAD -- internal/lock internal/readiness \
        internal/catalog internal/model internal/lint internal/manifest \
        internal/loader internal/reaudit                       # empty

  The only importers are `cmd/dossierx/brief.go`, `internal/check`
  (`ledger.go`, `staged.go`), `internal/render` and `internal/serve`. Since
  `internal/model` and `internal/lock` never see a brief, no brief byte can
  enter a claim hash.
- **Brief state never gates a claim.** `internal/lock/policy.go:93` builds the
  lock evaluator's findings from `lint.RunAll` alone; the brief rules are
  `briefs.Rules`, not `lint.Registry`. `TestBriefFindingsFailCheckAndNeverGateAClaim`
  (`cmd/dossierx/brief_cli_test.go`) holds a brief ERROR whose message names
  the lock candidate (the evaluator's own scoping rule would block on it) and
  asserts the preview is not blocked and carries no `brief-*` rule anywhere.
  Mutation check: appending `briefs.Load(cfg).Findings` to `allFindings` in
  `policy.go` fails it (`blocked: true`).
- **No brief, no change.** On five corpora the baseline and candidate binaries
  produce byte-identical `check` and `check --validate` envelopes, `claim list`,
  `manifest show <m> --isolation` and `--integration` for every module,
  `catalog.json`, both ledgers and the viewer `index.html` (timestamps and the
  build version normalised). With its brief, graph-demo differs in exactly one
  added line of `index.html`, the `dossierx-briefs` payload block:

  | Corpus | Artifacts compared | Identical |
  | --- | --- | --- |
  | fixture-basic | 9 | 9 |
  | fixture-conformance-v1 | 10 | 10 |
  | fixture-portability | 9 | 9 |
  | fixture-theme-flat | 9 | 9 |
  | fixture-graph-demo, `briefs/` removed | 10 | 10 |
  | fixture-graph-demo, with its brief | 10 | 9 (`index.html`: +1 line, the payload) |

  `TestRenderWith_BriefsPayload` pins the same byte-identity in-process, and
  the committed fixture viewers' staleness test proves it on four corpora.
- **Both check modes judge the same tree.** `--validate` reads the working
  tree and `--staged` the index, under the same `FromFiles` rules: a brief
  edited but unstaged is seen by one and not the other
  (`TestBriefFindingsFollowTheTreeEachModeJudges`), and a symlink or gitlink
  under `briefs_dir` — including a linked `briefs_dir` — is refused by both
  (`TestBriefSymlinksAreRefusedInBothModes`), where the index used to drop it.
- **Nothing is silently unread.** An unreadable folder or file is one
  `brief-shape` finding on its path and the rest of the tree is still read;
  `brief list` and `brief show` carry the tree's findings in `data.findings`
  (`TestLoad_AnUnreadableEntryIsOneFindingNotAnEmptyTree`,
  `TestBriefListAndShowReportWhatTheyCouldNotRead`). A read error in the
  briefs tree is isolated to that tree's fingerprint, so claim live reload
  continues (`TestSSE_UnreadableBriefFolderKeepsClaimReload`).
- **The payload is charged.** Every payload byte comes off the output budget
  (eager) or the intermediate budget (lazy), once, and a budget one byte short
  refuses the render at the payload
  (`TestBuildEagerShellData_ChargesTheBriefsPayloadToTheOutputBudget`,
  `TestLazyShell_BriefsPayload`; removing the charge fails both).

## Complexity and output size

N briefs, B their total bytes, F files in the tree, R the longest `rests_on`,
C the claims, P the longest brief path. Every pass is linear in its input up
to a sort; none enumerates paths or pairs.

| Pass | Bound |
| --- | --- |
| Discovery (`Load` / `FromFiles`) | one walk and one sort of F entries, O(F log F); each brief parsed by four block scans of its own bytes (title, text, accepted images, refused images) plus the frontmatter, O(B) |
| Caps | one pass over briefs, images and folders, O(N + images) |
| `brief-rests-on-unknown` | a claim-id set, O(C), and one lookup per `rests_on` entry, O(N·R) |
| `brief-rests-on-duplicate` | one sorted key per brief, O(N·R log R); one finding per member of a group, each naming **one** other path and a count of the rest, so a group of k is O(k·P) bytes (it was O(k²·P) before REG-4) |
| Findings order | one sort, O(K log K) for K findings |
| Payload | one document-mode render per brief and one JSON encode, O(B); charged to the render budget before the shell executes |
| `check --staged` | two `git ls-files -s` over the briefs pathspec and one `cat-file --batch`, O(index entries under `briefs_dir`) |
| `serve` watcher | one stat-walk of the briefs tree per poll, O(F) |
| `brief list` / `brief show` | discovery plus one pass over the findings, O(F log F + B) |

The default caps (60 briefs, 2,000 words, 3 images of 1 MiB) bound a default
project's briefs input; a raised cap raises it linearly. Measured on the
worst case for the duplicate rule — every brief at the word cap, three present
images each, twelve to a folder, and all resting on the same two claims, one
group of N:

| Briefs | `FromFiles` + `Findings` | allocations | bytes allocated | finding message bytes |
| --- | --- | --- | --- | --- |
| 60 | 9.3 ms | 11,876 | 11.6 MB | 11,460 |
| 600 | 92 ms | 118,665 | 115 MB | 115,389 |
| 2,000 | 290 ms | 395,203 | 382 MB | 386,190 |

At 2,000 the duplicate findings' messages were about 88 MB before REG-4; they
are 386 KB now, and `TestFindings_DuplicateMessageIsBoundedInGroupSize` fails
if any one exceeds 256 bytes at k = 2,000. The payload at the word cap:

| Briefs | `briefsPayloadJSONWithBudget` | allocations | bytes allocated | payload bytes |
| --- | --- | --- | --- | --- |
| 60 | 1.9 ms | 620 | 3.3 MB | 616,990 |
| 2,000 | 61 ms | 20,040 | 132 MB | 20,561,333 |

A default project's payload is therefore about 0.6 MB, under the 64 MiB viewer
bound (`conformance.MaxOutputBytes`), which refuses a larger one at the
payload.

    go test ./internal/briefs -run '^$' -bench BriefsAtScale -benchmem
    go test ./internal/render -run '^$' -bench BriefsPayload -benchmem
    go test ./internal/briefs -run TestFindings_DuplicateMessageIsBoundedInGroupSize -v

## The differential, as run

`dx-base` is built from `git archive 4944032861cc`, `dx-cand` from the
candidate; both run over identical copies of the candidate's fixtures (whose
claims and configs match the baseline's; graph-demo gains only its `briefs/`
and a config comment). With `W` the repository root and `R` a scratch
directory holding the archive as `base.tar`:

```bash
mkdir -p "$R/basetree" && tar -xf "$R/base.tar" -C "$R/basetree"
(cd "$R/basetree" && go build -trimpath -o "$R/dx-base" ./cmd/dossierx)
(cd "$W" && go build -trimpath -o "$R/dx-cand" ./cmd/dossierx)
norm() { sed -E 's/20[0-9]{2}-[0-9]{2}-[0-9]{2}T[0-9:]{8}(\.[0-9]+)?Z/TS/g; s#/(base|cand)-#/X-#g; s/"duration_ms":[0-9]+/"duration_ms":N/g; s/"version":"[^"]*"/"version":V/g' "$1"; }
run() {  # run <label> <fixture> [nobriefs]
  for v in base cand; do
    d="$R/$v-$1"; rm -rf "$d"; cp -R "$W/testdata/$2" "$d"
    [ "${3:-}" = nobriefs ] && rm -rf "$d/briefs"
    ( cd "$d"
      "$R/dx-$v" check > "$R/$v-$1.check.json" 2> "$R/$v-$1.check.err"; echo "exit $?" >> "$R/$v-$1.check.err"
      "$R/dx-$v" check --validate > "$R/$v-$1.validate.json" 2>&1; echo "exit $?" >> "$R/$v-$1.validate.json"
      "$R/dx-$v" claim list > "$R/$v-$1.list.json" 2>&1
      for m in $(ls claims | grep -v '\.'); do
        "$R/dx-$v" manifest show "$m" --isolation; "$R/dx-$v" manifest show "$m" --integration
      done > "$R/$v-$1.manifest.json" 2>&1 )
  done
  for k in check.json check.err validate.json list.json manifest.json; do
    diff <(norm "$R/base-$1.$k") <(norm "$R/cand-$1.$k") > /dev/null || echo "$1 $k DIFF"
  done
  for p in $(cd "$R/base-$1" && find build -type f \( -name '*.json' -o -name '*.html' \)); do
    cmp -s <(norm "$R/base-$1/$p") <(norm "$R/cand-$1/$p") || echo "$1 $p DIFF"
  done
}
run basic fixture-basic; run conformance-v1 fixture-conformance-v1
run portability fixture-portability; run theme-flat fixture-theme-flat
run graph-demo-nobriefs fixture-graph-demo nobriefs; run graph-demo fixture-graph-demo
diff -a <(norm "$R/base-graph-demo/build/viewer/index.html") <(norm "$R/cand-graph-demo/build/viewer/index.html")
```

Its only output is `graph-demo build/viewer/index.html DIFF`, and the last
`diff` prints one added line, `<script type="application/json"
id="dossierx-briefs">…</script>`. Every `check` exited 0 on both binaries.

## Exclusions

- **The readiness shape matrix** (chains, diamonds, cycles, dense DAGs) and
  the witness-path bounds: no traversal, edge, cause, condition or baseline
  changed, and readiness does not read briefs (the import evidence above).
- **Lock and write-path mutation proofs**: nothing here writes, and the lock
  evaluator's code and inputs are unchanged; the one lock-adjacent obligation,
  that brief findings never reach it, is the negative control above.
- **Catalog and graph-payload projections**: neither carries briefs;
  `catalog.json` and the graph payload inside `index.html` are byte-identical
  on every corpus above.

## Verdict

PASS for this change: every applicable obligation above ran on the candidate.
This is not release approval.
