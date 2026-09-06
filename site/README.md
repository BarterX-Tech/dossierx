# DossierX site

Static HTML, published to GitHub Pages. No bundler, package manager, or build step.

- `index.html` is the landing-page memo, including the DossierX vision.
- `memo.css` and `memo.js` provide its layout and light/dark theme switch.
- `favicon.svg` is the shared site icon.
- `releases.html` is the release ledger and uses `styles.css`.
- `system-record.html`, `where-to-start.html`, and `real-example.html` remain
  available at their existing URLs, with `proposal.css` and the example assets.

The homepage follows the typography, narrow column, and cream/charcoal palette
of nitinkhanna.io. It contains no blog posts or placeholder writing links.
Product details and installation instructions live in the repository README.

## Preview

```bash
python3 -m http.server --directory site 8000
```

Open `http://localhost:8000/`. The stylesheet, script, and icon use relative
URLs, so the homepage also works under the GitHub Pages project subpath.
The theme follows the browser preference until a visitor chooses a theme;
the switch saves that choice locally when browser storage is available.

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
