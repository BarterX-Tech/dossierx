## The surface: ci-merge-gate-template

scripts/ci/dossierx-check.yml is the merge-gate workflow clients COPY INTO THEIR
OWN REPOSITORY. It ships a binary version into somebody else's CI, and it carries
the release-version pin that matters most.

Check, specifically:

- The version pin names this release. A stale pin here installs an old binary
  into every client that copies this file, and they will not notice.
- Every `dossierx` invocation in the workflow exists as a command with the flags
  shown, and the exit codes the workflow branches on are the codes the inventory
  declares.
- Any behaviour this release changed that would alter what this workflow reports
  in a client's repository — a rule broadened so a previously-passing corpus now
  fails — is a finding even when the file itself is unchanged.
- Comments in the file that describe what a step does are still true.
