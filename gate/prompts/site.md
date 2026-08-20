## The surface: site

The published site: two static HTML pages and their stylesheet, under `site/`.
`index.html` is a memo on why this project exists; `releases.html` is the
release ledger.

THE FILES YOU HAVE BEEN HANDED ARE THE SURFACE, and that is a change from
previous releases worth stating plainly. This used to be a Vite + React
application, so the surface was defined as the RENDERED DOM of a real build and
you were handed an extraction of it rather than the source. There is no build
now — `.github/workflows/deploy-site.yml` uploads the directory as it stands —
so the bytes in front of you are the bytes GitHub Pages serves. Nothing stands
between what you read and what a visitor gets, and no capture is staged.

Check, specifically:

- Every counted claim matches `counts` in the inventory. **The memo is intended
  to carry no count at all**, for the reason its own comment gives: the meta
  description advertised a "19-command JSON CLI" through two minor releases
  after the surface changed. A number that has appeared on `index.html` is
  therefore worth reporting even when it is currently correct — it is a claim
  that will go stale with nothing deriving it.
- Every command, flag and error code shown exists in the inventory. The memo
  names very few by design; a new one is worth a second look.
- Every version string names this release. Exactly one entry in `releases.html`
  carries `data-current="true"`, and it must be the release being cut. A second
  marked entry, or none, is a defect regardless of what the rest of the page
  says.
- The head metadata — title and meta description on both pages — describes this
  project as it now is.
- The memo's argument is still true of the tool. It claims the project is
  pre-1.0, that the command surface has moved and may move again, and that the
  right unit of review is unsettled. If a release has made any of that false —
  a 1.0, a settled surface — the page is wrong in the way that matters most,
  because it is the page's whole subject.

ONE LIMIT YOU MUST RESPECT. You are reading HTML, so you can see every `href`
— but you cannot resolve one. If a claim you are judging depends on where a
link actually goes, or on whether a fragment exists at the other end, report
that you could not check it and name what would answer it.
