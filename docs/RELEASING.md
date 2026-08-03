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
- [ ] **CI is green on `main`** — not on the branch, on the merge commit.
- [ ] **CHANGELOG.md has an entry** for the new version, dated, following
      [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). GoReleaser's
      generated notes are commit subjects; they are not a substitute for this.
- [ ] **Breaking changes and silent-behaviour changes are called out first** in
      that entry. v0.3.1's renderer expansion changed what already-locked claim
      bodies render as, with no edit, no content-hash change and no ledger
      event — `dossierx check` reported exactly what it reported before. A
      change a consumer's gate cannot detect for them belongs at the top of the
      entry, not in a bullet halfway down.
- [ ] **The version pins are moved.** Sweep for them rather than recalling
      where they are:

      grep -rn "dossierx@v\|/v0\.[0-9]" --include="*.md" --include="*.yml" . \
        | grep -v "CHANGELOG\|docs/RELEASING.md"

      As of v0.4.0 that is `README.md` (the `go install` line and the
      `install-git-hook.sh` raw URL), `skills/dossierx/SKILL.md` (the same raw
      URL), and `scripts/ci/dossierx-check.yml` (the `go install` line — this
      one is a template users copy into their own repository, so a stale pin
      there ships a stale binary into someone else's merge gate). It went stale
      through v0.3.0 and v0.3.1 and was found by a sweep, not by memory.
- [ ] **The site's release entry is appended.** In `site/src/content.ts` the
      `releases` array is **oldest-first**, and `ReleaseTimeline` treats
      `releases[releases.length - 1]` as current. Append; do not prepend. Set
      `commit` on the new entry, and move `tag: "Latest release"` off the
      previous one.

      Every other version string on the site derives from that entry —
      the hero kicker, the hero badge, the release-history intro, and the
      `dossierx version` example all read `latestRelease` / `latestVersion`.
      Do not reintroduce a hand-typed copy; each of those four had one, and
      three of them went stale.

## Tagging

- [ ] Merge to `main` with `--no-ff` so the release has a merge commit to name.
- [ ] Tag and push:

      git tag -a vX.Y.Z -m "vX.Y.Z — <title>"
      git push origin main
      git push origin vX.Y.Z

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

      If it prints a `(devel)` fallback instead of the tag, the ldflags did not
      apply — see the comment in `.goreleaser.yaml` about `-X main.version`
      needing the `main.` prefix rather than the full import path.
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
