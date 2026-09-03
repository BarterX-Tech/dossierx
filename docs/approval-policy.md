# Approval policy and dependency readiness

DossierX separates two questions that used to be read as one:

* **Local approval:** has a human approved this claim's exact current content?
* **Dependency readiness:** can that approved claim be used with the current,
  approved and review-clear `rests_on` chain it consumes?

A claim may be locally approved while its required dependency is still a
readable draft. That is an honest conditional approval. The claim remains
unready for integrated use until the dependency is approved and its chain is
current. A missing, unreadable, retired, cyclic, or historically unknown
required dependency keeps the chain unready. A content change in a dependency
creates review on the direct consumer and an inherited review cause on every
downstream consumer, with the full path shown on that consumer.

`governed_by` remains a drift input. A change to its claim-valued target can
create a direct dependency review cause, but its approval status does not by
itself become a `rests_on` approval prerequisite. Hashes detect comparable
content changes; they do not decide semantic compatibility. Existing lint,
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
  `consumer -> boundary -> foundation`.
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
reported with its cycle path and cannot be approved as a required premise.

## Existing projects

A missing lock store belongs to a new project and defaults to local-approval
policy v1. An existing store without explicit policy adoption remains on the
legacy policy; loading a newer binary does not reinterpret its old approvals,
refresh baselines, or clear review causes.

Adopt v1 explicitly after reviewing the migration:

```text
dossierx claim migrate-lock-policy --dry-run
dossierx claim migrate-lock-policy \
  --reason "Reviewed the local approval and dependency readiness policy"
```

Migration preserves existing approvals, dependency baselines, receipts and
review causes. It does not lock or unlock claims, refresh a baseline, or make a
historical approval mean that an unseen draft dependency was approved.

## What readiness does not prove

The number of locked claims, a complete build order, passing fixture tests, or
a green local approval count is not an integrated-readiness certificate. The
relevant dependency chain must be approved, current, and clear of active
review, and the project still needs the implementation and independent
evidence required by its acceptance contract. Consumers should show local
approval, dependency conditions, review causes and paths together so a locked
claim cannot appear ready merely because its own status says `locked`.
