# Releasing DossierX

DossierX is alpha. A release is a maintainer reading the change, running the
suites, and tagging. There is no gate pipeline in front of it any more, and this
file is the whole procedure.

`.github/workflows/ci.yml` runs the suites on every push and pull request, and
that is the only thing that runs without being asked.

**Nothing stands between a `v*` tag and a published release.** Pushing the tag
runs GoReleaser, which builds the six platform archives, stamps `main.version`,
`main.commit` and `main.date` through ldflags, generates the release notes from
the Conventional Commit subjects, and publishes. Any commit, on any branch, tagged
anything beginning with `v` publishes a release of this project.

So these three are yours, every time, and there is no second reader:

- the tag names the **merge commit**, not the release branch head. The branch head
  carries this release's stamp too, so it looks right from every angle except the
  one that matters.
- `site/releases.html`'s `data-current="true"` entry names the version being
  tagged. A tag that disagrees ships a binary the project's own page calls another
  release.
- the tag names the commit you meant.

The checklist below asks for each one.

## Before tagging

- [ ] **The suites are green, all four of them.** `go test ./...` does not
      descend into `viewer-tests/`, which is a separate module, so a green root
      run says nothing about the browser suite.

      make test
      make hook-test
      make viewer-test   # needs DOSSIERX_TEST_BROWSER and DOSSIERX_TEST_GORELEASER
      make viewer-lint

      `make viewer-test` skips its chromedp cases when `DOSSIERX_TEST_BROWSER` is
      unset, and a skip proves nothing. Point it at a real Chrome or Chromium
      before you read its result as coverage.

- [ ] **`CHANGELOG.md` has a heading for this version** and its entries describe
      what a consumer sees, not what the diff touched. The newest heading is the
      release being tagged; if it still says `[Unreleased]`, the tag is ahead of
      the file.

- [ ] **`site/releases.html` carries this version** as the entry marked
      `data-current="true"`, and the previous release's entry no longer is. That
      entry is what a visitor is shown. Nothing checks it against the tag, so a
      release tagged over a stale stamp publishes archives beside a page calling
      another version current.

- [ ] **`surface.json` is current.** It is the mechanically derived inventory of
      everything a client can observe, and a stale copy is a red build.

      go test ./cmd/dossierx -run TestGenerateSurfaceJSON -regenerate-goldens

      Read the diff. A command, flag or exit code that moved and is not in
      `CHANGELOG.md` is the thing this step exists to catch.

- [ ] **Every rendered fixture is regenerated and re-committed.** `ls -d
      testdata/fixture-*/` minus `fixture-coverage` — every one of them, not a
      sample — with:

      go run ./cmd/dossierx check --config testdata/<fixture>/project.config.yaml

      run from the repository root, exit 0 required for each. This rewrites
      `build/viewer/index.html` and `build/catalog/catalog.json` for every
      fixture, and `build/ledger/comment-digest.json` for a fixture that has
      no ledger yet. Afterwards `git status --porcelain testdata` may show
      only the viewers — each differing in its generation stamp alone, which
      `go test ./tests -run TestCommittedFixtureViewersAreNotStale -count=1 -v`
      passing on the uncommitted tree is the check for — and nothing
      untracked. Because the vendored mermaid renderer changes every rendered
      viewer this release, a fixture with a locked build order commits a
      viewer noticeably larger than before: record each fixture viewer's
      before/after `wc -c` in the CHANGELOG entry so the repository's growth
      is visible rather than discovered later from
      `TestCommittedFixtureViewersAreNotStale` failing — the way the fixture
      viewers went stale through v0.3.0 and v0.3.1.

