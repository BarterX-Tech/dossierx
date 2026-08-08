## The surface: release-procedure

docs/RELEASING.md is the ONE description of how this project releases. There is
exactly one of them by rule, and this prose has gone stale twice.

Check, specifically:

- Its own pin-site list. The document tells the maintainer which files carry a
  release-version pin; compare that list against the inventory's `version_pins`.
  A pin site the inventory reports and this list omits is the failure that ships
  a stale version into somebody else's CI, and the sweep is the only thing that
  would have caught it.
- Every command line in the procedure exists and takes the flags shown.
- Every step's stated precondition and effect matches the delta: a step that
  describes behaviour this release changed is false.
- Any file this procedure says must never be edited, or must always be
  regenerated, still has that status.
