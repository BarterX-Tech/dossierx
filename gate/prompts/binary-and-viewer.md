## The surface: binary-and-viewer

The engine itself, and every string compiled into it: cobra Short and Long text
and flag usage (`dossierx <noun> --help`), the error messages and hints, and the
viewer templates, CSS and graph client rendered into every client's index.html.

THIS SURFACE IS UNFIXABLE AFTER THE TAG. You have been handed the mechanical
inventory as the extract of it — the inventory's command, flag, error-code,
lint-rule, route, render and behaviour fields are extracted FROM this code — plus
the list of source files you were not handed.

Check, specifically:

- Every command's Short and Long text describes what the command now does, given
  the delta.
- Every error message and hint names a recovery that still exists: a hint
  pointing at a removed flag, or at a command that was renamed, sends a user
  nowhere.
- Every error code the prose references appears in `error_codes`.
- The viewer's own visible strings — labels, legends, empty states — describe
  what the viewer now shows, and the cross-release render diff says whether that
  changed.
- A `behaviour_fingerprint` entry that moved with no user-visible statement
  changed is not automatically a finding here, but it IS the thing the changelog
  surface must announce; say what moved so that reading has something to work
  from.
