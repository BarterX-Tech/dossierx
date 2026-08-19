# Releasing DossierX

A `v*` tag no longer publishes anything on its own. `.github/workflows/release.yml`
opens with a `gate` job that the publishing job `needs:`, so a tag that does not
get past it produces no archives at all. The gate establishes two facts about the
tagged **tree** rather than about whoever pushed: that the tagged commit is
reachable from `origin/main`, and that the tree at that commit carries the release
stamp for exactly this version — `site/src/content.ts`'s last `releases[]` entry
names the tag being pushed. Every exit path that is not a pass is a refusal; there
is deliberately no branch that reports "could not check" and exits 0. Only once
that job passes does GoReleaser run, building the six platform archives, stamping
`main.version` / `main.commit` / `main.date` via ldflags, generating the GitHub
release notes from Conventional Commit subjects, and publishing.

That file records one residual rather than describing it as fixed: the workflow
GitHub runs for a tag is the one in the tagged tree, so anyone with push rights can
weaken the gate and tag that commit. Nothing in this repository closes it — a check
cannot be its own enforcement — and only a forge-side tag protection rule can.

You do not push that tag by hand either. The irreversible half of a release is
`make release-publish`, a nine-step driver whose preconditions include the gate
below; the commands it runs are the driver's and you type none of them. What this
document is for is the half a program cannot do: reading the findings, ruling on
them, and the three post-publish checks that leave this repository entirely.

## Before tagging

