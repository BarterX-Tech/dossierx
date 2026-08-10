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

ONE LIMIT YOU MUST RESPECT. The rendered text you were handed carries no link
targets: the extractor captures text, labels and states, and no href at all. So
you cannot confirm from it that a link goes where its text says. If a claim you
are judging depends on where a link points, report that you could not check it
and name the file from the not-handed-over list that would answer it.
