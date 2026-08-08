## The surface: claim-format-spec

FORMAT.md is the claim format: the schema, the id grammar, the layouts, and what
the claim-body renderer accepts. Anyone authoring claims reads it, and the
inventory's `markdown_constructs` is the corpus this document is a claim about.

Check, specifically:

- Every markdown construct this document says the renderer accepts appears in
  `markdown_constructs`, and every construct in that list that a reader would
  need to know about is described here.
- The id grammar stated matches the grammar the engine enforces, including the
  error code raised on a violation.
- Every schema field named here exists, and every required field is described as
  required.
- Examples in this document are valid under the format this document describes.
