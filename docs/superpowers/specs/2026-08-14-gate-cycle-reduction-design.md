# Cutting release-gate cycles — three changes, inside v0.5.2

**Status:** design, approved 14 Aug 2026 — **SUPERSEDED IN PART BY WHAT SHIPPED, 14 Aug 2026.**
This is a design record and is kept as one: it says what was intended and why, which is the part
worth keeping. It is not a description of the tree. Where the two disagree, the tree is right, and
each disagreement found by the fix-wave reading of the implementation is corrected in place below
with a note rather than silently rewritten — a design record that quietly matches its
implementation records nothing about the decisions made along the way.
**For what shipped, read `docs/RELEASING.md` and these three files:
`cmd/dossierx/gate_subject_test.go` (lane B), `cmd/dossierx/gate_wave_test.go` (lane A) and
`tests/derived_facts_test.go` (lane C).**
**Applies to:** v0.5.2, in flight — branch `release/v0.5.2` at `f38ace0`, gate paused after round four.
**Scope ruled by the maintainer:** lanes A, B and C. The override record is deferred; see below.
**This is not a release procedure.** `docs/RELEASING.md` is the only one, and stays so. This file is
a design record of why three mechanisms exist and what they promise.

## What this is for

The v0.5.2 gate has run four reading rounds: 39 → 31 → 24 → 18 findings. The curve decays and does
not converge, and the reason is visible in the fix waves' own commit messages rather than inferred.

| Round | Findings | What the wave recorded about their origin |
|------:|---------:|-------------------------------------------|
| 1 | 39 | 20 were bundle-scope gaps — "an agent naming a byte it needed and was not handed" |
| 2 | 31 | 4 were regressions introduced by round one's fixes; 8 more `reads:` declarations added |
| 3 | 24 | a block titled "MINE, FROM ROUND TWO" — four more fixer regressions |
| 4 | 18 | "three of these are high severity and all three are mine"; `surfaces.yaml` gained two paths |

Three defects follow from that table.

1. **Nothing reads a fix wave before a full round pays to discover it.** Every round since the second
   opens by repairing the round before it.
2. **The subject moves under the curve.** `surfaces.yaml` grew in rounds 1, 2 and 4. Round four's own
   message says the widening "is what let round four adjudicate the retired-set question at all", so
   part of each round's count is new coverage rather than new defects. v0.5.1 went 48 → 27 → **34**
   the same way.
3. **A class of finding regenerates every release by construction** — a hand-typed number or path
   duplicating something the tree already derives. `docs/RELEASING.md`'s pin list has gone stale
   twice inside this one release.

The three lanes below address one defect each.

## What does not change

Stated first because every lane is constrained by it, and because a change that quietly relaxed one
of these would be worse than the cycles it saves.

- **Coverage never narrows.** Lane B holds coverage where round one set it and writes down what it
  defers. It does not sample, truncate, or drop a surface.
- **A skip is still a failure.** Lane C's checks fail the build on mismatch, and a check that cannot
  execute fails rather than passing over zero assertions.
- **Every finding still reaches the human, and nothing is filtered or deduplicated.** Lane A's
  readers add findings; they remove none.
- **The gate surfaces and never fixes.** Lane A's readers are report-only, on the same exclusive
  two-tool grant as every other reading agent.
- **The receipt still evaluates FAILED on any finding.** That is what the deferred override record
  would have changed, and it is not being changed.

---

## Lane A — read the fix wave before spending a round on it

### The mechanism

A new mode in `scripts/gate-stage2/run.sh`, which carried seven when this was written —
`surfaces`, `grant`, `model`, `command`, `fanout`, `delta`, `record` — and carries nine now that
`subject` and `wave` have both landed. Invoked with the range a fix wave produced:

```
run.sh wave --range <base>..<head>
```

*(Shipped with `--range` rather than a positional argument. The spelling above originally omitted
it, which would have failed as a usage error for anyone who typed it.)*

It hands **two** report-only agents one wave bundle: the full text of every changed file, the diff
for the range, and `gate/prompts/_wave.md`.

