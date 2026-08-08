## The surface: changelog

CHANGELOG.md is the release's own account of itself, and the only place a SILENT
BEHAVIOUR CHANGE can be announced to somebody deciding whether to upgrade.

This surface is judged the other way round from the rest. The question is not
only "is anything written here false" but "is anything the delta reports MISSING
from here". Work from the delta and the cross-release render diff inward:

- For every entry in the release delta — a command, flag, error code, lint rule
  or behaviour fingerprint that moved — find the line in this release's section
  that announces it. A moved `behaviour_fingerprint` with no line is the
  canonical failure: a corpus that passed `dossierx check` before the release
  fails after it, with nothing in the notes to warn anyone.
- For every artifact in the cross-release render diff, find the line describing
  what changed in rendered output. Already-locked, byte-identical claims
  rendering differently has shipped here twice.
- An empty delta is a legitimate answer, and it means this release changed no
  shipped behaviour. It does not license an entry claiming one.
- Check that entries describe what actually changed rather than what the commit
  subject said.
