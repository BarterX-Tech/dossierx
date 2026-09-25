# Approval policy and dependency readiness

DossierX separates two questions that used to be read as one:

* **Local approval:** has a human approved this claim's exact current content?
* **Dependency readiness:** can that approved claim be used with the current,
  approved and review-clear `rests_on` chain it consumes?

A claim may be locally approved while its required dependency is still a
readable draft. That is an honest conditional approval. The claim remains
unready for integrated use until the dependency is approved and its chain is
current. A missing, unreadable, retired, cyclic, or historically unknown
required dependency keeps the chain unready. The first four also refuse a new
local approval: only a readable draft can receive a conditional approval. A
content change in a dependency
creates review on the direct consumer and an inherited review cause on every
downstream consumer, with the full path shown on that consumer.

`rests_on` is the only drift input; the baseline set and the required chain
are the same set. Hashes
detect comparable content changes; they do not decide semantic compatibility. Existing lint,
integrity, comment, and human-review gates still apply.

## Preview and approval

One lock command evaluates a set of claims. A set containing one claim and a
set containing several claims use the same policy and candidate-state
evaluator.

Preview one claim:

```text
dossierx claim lock widget.contract.boundary --dry-run
```

Preview a group:

```text
dossierx claim lock widget.contract.boundary widget.contract.consumer --dry-run
```

The group preview evaluates the final candidate state without writing. It
returns every requested claim's local verdict, refusal reasons, dependency
conditions and a snapshot token. Adding an unrelated claim cannot change an
otherwise admissible claim's local verdict. A valid claim alongside an invalid
member is still a refused batch, and the batch writes no claim, approval,
ledger, baseline, or receipt.

After reviewing the preview, approve the same set with the human's reason and
the returned snapshot:

```text
dossierx claim lock widget.contract.boundary widget.contract.consumer \
  --reason "Reviewed the boundaries and their current assumptions" \
  --proposal "<snapshot returned by --dry-run>"
```

The snapshot binds the canonical requested id set and reviewed dependency
closure. Missing, malformed, stale, or wrong-set tokens refuse before approval
storage changes. Re-run the preview and review the new dependency text; the
writer never substitutes unseen content into an approval. A write failure must leave the previous state
or an explicitly recoverable state, never a successful-looking partial batch.

When a reviewer or upstream analysis has identified an actual semantic
contradiction, pass it explicitly as `--semantic-conflict
"claim-id=dependency-id=reason"` on preview and write. The evaluator records a
`semantic_contradiction_requires_human_review` refusal. It never infers this
from a hash, and refreshing a snapshot cannot clear it; a human must review the
stated conflict.

Under the local-approval policy, approving `consumer` against a readable draft
`boundary` can succeed locally while reporting `dependency_unapproved:
boundary`. Approving both in one final candidate state can clear that condition
because `boundary` is then actually approved. Group membership does not make
`consumer` locally admissible; it only changes the final dependency state.

## Causes and paths

Readiness is derived when it is consumed. It does not trust a stale saved
`review_pending` bit or a missed watcher event.

Review causes remain independent:

* `direct_dependency_change` means a directly compared dependency differs from
  the baseline reviewed for this claim.
* `upstream_dependency_review` means a required prerequisite has an active
  review cause, including an open thread or claim flag with unchanged wording.
  The path identifies every boundary, for example
  `consumer -> boundary -> foundation`. When multiple routes reach the same
  upstream review boundary, readiness emits a deterministic representative path
  (the shortest path with a lexicographical tie-break) as diagnostic witness
  evidence rather than enumerating every route.
* `unapproved_edit` means this claim held an approval, that approval was
  released by an honest `claim unlock`, and the wording has since moved away
  from what was approved. It is the only cause a claim that is not locked owns
  on its own content, and it is deliberately distinct from
  `approval_content_drift`: drift names a claim that is still `locked` whose
  bytes no longer match a STANDING approval, which is the integrity finding the
  ledger gate refuses on. This names the ordinary, intended act — unlock,
  rewrite, re-lock — observed part-way through. A draft holding an UNRELEASED
  record is neither: that is the `lock-ledger-orphan` tamper finding, and this
  cause stays silent on it.
* A claim's own open thread or flag remains its own direct cause. Clearing one
  inherited path does not clear another dependency path, a direct change, or
  the claim's own cause.

If `foundation` changes beneath `boundary`, the boundary receives a direct
cause and the consumer receives an inherited cause with the nested path even
when the boundary's wording is unchanged. If the boundary is reviewed and its
boundary remains compatible, only that satisfied inherited path clears. If the
boundary itself changes, the consumer receives a direct dependency-change
cause and must review the changed boundary.

Unlocking an unchanged dependency produces an unapproved-dependency condition,
not semantic drift. Missing, unreadable, or retired inputs retain the last
reviewed receipt for explanation while withholding readiness. A cycle is
reported with its cycle path (minimizing the complete witness path from the
evaluated claim through the cycle) and cannot be approved as a required premise.
Retired and unreadable prerequisites are terminal blockers. Their internal causes
and cycles are not inherited through that route. Cycle components use only
readable claims, so an invalid bridge cannot merge independent cycles. A claim's
own assessment still inspects its direct dependencies even when that claim is
invalid; a cycle cannot re-enter an invalid claim as a prerequisite. Alternate
readable routes remain eligible.

