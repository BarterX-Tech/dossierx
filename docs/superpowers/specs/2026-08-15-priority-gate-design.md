# Giving findings a Priority — reach and consequence, not free-text severity

**Status:** design, approved 15 Aug 2026 — **corrected in place, 15 Aug 2026,** against a fix-wave
reading of the implementation, each disagreement noted rather than silently rewritten. Three
mechanisms built after this design was approved are recorded as amendments at the end rather than
worked back into the numbered mechanisms above.
**Applies to:** v0.5.2, in flight — branch `release/v0.5.2`.
**Predecessor:** `docs/superpowers/specs/2026-08-14-gate-cycle-reduction-design.md` (lanes A–C, cycle
reduction). That design shortened the loop between a fix wave and the next round. This design does
not touch the loop; it changes what a finding, once raised, is worth.
**This is not a release procedure.** `docs/RELEASING.md` is the only one, and stays so.

## What this is for

Six reading rounds on v0.5.2 returned 39, 31, 24, 18, 58, 49 findings. Every one of those 219
findings is blocking by construction — `CLAUDE.md`'s rule that the receipt evaluates FAILED on any
finding at all, with no severity threshold consulted anywhere. Underneath that flat gate, the subset
of findings that would actually reach a user stayed roughly flat too: about 4 to 6 per round, the
rest being coverage gaps and defects in surfaces nothing downstream of the maintainer reads.

Round six is the sharpest evidence that free-text `Severity` is not doing its job. Nine agents used
nine different severity vocabularies that round, because the field is a string an agent writes about
its own work, read only by `gateRecordReceipt`'s sort comparator — never compared, never thresholded,
never used to decide anything. A word with no reader behind it drifts, and by round six it had
drifted nine ways at once.

The curve is not a coverage problem — lanes A–C already hold coverage fixed. It is a triage problem:
every finding costs the same round of human reading regardless of whether it can reach a user
tomorrow or only a maintainer next quarter. This design gives findings a computed Priority so reading
time goes where reach and consequence say it should, without filtering, thresholding away, or waving
through anything the maintainer has not seen.

## What does not change

- **Every finding still reaches the human, and nothing is filtered or deduplicated.** Priority sorts
  and gates the build; it never removes a finding from the record.
- **Coverage never narrows.** `reach_class` labels surfaces that already exist in `surfaces.yaml`; it
  exempts none of them from being read.
- **The gate surfaces and never fixes.** Nothing here grants a reading agent write access; Priority is
  computed by the recorder, not by an agent.
- **A skip is still a failure.** A finding the recorder cannot classify is refused, not defaulted to a
  low Priority — an unclassifiable finding is the "skipped check that reads as a pass" `CLAUDE.md`
  names, and refusing it is what keeps it from reading as fine.
- **P2/P3 findings still appear on the receipt.** They stop blocking the build; they do not stop
  existing on the page the human reads.

---

## Mechanism 1 — `reach_class`, machine-supplied per surface

### What it does

`surfaces.yaml` gains a `reach_class` on every declared surface: `client-shipped`, `consumer-docs`,
`maintainer`, or `process`. Set when the surface is declared, from what the surface *is* — a binary
artifact, a doc a consumer reads, an internal test, a release procedure — never chosen by an agent at
read time. An agent answering a surface reads its `reach_class`; it does not assign one. Same shape
as the predecessor design's freeze: a fact about the subject, fixed once, outside the loop that
reasons about findings against it.

### What it does not fix

Nothing about any individual finding's consequence. A `client-shipped` surface can still hold a
cosmetic finding; a `process` surface can still hold one that badly misleads a maintainer. Reach sets
the ceiling a surface's findings can reach, not where any one finding lands under it.

---

## Mechanism 2 — `consequence` and a mandatory `failure_scenario`

### What it does

Every new finding carries `consequence` — `acts-wrongly`, `misled`, or `cosmetic` — plus a mandatory,
falsifiable `failure_scenario`: a sentence naming who is reading or running the artifact and what
goes wrong for them. Free-text `Severity` is retired for new answers; historical answers keep their
existing string untouched. **This last clause overstates it: the field is gone from the schema, not
merely retired for new writes, so an old answer read back today carries no `consequence` at all and
blocks as unranked rather than keeping its `Severity` string — which is moot in practice, since the
fan-out already refuses to start while a previous round's answers still sit in `gate/answers/`, so no
historical answer actually survives to be read back this way.** `failure_scenario` exists because
`consequence` alone is exactly as
gameable as `Severity` was — three words drift as freely as nine unless something forces a checkable
claim. A scenario naming a reader and a wrong outcome can be disputed; a bare label cannot.

