# Graph-safety evidence: comment threads on briefs in the viewer (NIT-198, v0.7.22)

Scope: viewer data, serve routes and one lint rule. The brief page (NIT-197)
gains its thread counts and, for a static build, its threads baked in; the
sidebar mark gains the open-thread state; the "All briefs" index and Home's
Open threads card count a brief's open threads; `dossierx serve` gains seven
routes under `/api/briefs/{id}/comments` that call the `Brief*` thread
operations NIT-205 added to `internal/comments`, and a catch-all under
`/api/briefs/`; and `source-internal-drift` pins an internal source that
cites a brief by the brief's content hash (audit F1, below).

The change is a consumer and a router, never a producer of graph facts. It
reads each brief's `comments` (already parsed by `internal/briefs`) and counts
the open ones with `Brief.OpenThreads`, the same function the payload's
`open_threads` and `check`'s `open_brief_comments` use. It adds no claim edge,
cause, condition, baseline or traversal, computes no readiness, and writes
nothing except through the NIT-205 operations, whose locking, digest and
refusal rules are unchanged (docs/graph-safety/nit-192-briefs.md).

- Candidate: the head of `work/nit-198-viewer-comment-threads-on-briefs`,
  rebased onto `41f603be` (the NIT-178 combo branch after NIT-205 and its
  #137 follow-up) for the audit round. The figures below
  were measured on the working tree before this note was committed, with the
  commands shown.
- Environment: go1.26.5 darwin/arm64.

## What each element reads

| Element | Source | Rule |
| --- | --- | --- |
| Sidebar mark | the brief's frontmatter `status` and `comments` | `thread` when `OpenThreads() > 0`, else `locked` or `draft`; the board's `edited` and `review`, which outrank `thread`, are NIT-199's and NIT-200's |
| Page counts (`data-open-threads`, `data-threads`) | the brief's `comments` | open and total threads |
| Baked panel | the brief's `comments` | components' `comments.html`, keyed `data-brief-id`; only for a brief with a thread |
| Index line | `OpenThreads()` | "N open thread(s)" on a brief with any |
| `GET /api/briefs/{id}/comments` | `comments.Deps.BriefList` | one brief's threads, from the working tree, no lock |
| Brief writes | `comments.Deps.Brief{Add,Reply,Resolve,Reopen,Edit,Delete}` | unchanged NIT-205 operations |
| Home's Open threads card | `OpenThreads()` of every brief, beside each claim's `OpenThreadIDs()` | summed; leads to the first claim with one, else the first brief |
| `source-internal-drift` on a brief | `briefs.LockHash` via `lint.BriefContentHash` | see below |

## The brief pin (audit F1)

Before: an `internal` source's `sha256` pinned the whole file, so every thread
write into a cited brief's frontmatter (and a `brief lock` flipping its
status) was `source-internal-drift` on every claim citing it. After: when the
source's path resolves to `briefs_dir/<folder>/<slug>.md` and `record_id` is
unset, the pin is the brief's content hash — `briefs.LockHash(summary,
rests_on, body)`, the value `brief lock` signs and `brief show` prints as
`content` / `content_hash`. It is computed by parsing the file exactly as
discovery does (`briefs.FromFiles` on one file), and handed to the lint
through `lint.BriefContentHash`, which `internal/briefs` sets at init because
it already imports `internal/lint`. A build that links no brief reader
reports the source as unchecked rather than falling back to the whole file.
Every other internal source keeps its whole-file (or JSONL record) pin.

This is a lint verdict, not a graph fact: no edge, cause, baseline or
readiness input changed. `sources` stays outside the dependency-drift
`ContentHash` and inside the lock hash, as before; what changed is only which
bytes of a brief the pin compares. `--validate`, `--staged` (which lints
sources from the working tree, as it always has) and the NIT-202 relationship
row (`sourceDriftCheck`, which runs the registered lint) share the one lint,
so they cannot disagree. `TestACitedBriefsPinIgnoresItsThreadsAndStatus`
(`internal/check`) runs both modes through a thread add, reply and resolve and
a status flip (clean), a body edit (drift) and a status-line edit to a note
outside `briefs_dir` (drift, whole-file). Mutations — skipping the brief
branch, hashing the file digest instead of the lock hash, never reporting a
brief mismatch — each fail it. Cost: one parse of the cited brief per source,
O(brief bytes), memoised per pin in the render as before.

## Preserved invariants

- **A claim never learns about a brief's threads.** No readiness, lock or
  catalog package changed; the only engine changes outside render and serve
  are the brief pin (the `source-internal-drift` branch and the `briefs`
  init that feeds it) and `brief show`'s `content:` line:

      git diff --stat 41f603be -- internal/lock internal/readiness \
        internal/catalog internal/model internal/manifest internal/loader \
        internal/reaudit internal/comments internal/digest       # empty

- **The claim routes answer byte for byte as before.** `commentDTO` now embeds
  the thread fields as `threadDTO` after `claim_id`; `encoding/json` flattens
  an embedded struct in field order, so the claim wire bytes are unchanged
  (checked with a standalone encode of the old and new shapes, and by every
  existing claim route test, unedited). `GET /api/comments` still lists claims
  only; `TestBriefComments_TheHumanOpensRepliesAndResolves` asserts a brief's
  thread never appears there.
- **A brief is addressed by its id and only its id.** `internal/briefs`'
  `Lookup` accepts a path too; `briefRef` refuses any `{id}` holding a slash or
  backslash (an escaped `%2F` arrives decoded), so a path, `./`-prefixed id or
  unknown id is `404 brief_not_found` and writes nothing
  (`TestBriefComments_AnIDAndOnlyAnID`). Mutation: dropping the refusal fails
  it.
- **A brief and a claim sharing an id never cross.** A project claim
  (`project.<slug>`) and a brief in `briefs/project/` share an id. The routes
  are separate namespaces, the viewer keys a baked panel by `data-brief-id`
  (never `data-claim-id`) and the rail's subject key for a brief starts with
  U+0000, which no `data-claim-id` can carry (the HTML parser replaces it)
  (`TestBriefComments_ABriefAndAClaimSharingAnIDNeverCross`).
- **Rights and admission are the claim routes'.** Every brief write is in the
  admission matrix beside the claim writes (Host, Origin, Content-Type,
  Sec-Fetch-Site: `TestAdmission_*`), refused `read_only` by a hook-less shell
  (`TestViewerDegradation_HookLessOverride`), and an agent resolving or
  deleting the human's thread is `403 rights_denied`. Mutation: ignoring
  `?as=` on DELETE, or dropping the admission's mutating-method gate, fails
  them.
- **Live reload still swaps only `<nav>` and `<main>`.** The counts and the
  baked panel live inside the brief section in `<main>`; the rail
  (`#commentsPanel`) stays outside both, as for claims, and is re-rendered by
  subject key after a swap. No render stamp was added inside `<main>`/`<nav>`.
