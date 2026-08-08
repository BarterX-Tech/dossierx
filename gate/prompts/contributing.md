## The surface: contributing

CONTRIBUTING.md tells a contributor how to build and test this project,
including the two suites `go test ./...` does not reach.

Check, specifically:

- Every command line quoted here would work: the packages exist, the flags exist,
  the environment variables named are the ones the suites actually read.
- The suites described as separate are still separate, and any suite added or
  removed by this release is reflected.
- Any tool version or prerequisite stated matches what the repository now
  requires.
- A contributor following this document end to end reaches a green tree. If a
  step is missing, that is a finding.
