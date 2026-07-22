# DossierX site

The public marketing / documentation site for [DossierX](https://github.com/BarterX-Tech/dossierx),
built with [Vite](https://vitejs.dev/) + React + TypeScript and published to GitHub Pages.

## Run it locally

```bash
npm install
npm run dev
```

This starts the Vite dev server (with hot-module reloading) and prints a local
URL to open in your browser.

## Build

```bash
npm run build
```

This type-checks (`tsc -b`) and produces a static, deployable bundle in `dist/`.

The site is served from a project-Pages subpath, so the build reads its base
path from the `VITE_BASE` environment variable (defaulting to `/dossierx/` — see
`vite.config.ts`). The deploy workflow sets `VITE_BASE=/dossierx/` explicitly.
To preview a production build locally:

```bash
npm run preview
```

## Deployment

Deploys are **automatic**. On every push to `main` that touches `site/**` (and
via manual `workflow_dispatch`), the
[`.github/workflows/deploy-site.yml`](../.github/workflows/deploy-site.yml)
GitHub Actions workflow builds this directory and publishes `dist/` to GitHub
Pages using the modern Pages artifact deployment
(`actions/upload-pages-artifact` + `actions/deploy-pages`) — no `gh-pages`
branch pushing involved. Merge to `main` and the site updates on its own.

> **One-time admin setup required.** For the workflow to publish, the
> repository's GitHub Pages **source must be set to "GitHub Actions"** under
> **Settings → Pages → Build and deployment → Source**. This is a manual
> setting change that only a repository admin can make; it cannot be done from
> code or from the workflow itself. If deploys succeed in the Actions log but
> the live site doesn't update, check this setting first.
