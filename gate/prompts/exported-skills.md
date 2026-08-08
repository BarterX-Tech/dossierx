## The surface: exported-skills

.claude/skills/ is the skills export output, checked into this repository as
symlinks so this repository's own agents read exactly the bytes a client's agents
would. A divergence here means the export is producing something other than the
source.

You have been handed the export capture — what `dossierx skills export` actually
writes — alongside the linked bundles themselves.

Check, specifically:

- Every bundle the capture writes is one the inventory's `skills` list declares,
  and none is missing.
- The exported text matches the source bundle after the export's own transforms.
  A wikilink left as literal `[[name]]` in the exported form is a link that
  resolves for nobody.
- The three forms the export produces are all present in the capture.
- Anything the capture shows being written into a client's repository that a
  client would not expect is a finding.
