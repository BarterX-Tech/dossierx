# DossierX site

Two static pages, published to GitHub Pages.

- `index.html` is a memo on why this project exists.
- `releases.html` is the release ledger.

`styles.css` is the whole stylesheet. `favicon.svg` is the whole asset set.

## There is no build

No bundler, no package manager, no generated output. Open `index.html` in a
browser, or serve the directory with anything at all.

```bash
python3 -m http.server --directory site 8000
```

Every link between the two pages is relative, so the pages work the same from a
file path, a local server, and the project-Pages subpath. Nothing here needs to
know it is served from `/dossierx/`.

**This was a Vite, React and TypeScript application, and the build was removed
on purpose.** The site had grown into twelve sections and fifteen components
describing a pre-1.0 tool whose direction is not settled, and describing it in
prose that had to be re-verified against the binary at every release: counted
claims, command tables, terminal transcripts, error codes. Four version strings
and one command count went stale in front of users anyway. The client-facing
account of what DossierX does now lives in the repository's `README.md`, next to
the binary that settles it. This site makes the case for the project and stops
there, which is why nothing on `index.html` states a count or names a version.

## The ledger is a release precondition

`releases.html` is not only a page. The release driver refuses to publish unless
the tree declares the release being tagged, and it establishes that from two
statements that have to agree. `CHANGELOG.md`'s newest version heading is one.
The entry in `releases.html` carrying `data-current="true"` is the other.
Exactly one entry may carry it. `tests/derived_facts_test.go` reads this file and
fails the build when the two disagree, and `docs/RELEASING.md` carries the step
that updates it.

Do not add a `commit` field, in any form. One existed and was deleted because it
could not converge. Writing the sha is itself a commit, so the value went stale
as it landed.

## Deployment

Deploys are automatic. On every push to `main` touching `site/**`, and on manual
`workflow_dispatch`,
[`.github/workflows/deploy-site.yml`](../.github/workflows/deploy-site.yml)
uploads this directory to GitHub Pages. No build step, and no `gh-pages` branch
push.

> **One-time admin setup.** The repository's Pages source must be set to
> "GitHub Actions" under **Settings, Pages, Build and deployment, Source**. Only
> a repository admin can make that change, and the workflow cannot set it. If
> deploys succeed in the Actions log but the live site does not update, check
> this first.