*(This paragraph originally said the mode maps changed paths onto declared surfaces. It deliberately
does not, and the shipped code says so in capitals: `surfaces.yaml`'s path matching has exactly one
implementation, in the Go producer, and a second one in shell would be free to disagree with it.)*

The wave prompt asks a narrower question than a surface prompt does — *did this wave introduce a
statement that is false about the tree it just changed?* — because that is the only question a diff
can answer.

Full file text, not only hunks: a sentence introduced by a wave is false or true against the
paragraph around it, and a hunk hides that paragraph.

### The safety property, which is the whole design

**A wave answer is never a surface answer.** It is not written to `gate/answers/`, it is not keyed to
a tree, it never reaches `gateStage3`, and it cannot contribute to a receipt.

The reason is the stage-3 invariant: one answer per declared surface, fresh or carried, keyed to the
tree. A wave read is keyed to a *range*. Admitting it would let a narrow read stand where a full
bundle read is required — which is the "skipped check that reads as a pass" the gate exists to
refuse. Its clean result means **"no regression found in this diff"** and never "this surface passes".

### Failure handling

The wave read is a step in the fix procedure, not a stage of the gate. It produces no verdict, so it
cannot produce a false green. If it cannot run — no range, missing hasher, refused grant — it exits
non-zero and the wave is not done. `docs/RELEASING.md` gains that step; nothing else moves.

### What it does not fix

Anything the wave did not touch. It shortens the loop; it does not replace a round.

---

## Lane B — freeze the subject for the length of a release

### The mechanism

A tracked file, `gate/subject.json`:

```json
{
  "version": "v0.5.2",
  "frozen_sha256": "<sha256 of surfaces.yaml at the first fan-out — never edited>",
  "surfaces_sha256": "<the digest currently accepted; a thaw moves this one>",
  "frozen_at_run": "<run id of round one>",
  "thaw_reason": ""
}
```

*(`frozen_sha256` was missing from this sketch. It is the field the whole thaw rule is defined on —
the two digests disagreeing is what says a release re-opened its subject.)*

The first fan-out of a release writes it. Every later fan-out for that version recomputes
`surfaces.yaml`'s digest and **refuses** on a mismatch, naming both digests and the file. A new version — read from
`CHANGELOG.md`'s newest heading, which is one of the two sources `D1` derives from, and enough to
tell one release from the next — starts a new freeze automatically, so
nothing has to be reset by hand between releases.

`gate/.gitignore` ignores everything it does not name, so this file must be added to its allow-list
or it will be invisible to git, which is the wrong property for something a human is meant to review.

### Thawing is deliberate and recorded

A coverage gap found mid-release is recorded as a finding **against the next release**, not acted on
silently. If the maintainer rules a specific gap blocking, they edit `gate/subject.json` with a
non-empty `thaw_reason` and the new digest. **This originally said every key changes when `surfaces.yaml` does, so a thaw re-reads every
surface. That is false**, and the fix-wave reading of the implementation caught it in five places.
The correction was then wrong in the other direction — "no stage-2 key hashes the manifest" — and a
second fix-wave reading caught that too. What holds: a key hashes five things, including the
ASSEMBLED BUNDLE, and `contributing` declares the manifest in its `reads:`, so the manifest reaches
that surface's bundle and its key moves whenever the manifest does. A thaw moves `contributing`'s
key always, plus every surface whose documents or `reads:` list the edit changed. The freeze's value is that the widening is visible, dated and reasoned — not that it is
expensive.

### Freezing is not narrowing

Coverage stays exactly where round one set it. Nothing is dropped, and the deferral is written down
where the next release reads it. `docs/RELEASING.md` gains a paragraph saying so explicitly, because
the distinction is load-bearing against `CLAUDE.md`'s rule and a reader should not have to derive it.

### What it does not fix

Nothing about finding quality. It only makes the round-over-round curve measure a fixed subject.

---

## Lane C — derive the two facts that keep going stale

`surface.json` is a mechanically derived inventory, regenerated by `TestGenerateSurfaceJSON` and
compared byte for byte in CI, so a check may read it as trusted. Two assertions in prose duplicate
what it already carries, and both have failed inside this release.

