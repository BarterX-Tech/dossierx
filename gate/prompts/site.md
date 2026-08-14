## The surface: site

The marketing and documentation site. THE SURFACE IS THE RENDERED DOM of a real
build plus its head metadata, and that is what you have been handed as the
rendered site text — not the component source. But every file listed as not
handed over decides what that DOM says.

Check, specifically:

- Every counted claim in the rendered text matches `counts` in the inventory. The
  site's counts are the ones that have gone stale live.
- Every command, flag and error code shown on the site exists in the inventory.
- Every version string shown names this release.
- Every terminal transcript on the site shows output the engine would actually
  produce.
- The head metadata — title and meta description — describes this project as it
  now is.

LINK DESTINATIONS ARE IN YOUR MATERIAL, as of v0.5.2. The rendered dump carries a
`links` list per page: every anchor the page rendered at any point, as the text a
reader sees and the destination it resolves to. So a sentence that is a claim about
where a link goes — "See the full release history", "View on GitHub", a section
nav promising an in-page anchor — is CHECKABLE, and reporting it as uncheckable is
now itself wrong. This paragraph told you the opposite until v0.5.2 and the round
that read it filed three such findings; the capture was widened rather than the
findings waved away.

TWO THINGS THE LIST STILL CANNOT TELL YOU, and both are honest limits rather than
gaps to report as findings. A same-origin destination resolves against the
ephemeral local server the capture ran under, so its host and port are an artefact
of the run and only the PATH is meaningful. And a link the traversal never revealed
is not in the list at all — absence is not evidence that a link is missing.
