## The surface: binary-and-viewer

The engine itself, and every string compiled into it: cobra Short and Long text
and flag usage (`dossierx <noun> --help`), the error messages and hints, and the
viewer templates, CSS and graph client rendered into every client's index.html.

THIS SURFACE IS UNFIXABLE AFTER THE TAG.

What you have been handed, and what you have not:

- The mechanical inventory, surface.json. Its command, flag, error-code,
  lint-rule, route, render and behaviour fields are extracted FROM this code. It
  is a projection of BEHAVIOUR and carries no prose at all — no Long text, no
  flag usage, no error message, no hint, no template.
- The prose itself, in bytes, for the four classes the checks below are about:
  every non-test file under `cmd/dossierx/` (the cobra text, and the error
  messages and hints the CLI prints), `internal/cliout/` (the error-code
  registry and the envelope every message is built from),
  `internal/render/viewer/template/` (the viewer shell, its CSS and the graph
  client) and the `.html` component templates under
  `internal/render/components/`.
- The cross-release render diff, gate/render-diff.json.
- The list of the remaining source files, which you were NOT handed. Those are
  the engine's logic, and surface.json is the projection of them. If a reading
  of yours depends on one of them, say so and name it — do not assume it agrees
  with what you were given.

Check, specifically:

- Every command's Short and Long text describes what the command now does, given
  the delta. Read the text itself, not the inventory's summary of it.
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