Across all condition and cause categories, readiness emits one entry per distinct
underlying obstacle with its deterministic representative witness path. Let $V$
be claims, $E_r$ required edges, $E_b$ baseline/drift edges, and $D$ witness depth.
BFS visits $O(V + E_r)$ nodes and edges per evaluated claim. Its existing-node
path storage and copying cost $O(VD)$ per claim, or $O(V^2D)$ across all claims.
Parent-path comparisons can cost $O(E_rD)$ per claim. Missing-target and emitted
fact witnesses add $O(RD)$ storage, where $R = O(V + E_b)$ is the number of
independent records per assessment, including multiple cause kinds per owner.
Adjacency and record sorting, baseline checks and content hashing, and cached
cycle searches add separate work. Each cycle entry is searched at most once;
cycle searches and complete-witness comparisons remain polynomial in nodes,
edges and depth, independent of the number of routes.

The whole readiness projection holds $O(VRD)$ identifier slots, including the
depth multiplier in explicit witnesses. Actual catalog bytes also depend on
identifier/detail lengths and duplicated aliases; source claims and viewer
markup add further bytes. This eliminates exponential route enumeration, but
does not guarantee linear work, memory, or serialized output.

## Existing projects

Local-approval policy v1 is the only policy. A store that records an older
policy, or predates the field, loads as v1 and its next write records the
carry-over (`policy_version: 1`, `policy_migrated_at`,
`policy_migration_reason`). No approval it holds is reinterpreted.

Existing approvals, dependency baselines, receipts and review causes stay as
recorded. Loading a newer binary does not lock or unlock claims, refresh a
baseline, or make a historical approval mean that an unseen draft dependency
was approved.

## What was approved, and what is written now

A lock-ledger record carries the claim it approved, not only the hash of it.
The hash answers whether what is written still matches what was approved; the
retained content is what lets a consumer answer the question a reviewer asks
next, which is what changed. The text survives `claim unlock` for exactly the
window in which it is needed — between the unlock and the next lock, when the
current wording is drifting away from it — and the next approval replaces it.

A record written before this content was retained carries a hash and no text.
That case is reported as such. A consumer must not treat a missing approved
text as an empty one: diffing against nothing renders the whole claim as newly
added, which is a confident wrong answer to a question the record cannot
answer.

### Recovering wording a record never kept

`dossierx claim recover-approved-content` completes those older records from
the project's own git history. It is a one-time migration, run deliberately,
and it is not part of `check`.

The search is settled by the hash, never by a date or a commit message. A
record already signs `LockedClaimHash` of the claim as approved, so a revision
whose `LockedClaimHash` equals that hash IS the approved claim, byte for byte
over every persisted field. Two consequences follow, and both are load-bearing:

* **It cannot widen an approval.** The only content it can store is content the
  record's existing hash already certifies. The write goes through
  `lock.Store.RetainApprovedContent`, which re-checks the hash itself and
  refuses otherwise (`lock.ErrContentMismatch`), so no caller can substitute
  different words behind an existing approval however it obtained them. Nothing
  else on the record moves: not the hash, the approval's time, actor or reason,
  nor the release fields. A record that already carries its wording is never
  replaced — the engine's own record is the better evidence.
* **It cannot fail upward.** A claim whose history holds no matching revision —
  a corpus imported without history, an approval taken over an uncommitted
  working tree, a rewritten history — is reported by name and left alone, and
  its panel goes on saying the wording was not retained. There is no nearest
  match, and a walk that stops early discloses that it did rather than
  reporting that nothing was found.

Approvals taken after the ledger began retaining content need none of this, so
the verb has nothing to do on a corpus it has already swept.

### What moved, when the wording did not

A claim's hash covers every field it persists, so a claim can be honestly
edited since its approval with its prose untouched: its embodiment retargeted,
an edge added, an audit note written. `lock.SignedFieldsDiffering` names those
fields and `lock.SignedFieldValue` reads them, both over the same reflection
and the same exclusion list the hash itself uses, so a field added to the schema
tomorrow is covered without a second hand-kept list. A consumer showing a
reader what changed must show these too: naming a field without showing it
reports a change the reader has no way to see.

### One predicate

Whether a claim is edited since its approval is `lock.EditedSinceApproval`, and
it has one implementation. Readiness raises `unapproved_edit` from it, the
viewer's panel is built from it, and recovery selects its candidates with it.
Separate copies would agree only by inspection, and both failure modes are
silent: a row on the Issues screen with no panel under it, or a panel for a
claim nothing reported.

## What readiness does not prove

The number of locked claims, a complete track, passing fixture tests, or
a green local approval count is not an integrated-readiness certificate. The
relevant dependency chain must be approved, current, and clear of active
review, and the project still needs the implementation and independent
evidence required by its acceptance contract. Consumers should show local
approval, dependency conditions, review causes and paths together so a locked
claim cannot appear ready merely because its own status says `locked`.

## Admitted evidence

Approval evidence is what the engine records or what a human does, and nothing
else: a `claim lock` written with the human's `--reason` and the snapshot they
previewed, a confirmed `claim reaudit`, a human's Resolve click on a thread, a
`dossierx check` that exited 0, a lint, ledger or conformance finding, and the
code-link coverage the scan reports. An agent's statement in chat that the code
matches a claim, or that a claim is synced, is not evidence and is never a
certificate: it records no approval, clears no review cause, and closes no
loop. When the evidence for a step is missing, the agent stops and names what
is missing. A model may draft the claim, the tag and the code; it never judges
whether the claim holds.