### C1 — the site's retired table against `surface.json`

`surface.json` lists 16 retired commands. `site/src/content.ts`'s `migration.rows` enumerates 12:
`lock`, `unlock`, `reaudit` and `flag` are missing, and `git show v0.2.0:cmd/dossierx/main.go`
confirms the first three were real top-level commands that were genuinely cut.

The check asserts set equality in both directions — an entry on the site that the binary does not
retire is as wrong as a retired command the site omits. Home: it shipped in a new file, `tests/derived_facts_test.go`, rather than in
`tests/docs_site_audit_test.go` as planned here — the reasoning about why prose that restates a
derived number is its own class wanted a header of its own.

### C2 — the pin-count prose against `version_pins`

`docs/RELEASING.md` states "FIVE pins across FOUR files". `surface.json`'s `version_pins` carries the
five entries and their files. The check derives both numbers and asserts the prose matches, so the
sentence cannot go stale a third time.

### What it does not fix

Prose that makes a judgement rather than states a number — most of `FORMAT.md`. A derived check can
only compare what `surface.json` extracts.

---

## Deferred: the override record

`cmd/dossierx/gate_stage3_test.go:51-63` already records this as deliberately unbuilt and notes it
now has a routine caller. It stays unbuilt this release, by the maintainer's ruling.

**The consequence, stated plainly:** the receipt fails on any finding and there is no field for a
human ruling, so round five must return **zero** findings to end this release. If it returns any, a
round six follows. The only alternative remains deleting a finding by hand, which leaves an
adjudicated finding indistinguishable from one nobody raised.

When it is built it needs three properties, and they are the design work, not the field: harder to
write than a fix, visible on the receipt's face, and never inherited by the next release.

## The release must describe these changes

Adding gate work to v0.5.2 changes what the release is. `CHANGELOG.md`'s `[0.5.2]` entry and the
site's `releases[]` entry must both describe it, or the release is wrong about itself — which round
five would correctly report. This is new prose in the exact place prose keeps failing, so it is
written last, from the diff, and read by lane A before round five sees it.

## Verification

| Lane | How it is proven |
|------|------------------|
| A | A test that the wave mode writes nothing under `gate/answers/` and that no wave output reaches a receipt. A usage test pinning the mode's grant to the same exclusive two-tool list. |
| B | Shipped as seven test functions. The four planned assertions ship as three of them (refusing a changed digest and accepting a matching one are one test), plus: a later round not erasing a recorded thaw; the fan-out verifying before it mints; a freeze naming no run being refused without writing anything; and the unreasoned re-openings pinned OPEN. That last is a correction — this spec implied a thaw always costs a reason, and only an edit to `surfaces_sha256` does. |
| C | Mutation both ways — remove a row from the site table, and change the pin-count sentence; each must red the build. |

## Order of work

1. **Lane C** — mechanical, touches no gate contract.
2. **Lane B** — the freeze file and its refusal.
3. **Lane A** — the wave mode and its prompt.
4. **The four open findings** — their fix wave is the first thing lane A reads, which is the first
   real test of lane A rather than a claim about it.
5. **The release's own entries** — CHANGELOG and site, describing lanes A–C.
6. **Round five**, full thirteen surfaces, carrying forward only what the fingerprints match, with
   the tool-grant honoured-check actually performed rather than assumed.

## Risks

- **Lane A's clean result is easy to over-read.** Mitigated by never writing it where a verdict is
  read, and by saying what it means in the prompt and the docs.
- **Lane B can defer a real gap for one release.** Mitigated by the deferral being a recorded finding
  rather than a silence, and by the thaw path existing for the case where the maintainer disagrees.
- **Lane C is only as good as the extraction.** A derived check inherits `surface.json`'s blind
  spots; it does not invent coverage.

## One finding found while writing this

`cmd/dossierx/gate_stage3_test.go:64-69` stated that `scripts/gate-stage2/run.sh` "has six modes".
It had seven. Fixed in the lane-A commit, and the count is no longer recalled: a test now reads the
number out of that comment and checks it against the script, and checks the script's two lists —
what `usage` advertises and what `case` dispatches — against each other.