- **A renamed or deleted brief.** Its section leaves the page on the reload;
  an open rail on it asks the API, gets 404, says the brief is no longer in the
  project and offers no composer. The threads recorded for it stay
  `comment-digest-abandoned`, NIT-205's rule.

## Complexity and output size

Per render: one pass over each brief's threads to count them, and for a brief
with threads one execution of the comments panel template over them:
O(total thread and reply bytes). Nothing is paired, traversed or enumerated.
The baked panel is the only new output that grows with content; it carries
each thread once, is written through the bounded render's `capacityWriter`, and
so is charged to the static viewer's 64 MiB bound with everything else. Serve
renders unbounded, as before.

Measured with a scratch test calling `renderBoundedAt` (unbounded) on generated
sets (80-word bodies, 60-word replies), not committed:

| Case | render | page |
| --- | --- | --- |
| 60 briefs, no threads | 7.3 ms | 1.41 MB |
| 60 briefs × 20 open threads × 5 replies | 34.0 ms | 6.09 MB |
| 2,000 briefs × 5 threads × 2 replies | 177.9 ms | 26.6 MB |

Each route handles one brief: `BriefList` is one discovery and a filter,
O(briefs + that brief's threads); the writes are NIT-205's operations,
measured there.

## Verdict

PASS for this change: every applicable obligation above ran on the candidate.
Excluded, with reason: the readiness shape matrix (chains, diamonds, cycles,
dense DAGs), because no traversal, edge, cause, condition, baseline or lock
state changed, and the write path is NIT-205's, unchanged. This is not
release approval.