**One recorded ruling.** The predecessor design's round-one class — "an agent naming a byte it needed
and was not handed" — classifies `misled`, not `acts-wrongly`: the scenario is framed on the
verdict's reader, told a surface passed when the agent answering it was missing information the
answer depended on. That is a claim about being misled, not about software acting wrongly. On a
`client-shipped` surface this yields P1.

### What it does not fix

It does not make a scenario true. Nothing checks a `failure_scenario` at write time. Disputing a
wrong scenario is a human's job, same as disputing a wrong finding always was.

---

## Mechanism 3 — the recorder computes Priority from the matrix

`consequence` × `reach_class` is a fixed matrix, computed by the recorder, never chosen by an agent:

| | client-shipped | consumer-docs | maintainer | process |
|---|---|---|---|---|
| **acts-wrongly** | P0 | P1 | P2 | P2 |
| **misled** | P1 | P2 | P3 | P3 |
| **cosmetic** | P2 | P3 | P3 | P3 |

A finding missing either input, or carrying a value outside the closed vocabulary, is refused rather
than assigned a default cell — the same posture `gateStage3` takes toward an unparseable answer.

**What it does not fix:** the matrix is a default, not a ceiling — see Safety below — and it does not
resolve disagreement about which cell a finding belongs in; a human can promote any finding regardless
of what it computed.

---

## Mechanism 4 — `evaluate()` reads Priority; P0 is non-overridable

`evaluate()` passes when no P0 exists and every P1 is fixed or ruled through `gate/overrides.json`.
P0 has no override path: a client-shipped defect that makes a follower act wrongly is not waveable by
signature, because the artifact is already wrong in the reader's hands the moment it ships. P2/P3
findings never block, and are written to a tracked `gate/deferred.json`, which the *next* release's
round one reads as input — the predecessor design's "recorded as a finding against the next release"
shape, applied here to a finding whose Priority says it can wait.

**What it does not fix:** the general override record the predecessor design left deferred.
`overrides.json` here is narrower — it clears a P1 and nothing else. **That undersold what shipped:
an entry naming a P0 finding or one that is unranked is refused outright, but a ruling on P1, P2 or
P3 is accepted — a P2 or P3 ruling simply clears nothing that was blocking to begin with, since
neither ever blocks `evaluate()`.** Whether a fuller mechanism is
ever built remains the open question `gate_stage3_test.go` already recorded.

---

## Mechanism 5 — the frame hunts reader harm first, sweep second

`gate/prompts/_frame.md`'s question is reframed: an agent's first pass asks what would misinform or
mis-serve the reader who depends on this surface, and only then sweeps for everything else.
Report-every-finding is unchanged — a low-Priority finding found on the sweep pass is still written
down in full. What moves is which question is answered first, not what may be reported.

**What it does not fix:** it shrinks no bundle and shortens no round. It changes the order in which
one agent, inside one round, looks for what it looks for.

---

## What was rejected, and why

**The evidence-derived classifier**, recorded as deliberately unbuilt in `gate_stage3_test.go`'s
residues note.
It proposed classifying findings from evidence rather than self-report, and died on an unsatisfiable
bar: file:line in code plus the contradicting prose span, when `surface.json` carries no source path
and no line number anywhere — every finding would classify UNSUPPORTED by construction. This design
does not resurrect that bar. `reach_class` comes from the manifest, which already exists; `consequence`
comes from a stated `failure_scenario`, which asks the agent to commit to a claim rather than asking
the bundle for evidence it cannot supply. Neither input needs anything the bundles do not already
carry.

## Safety