- [ ] **Every release-version pin points at the version being released.** Sweep
      with `git grep`, never a plain `grep -r`: on some machines `grep` resolves
      to ugrep, whose `-r` skips dot-directories, so `.github/` goes unsearched
      and a pin in a workflow file is invisible.

      git grep -nE 'dossierx(/cmd/dossierx)?@v|githubusercontent\.com/[^ ]*dossierx/v' -- \
        . ':!surface.json' ':!CHANGELOG.md' ':!docs/RELEASING.md'

      The three exclusions are not tidiness. `CHANGELOG.md` and this file are
      full of old version strings that are correct because they are old, and
      `surface.json` records the pins the sweep finds, so an unexcluded sweep
      finds its own output.

      As of v0.7.0 that is FOUR pins across THREE files: `README.md` (the `go
      install` line and the `install-git-hook.sh` raw URL),
      `skills/dossierx/SKILL.md` (the same raw URL), and
      `scripts/ci/dossierx-check.yml` (the `go install` line, which is a template
      users copy into their own repository, so a stale pin there ships a stale
      binary into somebody else's merge gate).

      **Do not work from that list. Work from the sweep and treat the list as a
      cross-check.** It is a cache of what the sweep found last time, and the
      hand-list form of it went stale through v0.3.0 and v0.3.1 before a sweep,
      not memory, caught it. Both counts are derived rather than remembered:
      `surface.json`'s `version_pins` is the mechanical answer, and
      `TestTheReleasingPinParagraphMatchesTheMechanicalSweep` in
      `tests/derived_facts_test.go` fails the build when this sentence and that
      inventory disagree. When they do, this paragraph is the wrong one.

- [ ] **The embedded skills still describe this engine.** `skills/*/SKILL.md` is
      `go:embed`-ed into the binary and installed into *other people's*
      repositories by `dossierx skills export`, where it becomes the operating
      instruction an agent follows against a corpus you will never see. A stale
      rule here does not render a wrong page; it teaches an agent the wrong
      recovery on somebody else's locked claims, and it ships inside the binary.

- [ ] **Mentions of the previous version that remain are historical.** Read them
      in context. Most prose about a past release is correct and must not be
      bumped: "v0.3.0 made the machine contract the product's spine" describes
      history, and rewriting it makes the page lie. Only claims about what is
      *current* move.

## Tagging

Start from a clean tree: `git status --porcelain` empty, and local `main` in sync
with `origin/main`. Anything modified or untracked here is content nobody
reviewed, and the merge carries it in.

- [ ] **Merge the release branch into `main` with `--no-ff`.**

      git fetch origin
      git checkout main && git merge --ff-only origin/main
      git merge --no-ff --no-edit <branch>

      `--no-ff` is what produces the merge commit the tag needs. A squash or a
      rebase leaves `main` with no merge commit at the tip, and the tag then names
      a branch head that was never merged. Nothing refuses that; check it.

- [ ] **Tag the merge commit and push the tag, in this order.**

      MERGE=$(git rev-parse HEAD)
      git tag -a vX.Y.Z -m "vX.Y.Z — <title>" "$MERGE"
      git push origin refs/tags/vX.Y.Z:refs/tags/vX.Y.Z
      # Verify the archives — see the next section — and only then:
      git push origin "$MERGE":refs/heads/main

      **The tag goes first and `main` goes last, and the order is not
      interchangeable.** A release branch edits `site/releases.html`, so pushing
      `main` fires `.github/workflows/deploy-site.yml` and publishes a page saying
      vX.Y.Z is the current release, while `Release`, which fires only on a tag
      push, has not built a single archive. Pushing `main` first announces a
      release nobody can download.

      **Both refspecs are fully qualified, and that is not tidiness either.** A
      release developed on a branch named `vX.Y.Z` gives that name two referents,
      and git resolves the ambiguity by search order rather than by intent. The
      short forms fail in two different ways over the same tree. `git push origin
      vX.Y.Z` refuses outright, *src refspec vX.Y.Z matches more than one*, so it
      is at least loud. `git rev-parse --short vX.Y.Z` does not: it warns, then
      answers, and it answers from `refs/tags` by a convention nothing enforces,
      while the branch of the same name points somewhere else. A release stamped
      from the wrong one of those two is wrong about which commit it shipped, and
      nothing downstream can tell. `refs/tags/…` and `refs/heads/…` have exactly
      one referent each.

      Naming release branches so they cannot collide, `release/vX.Y.Z`, is worth
      doing and is **not** a substitute. It relies on everyone remembering; a
      qualified refspec relies on nothing.

## Verifying — check the artifact, not the source

This is where the real failures have been. The rule:

> **Verify the thing the user sees, not the thing you edited.**

Confirming a string is present in a source file proves you made an edit. It does
not prove the edit reached the built output, that the output deployed, or that
the deployed page renders it. Those are four claims and only the last one matters.

- [ ] **The release page** lists all six archives plus `checksums.txt`.

- [ ] **A clean install reports the new version:**

      go install github.com/BarterX-Tech/dossierx/cmd/dossierx@vX.Y.Z
      dossierx version --format text

      This proves the module proxy serves the tag and that the tagged source
      builds and runs. It proves **nothing about the ldflags**: `go install
      ...@vX.Y.Z` builds from source with none at all, and the binary falls back
      to `debug.ReadBuildInfo`'s `info.Main.Version`, which the proxy sets to the
      tag, so it prints a version either way.

- [ ] **The ldflags reached the published binary.** This is the check the item
      above cannot make, and it is an artifact check: download the archive the
      release actually publishes and inspect *that* binary.

      gh release download vX.Y.Z --repo BarterX-Tech/dossierx --pattern 'dossierx_<os>_<arch>*'
      # unpack, then:
      go version -m ./dossierx
      ./dossierx version --format json

      **The `-ldflags` build setting is the signal, and it is the only one you
      should rest a verdict on.** `go version -m` prints the flags the binary was
      linked with, and the output must carry a `build -ldflags=` line
      naming `-X main.version=`. A build that got no ldflags carries no such line at all, and
      the historical failure, an `-X` aimed at the full import path instead of
      `main.`, shows up here as an `-ldflags` line that never names that symbol
      at all. Neither `-s` nor `-w` hides it: those drop the symbol table
      and DWARF, not the build-info section `go version -m` reads.

      **Read the same line for `-X main.commit=` and `-X main.date=`.** The no-op
      is per symbol. The version can be stamped correctly while those two are
      aimed at the import path, and the binary then reports the sha and the
      *commit's* timestamp out of `debug.ReadBuildInfo`, both well-formed and both
      wrong about which build this is. Compare each of the three values the
      `-ldflags` line names against the matching field of `version --format json`;
      a field the flags do not name at all is the tell.

      **Do not read the `version` output as proof of stamping.** Measured side by
      side on the v0.5.0 tree, an unstamped build reports the byte-identical
      commit and a plausible RFC 3339 date; only the version differs, and only by
      a leading `v`. Read the envelope to confirm the values the flags CARRIED are
      right, never to decide whether they applied.

      `TestLdflagsShowUpOnlyInTheBuildSettings` is the code half of this item, and
      `TestGoreleaserConfigProducesTheArtifactsTheProcedureExpects` holds
      `.goreleaser.yaml` to the six archives and the checksum file named here.

- [ ] **The rendered pages read correctly.** Load the live site and read the
      text, both `/` and `/releases.html`. The site is static HTML, so what is in
      `site/` is what is served, but a deploy that did not fire serves the
      previous copy and looks identical to a clean one from inside the repository.

### Three checks that stay a person's

These ask whether a system outside this repository did what it was told, and no
file here can answer that. A workflow that never fired, a deploy still serving
yesterday's page, and a run that ended without producing an artifact all leave
the repository byte-identical to the release that went right, so there is nothing
for a check to read.

- [ ] **(human) The `Release` workflow itself passed.** Not "the tag is on the
      forge", not "the release page loads": the run's own outcome. A run that
      failed halfway leaves a published tag with no archives behind it, and the
      tag is what every consumer resolves.
- [ ] **(human) `deploy-site` ran for this release.** It triggers only on changes
      under `site/**`, so a release touching no site file publishes nothing, fails
      nowhere, and leaves the site on the previous version. If it did not fire,
      the fix is a `workflow_dispatch`.
- [ ] **(human) The live page is the one you pushed.** Fetch it and read the
      version out of it. An unchanged page after a site edit is the failure.

## After

- [ ] Close the issues the release resolves, naming the tag.
- [ ] If the release changes rendered output for existing consumers, say so where
      they will see it. Locked claims do not re-review themselves; `dossierx claim
      unlock` is the deliberate path back to them.
