## The surface: agent-skills

skills/ holds the SKILL.md bundles, plus the embed directive and reading order
that decide which of them ship and in what sequence a client's agent meets them.

THIS SURFACE IS UNFIXABLE AFTER THE TAG. These bundles are go:embed-ed into the
binary and installed into OTHER people's repositories by `dossierx skills
export`, where a stale rule teaches an agent the wrong recovery against a corpus
nobody here will ever see.

Check, specifically:

- Every command, flag, error code and exit code a bundle names exists in the
  inventory with the meaning the bundle gives it.
- Every error-code-to-recovery mapping a bundle teaches is still the right
  recovery after this release's delta.
- The reading order and the set of bundles that ship match `skills` in the
  inventory.
- Any release-version pin in a bundle names this release.
- Cross-references between bundles resolve to bundles that ship.
