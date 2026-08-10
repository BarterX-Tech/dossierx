## The surface: readme

README.md is this project's front door on GitHub: the install instructions, the
two-roles framing, the noun table, the exit-code table and the lock lifecycle.
Every visitor reads it, and it carries two of the four release-version pins.

Check, specifically:

- Every noun and subcommand named here exists in the inventory's `commands`, and
  every flag shown on a command line is in that command's `flags`.
- Every exit code and error code quoted here appears in `error_codes` with the
  same meaning.
- Every counted claim ("seven nouns", "28 lint rules") matches `counts`.
- Every version pin in an install line names the release being published, not the
  previous one. A pin left behind installs the wrong binary for everyone who
  copies the line.
- The lock lifecycle described matches the behaviour the delta reports; a
  lifecycle rule that changed silently is the worst case here.
