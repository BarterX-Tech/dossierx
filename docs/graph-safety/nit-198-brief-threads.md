# Graph-safety evidence: comment threads on briefs in the viewer (NIT-198, v0.7.22)

Scope: viewer data and serve routes. The brief page (NIT-197) gains its
thread counts and, for a static build, its threads baked in; the sidebar mark
gains the open-thread state; the "All briefs" index gains a per-brief count;
and `dossierx serve` gains seven routes under `/api/briefs/{id}/comments` that
call the `Brief*` thread operations NIT-205 added to `internal/comments`.

The change is a consumer and a router, never a producer of graph facts. It
reads each brief's `comments` (already parsed by `internal/briefs`) and counts
the open ones with `Brief.OpenThreads`, the same function the payload's
`open_threads` and `check`'s `open_brief_comments` use. It adds no claim edge,
cause, condition, baseline or traversal, computes no readiness, and writes
nothing except through the NIT-205 operations, whose locking, digest and
refusal rules are unchanged (docs/graph-safety/nit-192-briefs.md).

- Candidate: the head of `work/nit-198-viewer-comment-threads-on-briefs`, cut
  from `2bde5e52` (the NIT-178 combo branch after NIT-205). The figures below
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

## Preserved invariants

- **A claim never learns about a brief's threads.** No claim-side package
  changed:

      git diff --stat 2bde5e52 -- internal/lock internal/readiness \
        internal/catalog internal/model internal/lint internal/manifest \
        internal/loader internal/reaudit internal/briefs internal/comments \
        internal/digest                                           # empty

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