- [ ] **`go test ./...` passes**, including the two suites it does not reach on
      its own — see [CONTRIBUTING.md](../CONTRIBUTING.md#the-two-suites-go-test--does-not-reach).

      It needs nothing but Go. The two checks with an external prerequisite — a
      browser and a `goreleaser` binary — live in `viewer-tests/`, which
      `go test ./...` does not descend into; the next item runs them.

      **A clean `go vet` is not a clean lint.** If you are running the checks
      locally rather than reading them off CI, run `golangci-lint run ./...` as
      well: `errcheck` with `check-blank` failed CI during a release after a
      clean local vet, and the two tools do not look for the same things. The
      lint job is not one of the suites `make ci-evidence` accounts for — it
      emits no `go test -json` account of anything — so nothing downstream
      recovers this for you.
- [ ] **The release build has been run, before the tag.** Every other check reads
      what the release build was *told* to do; this is the one that watches it do
      it. It **fails rather than skips** when either tool is unnamed, so supply
      both:

          go install github.com/goreleaser/goreleaser/v2@latest
          DOSSIERX_TEST_GORELEASER="$(go env GOPATH)/bin/goreleaser" \
          DOSSIERX_TEST_BROWSER=/path/to/chrome \
          make viewer-test

      `TestGoreleaserSnapshotBuildsSixArchivesAndStampsTheBinary` in
      `viewer-tests/` runs `goreleaser release --snapshot --clean` against a temp
      `dist`, then asserts the six archives exist under the names the
      **Verifying** section tells you to download, that `checksums.txt` lists all
      six, and that the snapshot binary reports the same version, commit and date
      that its own recorded `-ldflags` line names.
- [ ] **CI is green on `main`** — not on the branch, on the merge commit, and
      not on the strength of a conclusion.

      **A green badge is not the check, and neither is a green check run.** Every
      layer above the test binary reports a *conclusion*, and a conclusion is
      `success` over zero tests: a suite emptied by a `-run` selector exported at
      run time prints `ok <pkg> [no tests to run]` for every package, and the
      step, the job and the check run all conclude success over it. So run the
      machine half first:

          make ci-evidence DOSSIERX_GATE_CI_SHA=$(git log --merges -1 --format=%H main)

      **`git fetch` first — that command reads your local `main`.** The commit
      you name has to be an object this clone actually has, because the driver's
      D1 later resolves it locally to compare its tree against the tree being
      released; on a stale clone the pipe silently yields an older merge commit,
      and a sha pasted from the forge is not present at all. Either way the
      record is about the wrong commit and D1 refuses the release with "the
      CI-run evidence record … names no object carrying the tree being released
      … or a commit this clone has never fetched" — the refusal names this
      mistake, so read it as this mistake rather than as a broken gate.

      That fetches the CI run **for that exact sha** — never HEAD, because the
      `content.ts` commit stamp lands on `main` after the merge as a matter of
      routine — derives from `.github/workflows/ci.yml` which suites exist and in
      how many matrix instantiations, fetches each instantiation's log, and reads
      the `go test -json` account the suite step emits. It fails, rather than
      passing quietly, when a declared instantiation produced no account, when a
      package reported `PASS` having executed no test, when any test failed
      (including one a `continue-on-error` step forgave — it never reads a
      conclusion), and when the account cannot be attributed or fetched at all.
      It writes a verdict record to `DOSSIERX_GATE_CI_EVIDENCE_OUT`
      (`/tmp/dossierx-ci-run-evidence.json` by default) and refuses to exit 0
      without one, because `go test` exits 0 for a skip and for a selector that
      matches nothing — so a *silent* pass here would be indistinguishable from a
      gate that examined nothing. **Read the record and confirm it names the sha
      you are about to tag** and the instantiations you expect.

      GitHub's log retention is finite, so this will FAIL rather than pass when
      re-run against an older release. That is intended.

      **Then open the run**, because two things are still a person's:

      - the account is now a `-json` event stream, so the suite steps' logs are
        several thousand lines of JSON rather than one `ok <pkg> <time>` line per
        package. Do not try to skim it — that is what the record above is for.
        What is worth a glance is whether the run contains jobs the command did
        *not* account for: `hooks` runs a shell script and, on Windows, a
        Pester suite, neither of which emits an account the evidence command
        counts, so a smoke test that degenerated into asserting nothing still
        reaches you as a green conclusion.
      - the run exists and belongs to this commit at all. A workflow whose
        triggers or `paths:` filter stopped matching produces no run, and a
        commit with nothing to report reads as a commit with nothing wrong. The
        command above reports that as a failure; confirm you agree with what it
        found rather than assuming a clean exit means a run was there.
- [ ] **The stage-2 reading gate has been run for this tree, and it is green.**
      Everything above checks that the code does what it is supposed to. This is
      the item that checks whether the PROSE is still true: thirteen agents read
      the thirteen surfaces `surfaces.yaml` declares — the README, the CHANGELOG,
      this file, the skills, the site, the merge-gate template, the rest — and
      report what this release has made false about them. It is also the item the
      release driver's D1 refuses without, so a release that skips it does not
      reach the tag; it stops at a refusal naming a `gate/fanout.json` nobody
      produced.

      Every command below is keyed to ONE tree and ONE resolved baseline. Fix
      both first and pass them everywhere:

          ROOT=$(git rev-parse --show-toplevel)
          TREE=$(git rev-parse "HEAD^{tree}")
          PREV=${DOSSIERX_PREV_RELEASE_TAG:-$(git describe --tags --abbrev=0)}
          PREV_COMMIT=$(git rev-parse "$PREV^{commit}")

      Both are full 40-digit object names and every step refuses anything else. A
      tag NAME is a mutable pointer that `git tag -f` re-points, and an
      abbreviation is a prefix that means a different object in a different
      clone; either is an answer that stops being true later, and which release
      the comparison was against is the whole value of this gate.

      **1. Stage the run's evidence — each artifact by its producer, never by
      hand.** A bundle is assembled from five things: the surface's question
      (`gate/prompts/<surface>.md`), the surface's own files read out of the tree,
      the documents the surface's `reads:` list in `surfaces.yaml` borrows from
      other surfaces (handed over as context, marked "NOT yours to report on" —
      ownership and the duty to review stay with the surface that claims the
      file), the committed inventory `surface.json`, and the six uncommitted artifacts
      under `gate/` produced below. Only those six are staged here, and the
      difference is worth knowing rather than discovering: none of the six has a
      committed form (`gate/.gitignore` ignores every one), so whatever happens to
      be at those paths on the day of the run is what the agents read. `record`
      can hold three of the six to account — the delta by recomputing it, the
      two stamped captures by reading the tree each names on its own face — and
      the other three it can only digest, so a hand-written
      `gate/export-output.json` is recorded exactly as cleanly as a real one.
      Produce them:

          # the rendered site text, extracted from a real build in a real
          # browser and stamped with the tree that build was made from
          DOSSIERX_SITE_TEXT_OUT="$ROOT/gate/site-text.json" \
          DOSSIERX_SITE_TEXT_TREE="$TREE" \
          DOSSIERX_TEST_GORELEASER="$(go env GOPATH)/bin/goreleaser" \
          DOSSIERX_TEST_BROWSER=/path/to/chrome \
          make viewer-test

          # the cross-release render diff, read by two surfaces
          go test ./tests -run TestRenderDiffCapture_G1Capture -args \
            -render-diff-out="$ROOT/gate/render-diff.json" \
            -render-diff-baseline-commit="$PREV_COMMIT" \
            -render-diff-tree="$TREE"

          # what `dossierx skills export` actually writes into a project
          go test ./tests -run TestCaptureSkillsExport_G1Capture -args \
            -skills-export-capture-out="$ROOT/gate/export-output.json"

          # GoReleaser's release notes as this tree predicts them
          go test ./tests -run TestPredictReleaseNotesForRange_G1Capture -args \
            -release-notes-range="$PREV..HEAD" \
            -release-notes-predict-out="$ROOT/gate/release-notes-prediction.json"

          # the resolved baseline inventory, and the surface delta over it
          scripts/gate-stage2/run.sh delta \
            --baseline-ref "$PREV" --baseline-commit "$PREV_COMMIT" \
            --baseline-file "$ROOT/surface.baseline.json"

          # the run manifest: this tree, this baseline, these exact bytes
          scripts/gate-stage2/run.sh record --tree "$TREE" \
            --baseline-ref "$PREV" --baseline-commit "$PREV_COMMIT" \
            gate/baseline.json gate/delta.json gate/export-output.json \
            gate/release-notes-prediction.json gate/render-diff.json \
            gate/site-text.json

      **The output paths are absolute on purpose.** `go test` runs each test
      binary with its own package directory as the working directory, so a
      relative `gate/…` lands under `tests/` and the gate then looks for an
      artifact nobody produced.

      **The two `DOSSIERX_SITE_TEXT_*` variables imply each other, and the
      extraction fails loudly when only one is set.** `DOSSIERX_SITE_TEXT_TREE`
      is the same full 40-digit tree object name everything else in this run is
      keyed to — `$TREE`, never a tag and never an abbreviation; the producer
      refuses both. The extraction writes it into `gate/site-text.json` as the
      document's FIRST field, and `record` reads it back and refuses a capture
      stamped with any other tree. The stamp exists because this is the one
      artifact `record` cannot check by recomputing — an extraction needs this
      build and a real browser — and before it existed the capture named which
      node and npm built the site and nothing that named a RELEASE, so an
      extraction left on disk from the previous gate run recorded cleanly and
      was hashed into the `site` surface's key as this release's rendered DOM.
      A stale capture is now refused rather than hashed cleanly into a key.
      What the stamp cannot promise, exactly as `gate/render-diff.json`'s
      cannot: that the working tree the build read was clean at that identity —
      the extraction records the value it was handed, verbatim.

      **`delta` takes no `--tree`, and that is a decision rather than an
      omission.** The delta is a pure function of `surface.json` and the
      resolved baseline, and its bytes are hashed into every surface's key and
      assembled into every surface's bundle — so the tree stamp it used to
      carry moved on every commit, re-keyed all thirteen surfaces every time,
      and the carry-forward machinery never once fired: a one-character README
      fix re-ran thirteen reading agents. The freshness the stamp bought is
      bought the stronger way now — `record` re-derives the whole document
      from the same two files and refuses on any byte of disagreement, which
      catches every stale delta the stamp caught and also the hand-written one
      the stamp waved through whenever it happened to say the right tree. (The
      script's global parser still accepts `--tree`, so an invocation copied
      from an older revision of this file runs; the value is unused.)

      **`surface.json` reaches all thirteen agents and is deliberately not on
      that manifest.** It is the mechanical inventory every surface's prose is
      judged against — commands, flags, lint rules, error codes, the envelope,
      the counts, the version pins — so leaving it unmentioned here is how a
      maintainer comes to believe the evidence set is closed at six. It is not
      staged because staging it would attest less than what already holds:
      `cmd/dossierx/surface_test.go` is both halves of one contract, writing the
      file under `-regenerate-goldens` and, on every ordinary run, failing when
      the committed copy is not what the current tree emits. A `record` entry
      would attest the bytes this run read; the test attests that those bytes are
      this tree's, which is the question. So the staging step for it is
      `go test ./cmd/dossierx` being green on the commit being gated —
      regenerate it and commit the result if it is not:

          go test ./cmd/dossierx -run TestGenerateSurfaceJSON -regenerate-goldens

      The `gate/prompts/<surface>.md` files reach the agents too, and are
      likewise unstaged: they are tracked and reviewed (`gate/.gitignore` ignores
      only what a run produces), and they are the QUESTION rather than the
      evidence. Neither is thereby unwatched. Both are hashed into the surface
      key: `surface.json` is one of the three SHARED evidence files every
      surface's fingerprint covers — beside `gate/baseline.json` and
      `gate/delta.json`; the set is `gateSharedEvidence` in
      `cmd/dossierx/gate_fingerprint_test.go`, and this sentence restates it —
      and the prompt sources are hashed into `method_version`, which the same
      fingerprint hashes in beside them. Change a byte of any of these and
      every surface is re-read rather than carried forward.

      `gate/site-text.json` used to be the fourth member of that set and no
      longer is, because SHARED means read by EVERY agent and the rendered
      site text is read by the `site` agent alone. Folded into the shared set,
      every re-extraction of the site — every release, since the extraction is
      per-run evidence — re-keyed all thirteen surfaces. It now reaches
      exactly one key the way the other single-reader captures always have:
      the assembler hands the `site` surface its capture verbatim, so the
      bundle digest covers those bytes for the one surface that reads them and
      for no other — `TestGateStage2ACaptureReachesOneSurfaceKeyAndNoOther`
      holds that true. What the demotion buys is the cache working at all: a
      site change moves the `site` key and leaves the other twelve carrying
      forward, instead of re-keying every surface over twelve documents
      nothing touched.

      **`--baseline-file` names v0.5.0's committed inventory only while v0.5.0 is
      the previous release.** That release shipped before the surface emitter
      existed and carries no `surface.json` of its own, which is the only reason
      `surface.baseline.json` is in this tree. Every later release carries its
      own, and the baseline is then read out of the tag —
      `git show "$PREV:surface.json" > "$ROOT/gate/baseline-input.json"`, and
      that path is what `--baseline-file` gets. Falling back on the committed
      bootstrap because a tag could not be read is the one move never to make: in
      a shallow clone that failure reads character for character like an absent
      tag, and the delta would then span two releases — full, plausible, and
      handed to thirteen agents as the truth about the past.

      Skipping any of this does not produce a smaller gate, it produces a failed
      one. The freshness check refuses with "it was found on disk rather than
      produced, and found on disk is not produced", and `record` holds every
      guarded artifact to account before it will name one — in one of two ways.
      The two captures it cannot recompute, the render diff and the site
      extraction, are refused when the stamp on their own face names another
      tree or another baseline. `gate/delta.json` carries no stamp to read:
      `record` re-derives it outright from `surface.json` and
      `gate/baseline.json` and refuses on any byte of disagreement — a delta
      computed before a fix moved the inventory, one computed against another
      baseline, and one written by hand all fail the same comparison.

      **2. Produce the fan-out.**

          DOSSIERX_GATE_AGENT=<the agent runner> \
          scripts/gate-stage2/run.sh fanout --tree "$TREE"

      That mints this run's identifier; refuses to start while a previous run's
      answers are still sitting in `gate/answers/`, and tells you to delete them,
      because they would otherwise sit beside a fresh identifier looking like
      answers somebody gave THIS release; assembles one bundle per declared
      surface into `gate/bundles/<surface>.md`; writes `gate/fanout.json` LAST,
      so a record naming bundles that were never written cannot exist; and prints
      one invocation per surface on stdout. A surface whose bundle cannot be
      assembled fails the whole production — the fan-out is a refusal or it is
      whole, and it never shrinks to twelve. There is exactly one implementation
      of it, `TestGateFanoutProduce` in `cmd/dossierx/gate_fanout_test.go`; the
      shell wraps that and re-implements nothing.

      A `reads:` entry in `surfaces.yaml` that no longer resolves — the borrowed
      file moved or was deleted — is one of those whole-production refusals, and
      the fix is the manifest edit the refusal names, never deleting the entry
      to get past it: the entry exists because an agent once had to answer a
      question without that file, and deleting it re-creates that round. The
      borrowed bytes are inside the borrower's bundle and therefore inside its
      key, so editing a borrowed document re-runs the surfaces that declared it
      (and its owner, whose key covers it as a document) and no others.

      **3. Run the thirteen agents.** Run exactly the invocations `fanout`
      printed, one per surface, and change nothing about them. They are
      read-only: `gate/method.yaml` grants `SurfaceFinding` and `SurfaceVerdict`
      and nothing else, and the harness passes that as an exclusive allow-list —
      the assembled bundle is the whole evidence set, which is the property every
      key in this gate rests on. What comes back from each agent is its own three
      facts and nothing else, in one file:

          {"verdict": "PASS"|"FAILED", "findings": [...], "subjects": {...}}

      **The agent cannot write the answer file, so you record it.** An answer has
      to carry this run's identifier and the surface's CURRENT stage-2 key, and
      only the Go side computes that key. As each agent returns, record what it
      produced:

          go test ./cmd/dossierx -run '^TestGateAnswerRecord$' -count=1 -args \
            -answer-record -answer-surface=<surface> -answer-file=<payload.json>

      That reads `gate/fanout.json` for the run this checkout was fanned out
      under, refuses a surface `surfaces.yaml` does not declare, refuses a
      payload carrying anything beyond those three keys — a `fingerprint`
      written by the agent would otherwise be dropped in silence — holds every
      finding to the finding schema (`surface`, `rule`, `consequence`,
      `failure_scenario`, `blocking`, `detail`, optionally `about`; a
      `severity` from a runner ported from the old schema is refused by name,
      and so is an empty or adjective-only `failure_scenario` or any value
      outside a closed vocabulary), computes the key, and puts the assembled
      answer through the SAME validation the collection applies, so a
      malformed answer is refused here, in front of you, rather than at the
      end of the run. It then writes
      `gate/answers/<surface>.json`. `-count=1` is part of the invocation as
      belt and braces, not as the thing holding the belt: a replayed cache would
      print `ok (cached)`, write nothing, exit 0, and never reach the refusal
      below, which you would read as an answer that landed. Today `go test` will
      not cache this run in any case — it caches nothing whose command line
      carries a flag outside its own cacheable set, and everything after `-args`
      is outside it — but that is a promise the toolchain makes about itself, so
      the invocation states what the gate needs instead of relying on it.

      **It will not overwrite an answer.** One agent per surface per run is the
      whole shape of the run, so a second answer is not a correction: it is a
      second opinion replacing the first with nothing left on disk to say the
      first was ever given, and over {FAILED, PASS} that silently converts a
      blocked surface into a clean one. If an answer is wrong, delete
      `gate/answers/` in full and go back to step 2 — a re-run is a fresh
      fan-out, which mints a new identifier every answer must then name.

      An answer that is missing, unparseable, or attributed to a different run is
      a FAILED gate; it is never a gate over twelve surfaces.

      **4. Then loop, and expect to.** What makes the receipt FAILED is a
      BLOCKING finding, and blocking is decided by the finding's own recorded
      fields, not by a count and not by anyone's adjective: a finding whose
      `consequence` is `acts-wrongly` — a reader following the document does
      the wrong thing — blocks unconditionally, at every reach, with no
      override; any other finding blocks exactly when the agent that raised it
      judged it `blocks`. A finding judged `deferrable` does not stop the
      release and is not dropped for it: it stays on the receipt in full and
      reaches you with everything else. The gate surfaces and never fixes, so
      the fixes are yours, and a fix moves the tree. So produce this item again
      against the new `$TREE`, end to end: CI, `make ci-evidence` for the new
      merge commit, the captures, `delta`, `record`, a fresh fan-out. The
      tree-stamped captures leave no room to cut that short — `record` refuses
      their old stamps by name. The one artifact deliberately NOT keyed to a
      tree is `gate/delta.json`: its freshness is recomputation, so when the fix
      moved no inventory the delta on disk is byte-for-byte the one the new tree
      would produce and re-recording it is honest — that reuse is the cache
      working, not a step skipped — and when the fix moved `surface.json`,
      `record` refuses it until `delta` is re-run. Repeat until no surface
      reports a blocking finding.

      **5. Read the findings yourself before you authorize anything.** Nothing
      is filtered, deduplicated away or dropped on the way to you — the
      deferrable findings included. Each finding carries its agent's judgement
      (`consequence`, `failure_scenario`, `blocking`); the judgement is the
      agent's and the gate honours it, so what is left for you is to read the
      scenarios and disagree where you must. Disagreeing has exactly one shape
      in each direction. A deferrable finding you judge blocking is a fix you
      make before authorizing — the gate will not stop you from shipping over
      it, so this is the one place your reading is the check. A blocking
      finding you judge mistaken can only be cleared by disproving its own
      `failure_scenario` — the sentence exists so that it CAN be disproven —
      and then deleting the finding from `gate/answers/<surface>.json` by
      hand, since there is still no override field on the receipt. Know what
      that deletion costs before you reach for it: a deleted finding leaves no
      trace, so an adjudicated finding becomes indistinguishable from one
      nobody ever raised, and the next reader of that record cannot tell that
      you looked. And know what it cannot touch at all: an `acts-wrongly`
      finding is not deferrable and not signable-away by anyone — fix the
      software, or show with evidence that there was never a defect. Why the
      classifier that would derive a finding's weight from its evidence was
      not built, and why no override record was added in its place, is
      recorded at `cmd/dossierx/gate_stage3_test.go:42-57`.

      **This does not stand in for the driver's own check, and is not meant to.**
      D1 recomputes all of it inside its own process — it re-reads the fan-out
      record produced for the tree it is about to release, collects one answer per
      declared surface, recomputes the verdict, and requires the CI-run evidence —
      and it refuses when any of that is absent or is about a different tree. This
      item is what makes that recomputation possible.
- [ ] **CHANGELOG.md has an entry** for the new version, dated, following
      [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). GoReleaser's
      generated notes are commit subjects; they are not a substitute for this.
- [ ] **Breaking changes and silent-behaviour changes are called out first** in
      that entry. v0.3.1's renderer expansion changed what already-locked claim
      bodies render as, with no edit, no content-hash change and no ledger
      event — `dossierx check` reported exactly what it reported before. A
      change a consumer's gate cannot detect for them belongs at the top of the
      entry, not in a bullet halfway down.
- [ ] **The two contract snapshots are read, and the entry above is written
      from them.** These are the files that tell you a silent change happened:

          git diff vX.Y.Z-previous -- testdata/render-across-releases.golden.txt \
                                      testdata/envelope-contract.golden.txt

      `render-across-releases.golden.txt` diffs everything this tree renders
      against everything the previous release rendered; it is kept current on
      every push, so reading it is the step, not regenerating it. Every entry
      under **SILENT RENDER CHANGES** is a locked, byte-identical claim rendering
      differently and needs a CHANGELOG line. Read **EXPLAINED BY AN INPUT
      CHANGE** too — a hunk the named inputs do not account for is a silent
      change wearing an explanation.

      `envelope-contract.golden.txt` is the same for the JSON envelope: per
      pinned invocation, the keys of `data` with each one's JSON type, the error
      code, and the exit status. A diff there is a change to the machine contract
      `skills/dossierx/SKILL.md` documents to every client's agent.
- [ ] **The version pins are moved.** Sweep for them rather than recalling
      where they are:

      git grep -nE "dossierx(/cmd/dossierx)?@v|githubusercontent\.com/[^ ]*dossierx/v" \
        -- . ':!CHANGELOG.md' ':!docs/RELEASING.md' ':!surface.baseline.json'

      `surface.baseline.json` is excluded for CHANGELOG.md's reason: it is the
      frozen surface inventory of v0.5.0, so the four v0.5.0 pins inside it are
      historical facts that are correct precisely because they are old. It is
      also the ONLY record of what v0.5.0's surface was — that release shipped
      before the emitter existed and carries no `surface.json` of its own — so
      bumping a pin in it does not leave a stale file behind, it destroys the
      baseline the first gated release is diffed against, and every delta after
      that reports as unchanged the surfaces that changed.

      This used to be a `grep -rn --include="*.md" --include="*.yml"`, which does
      not search `*.yaml` — and when that sweep was retired this repo held 232
      of those against 10 `.yml`. It missed nothing, but a sweep with a blind
      spot degrades into memory, which is the exact thing this item exists to
      avoid. `git grep` needs no filter list to keep current.

      As of v0.5.1 that is FOUR pins across THREE files: `README.md` (the
      `go install` line and the `install-git-hook.sh` raw URL),
      `skills/dossierx/SKILL.md` (the same raw URL), and
      `scripts/ci/dossierx-check.yml` (the `go install` line — this one is a
      template users copy into their own repository, so a stale pin there
      ships a stale binary into someone else's merge gate).

      **Do not work from that list — work from the sweep, and treat the list
      as a cross-check.** The list is a cache of what the sweep found last
      time, and the hand-list form of it went stale through v0.3.0 and v0.3.1
      before a sweep, not memory, caught it. Both counts above are derived,
      not remembered: `surface.json`'s `version_pins` is the mechanical
      answer, regenerated from the tree on every push, and
      `TestTheReleasingPinParagraphMatchesTheMechanicalSweep` in
      `tests/derived_facts_test.go` fails the build when this sentence and
      that inventory disagree — when they do, this paragraph is the wrong
      one.
- [ ] **The embedded skills still describe this engine.** `skills/*/SKILL.md` is
      `go:embed`-ed into the binary and installed into *other people's*
      repositories by `dossierx skills export`, where it becomes the operating
      instruction an agent follows against a corpus you will never see. A stale
      rule here does not render a wrong page — it teaches an agent the wrong
      recovery on somebody else's locked claims, and it ships inside the binary,
      so a fix after the tag never reaches anyone who already installed.

      Ask the falsification question, not the mention question. Not "do the
      skills mention the new feature" but, for every assertion in them, "did
      this release make that FALSE?" — every command and flag against
      `dossierx <noun> --help`, every `error.code` and lint rule name against
      the code, every count, every "as of vX" claim.

      Then the case the skills are worst at: **a new refusal that can fire on a
      corpus the agent did not change.** An agent meeting one hunts for what it
      broke, finds nothing, and loops. If this release adds such a rule and no
      skill names it, that is blocking. v0.5.0's `mixed-cycle` is the worked
      example, and the router carries a section for it.
- [ ] **The site's release entry is appended.** In `site/src/content.ts` the
      `releases` array is **oldest-first**, and the last entry is the current
      release. Append; do not prepend. Move `tag: "Latest release"` off the
      previous entry.

      **Two expressions say "last", in two files, and they must agree.**
      `content.ts` selects `releases[releases.length - 1]` and
      `ReleaseTimeline.tsx` badges `releases.length - 1` "latest". Change one and
      every derived string names one release while the timeline badges another.
      Both are pinned by `TestSiteSelectsTheReleaseThisTreeModels`.

      **There is no `commit` field, and no step that stamps one.** It held the
      tagged release's short sha and was deleted outright, because it could not
      converge: writing the sha is itself a commit, so the value was stale the
      moment it landed — v0.4.1 shipped naming `5327923` while `refs/tags/v0.4.1`
      points at `206b4a4`. If you find an entry carrying one, delete it; do not
      fill it in.

      Every other version string on the site derives from that entry —
      the hero kicker, the hero badge, the release-history intro, and the
      `dossierx version` example all read `latestRelease` / `latestVersion`.
      Do not reintroduce a hand-typed copy; each of those four had one, and
      three of them went stale.

      **The `dossierx version` example reads `latestBinaryVersion`, not
      `latestVersion`, and the difference is a leading `v`.** GoReleaser's
      `{{.Version}}` strips it, so the archive published for `v0.5.0` prints
      `dossierx version 0.5.0`. `v0.5.0` is right everywhere the site names the
      RELEASE and wrong in a block depicting what a command prints.
- [ ] **The three committed sample viewers are regenerated.** This is the last
      item deliberately: regeneration has to reflect the branch's finished
      renderer, lint and CSS state, so it runs after everything above.

      `testdata/fixture-basic/viewer/index.html`,
      `testdata/fixture-portability/viewer/index.html` and
      `testdata/fixture-graph-demo/viewer/index.html` are tracked, generated
      artifacts (line 1: "generated by dossierx check … do not edit"). Run:

      go run ./cmd/dossierx check --config testdata/fixture-basic/project.config.yaml
      go run ./cmd/dossierx check --config testdata/fixture-portability/project.config.yaml
      go run ./cmd/dossierx check --config testdata/fixture-graph-demo/project.config.yaml

      and commit the diff. The only expected changes are the generation
      timestamp — which now appears in **three** places per document: line 1's
      `generated by dossierx check at …`, the sidebar-footer "Generated …"
      string, and the claims-graph payload's `generated_at` field — plus
      whatever markup this release's own change produces; anything else is a
      regression, not drift. That is still only **two timestamp formats**
      (RFC3339 and `2006-01-02 15:04 UTC`), which is what the staleness test
      below normalizes.

      **This item is now enforced, not remembered.**
      `TestCommittedFixtureViewersAreNotStale` in `tests/` regenerates every
      discovered fixture into a temp directory and diffs it against the
      committed one, so a commit that changes rendered output without
      regenerating is red. It discovers fixtures rather than hardcoding them,
      so a fourth fixture is covered the day it is added. Before that test
      existed, a rendering, CSS or viewer-chrome change shipped without these
      files going stale in both v0.3.1 and v0.4.1 — caught by review each time,
      never by CI. Keep this checklist item anyway: the test tells you the
      fixtures are stale, this tells you what to run.

## Tagging

- [ ] **`origin/main` is already an ancestor of the release branch.**

      git fetch origin && git merge-base --is-ancestor origin/main <branch>

      Exit 0 or the merge below is a real three-way merge, whose tree carries
      content from `main` that nothing verified. The recovery is
      `git merge origin/main` into the branch and re-run the checks above.

      The `git fetch` is not politeness: `origin/main` is a file in your clone,
      and asked without refreshing it the question answers "yes" exactly when the
      release is about to go wrong.

- [ ] **The merge belongs to the driver — never merge and then invoke it.** The
      merge is D2: `git merge --no-ff --no-edit` onto `main`, with the resulting
      merge commit captured by value and carried into the tag D4 creates. It is
      the step the whole ordering hangs off, because the release has to have a
      merge commit to name.

      **Merging first does not skip D2, it empties it.** `git merge --no-ff`
      finds `main` already contains the branch, prints `Already up to date.`,
      exits 0 and creates nothing. D2 then reports done having done nothing, and
      the driver tags whatever `main` already pointed at. Every guarantee D2
      exists for — that the merge was `--no-ff` rather than a squash or a rebase,
      that the commit the tag names is the merge the driver itself made — is
      gone, and nothing anywhere says so. That silent no-op is the state to
      avoid, and it is reached only by doing the merge and then running the
      driver.

      **So there is no hand-merge step in this procedure, and you are not
      skipping one.** The driver's evidence is wired to this repository's own
      gate run: D1 reads the fan-out record produced for the tree being
      released, collects one answer per declared surface, recomputes the verdict
      against that tree and requires the CI-run evidence, and D2 through D8 then
      run unattended. The merge is D2's and nobody else's. If D1 refuses, read
      what it names — most often that no fan-out was produced for this tree —
      and fix that; merging by hand does not unblock it, it empties the step.

      **Either way, start from a clean tree**: `git status --porcelain` empty and
      local `main` in sync with `origin/main`. Anything modified or untracked at
      this point is content no gate read and the merge carries it in. Both
      records this procedure writes — the ci-evidence record and the driver's own
      run record — default to paths outside the repository so that neither of
      them is what dirties it.

      **And `main` must not be checked out in another worktree.** D2's first act
      is `git checkout main` in the checkout you invoke from, and git allows a
      branch to be checked out in one worktree at a time — so in the layout a
      release is usually prepared in, with the branch in a linked worktree and
      `main` sitting in the primary checkout, that checkout is refused. D1 asks
      this before anything is read and refuses by name, printing the path of the
      worktree that holds the branch and the one command that clears it:

      git -C <that worktree> switch --detach

      Switch it back once the release is published. Nothing about the release
      itself changes — this is a fact about your desk, not about the tree — and
      the passage is here only so the refusal is not a surprise. Left to D2 the
      same layout stops the release anyway, after the whole gate run, with git's
      own `fatal:` and no mention of a release or a recovery.

- [ ] **Tag and push, in the driver's order.**

      make release-publish DOSSIERX_RELEASE_VERSION=vX.Y.Z \
                           DOSSIERX_RELEASE_AUTHORIZE=vX.Y.Z

      That target is the executor of every irreversible step below, and running
      it is the authorization: the driver publishes when a human types this and
      at no other time. `DOSSIERX_RELEASE_AUTHORIZE` is the version a second
      time on purpose — a boolean left in a profile or a secret would authorize
      every release forever, including the next one triggered by accident.

      The driver is `TestReleaseDriverPublishes` in
      `cmd/dossierx/gate_driver_test.go`. It records its own receipt in the same
      process, recomputes the verdict against the tree it is about to publish,
      and performs the steps in **this order**:

      git tag -a vX.Y.Z -m "vX.Y.Z — <title>" <merge-commit>
      git push origin vX.Y.Z
      # Verify the archives — see the section below — and only then:
      git push origin main

      **The tag goes first and `main` goes last, and the order is not
      interchangeable.** The release branch edits `site/src/content.ts`, so
      pushing `main` fires `.github/workflows/deploy-site.yml` and publishes a
      page announcing that vX.Y.Z is the current release — while `Release`,
      which fires only on a tag push, has not built a single archive. Pushing
      `main` first therefore announces a release nobody can download.

      **Name the merge commit explicitly.** `git tag -a vX.Y.Z` with no ref tags
      HEAD, which is only right when nothing has landed since the merge; the
      driver carries the merge commit by value from the merge to the tag and
      re-reads `<tag>^{tree}` immediately before pushing it.

      **Those commands are the driver's; you type none of them.** They are
      written out so the order is readable, not as a procedure to follow — D2
      through D8 are executed by the target above, including the archive
      verification between the two pushes. What this step asks of a person is
      the two things a program cannot do: confirm the gate's findings before
      authorizing, and type the version into the two variables.

      **A green run ends at D9, the handoff — and D9 is not a check.** The
      driver performs D0 through D8 and then hands the release over: its last
      act is to print, and to write into the run record, that vX.Y.Z is
      published and that the three checks in **Three checks that stay a
      person's** below have been made by nothing. After it pushes `main` it
      reads no workflow run, no release page and no byte of the live site —
      that is exactly why those three stay a person's — so read the handoff as
      the start of your work and never as a report that the release came back
      clean. It is the one ending that is not a failure; a run that stops
      anywhere earlier stops AS a failure, names the step, prints which steps
      are already public and which are not, and never resumes and never undoes.

      **No sha is stamped onto the site after this.** The step that wrote the
      release commit's short sha into `site/src/content.ts` is gone with the
      field it wrote to.

- [ ] **Regenerate the cross-release render report against the new baseline,**
      and push it to `main`:

      go test ./tests -run TestRenderedOutputAcrossReleases -regenerate-goldens

      The report is compared against the newest tag reachable from HEAD, so the
      tag you just pushed *is* the baseline from now on and the report empties
      out. It lands after the tag by necessity, so it is not inside the tagged
      tree; unlike the sha stamp it replaces in that position, it converges.

      Skipping it does not hide a change, it fabricates one: the next push reds
      `TestRenderedOutputAcrossReleases` with "written against a different
      release than the one it is now being compared with", and whoever meets
      that message goes looking for a rendering diff that was never there.

- [ ] Watch `Release`, `CI` and `CodeQL` while they run. `Release` is the one
      that must pass. Watching is not confirming — a run can fail after you stop
      looking — so its outcome is read again as a human check in **Three checks
      that stay a person's** below.

## Verifying — check the artifact, not the source

This is where the real failures have been. The rule:

> **Verify the thing the user sees, not the thing you edited.**

Confirming a string is present in a source file proves you made an edit. It
does not prove the edit reached the built output, that the built output
deployed, or that the deployed page renders it. Those are four different
claims and only the last one matters.

- [ ] **The release page** lists all six archives plus `checksums.txt`.
- [ ] **A clean install reports the new version:**

      go install github.com/BarterX-Tech/dossierx/cmd/dossierx@vX.Y.Z
      dossierx version --format text

      This proves the module proxy serves the tag and that the tagged source
      builds and runs. It proves **nothing about the ldflags**: `go install
      ...@vX.Y.Z` builds from source with none at all, and the binary then falls
      back to `debug.ReadBuildInfo`'s `info.Main.Version`, which the proxy sets to
      the tag — so it prints a version either way. (It cannot print `(devel)`
      either; that value is excluded and the last-resort fallback is `dev`.)

- [ ] **The ldflags reached the published binary.** This is the check the item
      above cannot make, and it is an artifact check: download the archive the
      release actually publishes and inspect *that* binary.

      gh release download vX.Y.Z --repo BarterX-Tech/dossierx --pattern 'dossierx_<os>_<arch>*'
      # unpack, then:
      go version -m ./dossierx
      ./dossierx version --format json

      **The `-ldflags` build setting is the signal, and it is the only one you
      should rest a verdict on.** `go version -m` prints the flags the binary
      was linked with, and the output must carry a `build -ldflags=` line
      naming `-X main.version=`. A build that got no ldflags carries no such
      line at all, and the historical failure — `-X` aimed at the full import
      path instead of `main.` — shows up here as an `-ldflags` line that never
      names `-X main.version=`. Neither `-s` nor `-w` hides it: those drop the
      symbol table and DWARF, not the build-info section `go version -m` reads.

      **Read the same line for `-X main.commit=` and `-X main.date=`.** The
      no-op is per symbol: the version can be stamped correctly while those two
      are aimed at the import path, and the binary then reports the sha and the
      *commit's* timestamp out of `debug.ReadBuildInfo` — both well-formed, both
      wrong about which build this is. Compare each of the three values the
      `-ldflags` line names against the matching field of `version --format
      json` below; a field the flags do not name at all is the tell.

      **Do not read the `version` output as proof of stamping.** Measured side by
      side on the v0.5.0 tree, an unstamped build reports the byte-identical
      commit and a plausible RFC 3339 date; only the version differs, and only by
      a leading `v`. Read the envelope to confirm the values the flags CARRIED are
      right, never to decide whether they applied.

      If the `-ldflags` line is absent or does not name `-X main.version=`, see
      the comment in `.goreleaser.yaml` about `-X main.version` needing the
      `main.` prefix rather than the full import path.
- [ ] **The rendered pages read correctly.** Load the live site and read the
      text, both `/` and `/releases.html` (a second Rollup entry point, not a
      route — `/releases/` is a 404).

      Grep is not sufficient here and gets steadily less sufficient: since the
      version strings became derived, they are minified into variables and a
      grep for the release tag in the bundle returns nothing whether the page is
      correct or broken. A zero from a 404 or a bad selector looks identical to
      a zero from a clean fix.

- [ ] **Any remaining mentions of the previous version are historical.** Read
      them in context before concluding they are stale. Most prose about a past
      release is correct and must not be bumped — "v0.3.0 made the machine
      contract the product's spine" describes history; rewriting it would make
      the page lie. Only the claims about what is *current* move.

### Three checks that stay a person's

These three ask whether a system outside this repository did what it was told,
and no file in this tree can answer that. A workflow that never fired, a deploy
that kept serving yesterday's bundle and a run that concluded without producing
an artifact all leave the repository byte-identical to the release that went
right, so there is nothing here for a check to read. They are not leftovers
waiting to be automated; they are read off the forge and off the live site, by a
person, every release. Skipping one is not "the machine has it" — it is nobody
having looked.

- [ ] **(human) The `Release` workflow itself passed.** Not "the tag is on the
      forge", not "the release page loads" — the run's own outcome. A run that
      failed or stopped halfway leaves a published tag with no archives behind
      it, and the tag is what every consumer resolves. Open the run for this
      tag and read its conclusion.
- [ ] **(human) `deploy-site` ran for this release.** `deploy-site.yml` triggers
      only on changes under `site/**`, so a release touching no site file
      publishes nothing, fails nowhere, and leaves the site serving the previous
      version — the quiet failure this item exists for. Check the workflow's
      runs for this release's commit; if it did not fire, the fix is a
      `workflow_dispatch`.
- [ ] **(human) The deployed bundle is the one that was built.** Vite
      content-hashes its assets, so fetch the live `index.html`, read the asset
      hashes out of it, and compare them against your local `dist/`. A match
      rules out a stale CDN copy and a deploy that succeeded while serving an
      older build. If you cannot build locally, at minimum confirm the live
      hashes CHANGED from what the previous deploy served — an unchanged hash
      after a site edit is the failure.

## After

- [ ] Close the issues the release resolves, naming the tag.
- [ ] If the release changes rendered output for existing consumers, say so
      where they will see it. Locked claims do not re-review themselves;
      `dossierx claim unlock` is the deliberate path back to them.
