# Releasing DossierX

The mechanical half of a release is one command: push a `v*` tag and
`.github/workflows/release.yml` runs GoReleaser, which builds the six
platform archives, stamps `main.version` / `main.commit` / `main.date` via
ldflags, generates the GitHub release notes from Conventional Commit subjects,
and publishes.

Everything that has ever gone wrong with a DossierX release has been in the
other half: the copies of the version number that live in prose, and the
verification step that checked the wrong artifact. This document is the
checklist for that half.

## Before tagging

- [ ] **`go test ./...` passes**, including the two suites it does not reach on
      its own — see [CONTRIBUTING.md](../CONTRIBUTING.md#the-two-suites-go-test--does-not-reach).

      It needs nothing but Go. The two checks with an external prerequisite — a
      browser and a `goreleaser` binary — live in `viewer-tests/`, which
      `go test ./...` does not descend into; the next item runs them.
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
- [ ] **CI is green on `main`** — not on the branch, on the merge commit.

      **Open the run; a green badge is not the check.** Nothing in this
      repository can establish that CI executed anything — `tests/ci_workflow_test.go`
      reads what the workflow *declares* and says so in its header — so this item
      is where a person answers it, and there are three things to look at:

      - the **viewer** job appears on the merge commit at all. A workflow whose
        triggers or `paths:` filter stopped matching produces no job, and a
        commit with nothing to report reads as a commit with nothing wrong.
      - its conclusion is **not "skipped"**. A skipped job is not a pass; it is a
        pass over zero assertions.
      - its **Viewer browser suite** step shows Go tests that actually ran —
        named tests and timings, not `[no tests to run]` beside every `ok`. A
        step allowed to continue on error reports the job green over a red
        suite; a suite narrowed by a `-run`/`-tags` selector reaching it at run
        time reports `ok` over zero assertions. Neither is distinguishable from
        a real pass without opening the step.
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
        -- . ':!CHANGELOG.md' ':!docs/RELEASING.md'

      This used to be a `grep -rn --include="*.md" --include="*.yml"`, which does
      not search `*.yaml` — and this repo has 232 of those against 10 `.yml`. It
      missed nothing, but a sweep with a blind spot degrades into memory, which
      is the exact thing this item exists to avoid. `git grep` needs no filter
      list to keep current.

      As of v0.5.0 that is `README.md` (the `go install` line and the
      `install-git-hook.sh` raw URL), `skills/dossierx/SKILL.md` (the same raw
      URL), and `scripts/ci/dossierx-check.yml` (the `go install` line — this
      one is a template users copy into their own repository, so a stale pin
      there ships a stale binary into someone else's merge gate). It went stale
      through v0.3.0 and v0.3.1 and was found by a sweep, not by memory.
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

- [ ] Merge to `main` with `--no-ff` so the release has a merge commit to name.
- [ ] Tag and push:

      git tag -a vX.Y.Z -m "vX.Y.Z — <title>"
      git push origin main
      git push origin vX.Y.Z

      `git tag -a vX.Y.Z` with no ref tags HEAD, which is only right when
      nothing has landed since the merge. Name the commit explicitly if
      anything has.

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

- [ ] Watch `Release`, `CI` and `CodeQL`. `Release` is the one that must pass;
      a failure there leaves a tag with no artifacts behind it.

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
- [ ] **The site redeployed.** `deploy-site.yml` triggers only on changes under
      `site/**`, so a release that touches no site file publishes nothing and
      the site keeps serving the previous version. Use `workflow_dispatch` if
      that happens.
- [ ] **The deployed bundle is the one you built.** Compare the asset hashes in
      the live `index.html` against your local `dist/` — Vite content-hashes
      them, so a match rules out a stale cache or a failed deploy having served
      an older build.
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

## After

- [ ] Close the issues the release resolves, naming the tag.
- [ ] If the release changes rendered output for existing consumers, say so
      where they will see it. Locked claims do not re-review themselves;
      `dossierx claim unlock` is the deliberate path back to them.