Nothing is filtered — every finding, at every Priority, is in the record a human reads. The matrix is
a default, never a ceiling: a human can promote any finding to any Priority, including P0, regardless
of what `reach_class` and `consequence` computed. The honest risk is a true P0 misfiled as P2 and
shipping unnoticed, mitigated three ways: the unfiltered list puts it on the receipt either way; the
mandatory scenario states in plain words what goes wrong and for whom, which is what lets a human
catch a misfiling; and promotion is a one-line ruling, not a rebuild — **built as `promote_to` on a
`gate/overrides.json` entry; see the Amendments below.**

## Verification

| Mechanism | How it is proven |
|---|---|
| 1 | Every surface in `surfaces.yaml` declares a `reach_class` from the closed four-value set; no reading agent's output can set or change one. |
| 2 | An answer with an empty `failure_scenario` or an out-of-vocabulary `consequence` is refused, not recorded. |
| 3 | Table-driven test over all twelve cells asserting the Priority each pair produces; a missing or out-of-vocabulary input is refused, not defaulted. |
| 4 | A P0 anywhere fails `evaluate()` regardless of `overrides.json`. A cleared P1 passes; an uncleared one fails. P2/P3 never affect `evaluate()` unless a ruling promoted one; otherwise they appear in `gate/deferred.json` and the receipt without blocking. |
| 5 | `_frame.md` orders the reader-harm question before the sweep question, and the report-every-finding instruction is still present, restated to cover both passes. |

## Risks

- **A true P0 misfiled lower.** Mitigated by the safety property above: nothing hidden, the scenario
  is auditable, promotion is cheap.
- **`failure_scenario` filled mechanically.** Falsifiable by design, but nothing checks it at write
  time — caught by a human reader, same as a wrong finding always was.
- **`reach_class` drifts from what a surface actually is.** Set once, by a human, on the same manifest
  that already answers every other question about a surface — corrected the same way, by editing it.
  An edit re-ranks nothing already recorded, though: only findings recorded after the edit see the
  new class, so a drift caught late still leaves the earlier findings at their original Priority.
- **Narrowing fix pressure before ship, even though nothing is filtered from the record.** A
  deliberate trade of immediate fix pressure for reading focus. If wrong for a release, the remedy is
  the same one-line promotion, not a change to the matrix.

## Amendments — 15 Aug 2026

Three mechanisms this design left as open questions or informal remedies are now being built. Each
is briefly described here rather than folded back into the numbered mechanisms above, which record
what was approved on 15 Aug, not what has shipped since.

**PROMOTION.** `gate/overrides.json` entries may carry `promote_to`. A promoting entry raises a
finding's effective Priority — P2 to P0, for instance — and is refused if it names anything at or
below what the matrix already computed: promotion only ever goes up. It keeps the finding on the
receipt and clears nothing, and a finding promoted to P0 blocks non-overridably, the same as any
other P0. This is the mechanism the "matrix is a floor, not a ceiling" language above was already
promising; it now has a field.

**ABOUT.** A finding may carry `about`, a repo-relative path the finding's substance actually
concerns — set when the defect lives in a file the agent read as borrowed context rather than in its
own surface's document. The recorder resolves the path to its owning surface and ranks the finding
at the HIGHER of the reading surface's `reach_class` and the owner's; an `about` that resolves to no
declared surface — including every OUT-OF-SCOPE file, which carries no `reach_class` at all — is
refused, and a finding resting on one of those has no lever and still ranks at the reading surface's
own class. This is the mitigation `gate_answer_test.go`'s surface-attribution comment names — not the
gap Mechanism 1's "What it does not fix" left open, which is about a finding's consequence varying
WITHIN one surface and is untouched by this field. The defect `about` addresses is different: a
`maintainer` surface reporting an `acts-wrongly` defect that actually lives in a `client-shipped`
file it only borrows used to rank at the maintainer's own P2 ceiling; it now ranks at the
client-shipped P0 the defect actually costs.

**PROJECTOR.** After a round's answers are recorded, the maintainer runs
`go test ./cmd/dossierx -run '^TestGateDeferredProject$' -count=1 -args -deferred-project`, which
reads the round's recorded answers and writes `gate/deferred.json` from them — refusing to run over
an incomplete round. The driver's own D1 write is a re-projection of the same comparator, so what D1
checks and what the maintainer committed agree by construction. The maintainer then commits the
ledger, subject to `subject.json`'s own caveat that committing moves the tree.
