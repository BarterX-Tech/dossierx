## The surface: release-notes

The GitHub release body is generated at tag time by GoReleaser from Conventional
Commit SUBJECTS, so it does not exist until the tag. .goreleaser.yaml's
`changelog.groups` and `changelog.filters.exclude` are the rules that decide what
it will say, and those rules ARE the reviewable surface.

You have been handed the prediction of what the release body will contain for
this release's commit range, alongside the config.

Check, specifically:

- Every user-visible change in the release delta appears somewhere in the
  predicted body. A change landing under a `docs:` or `chore:` subject is dropped
  by the exclude filters and is invisible on the release page while being fully
  described in CHANGELOG.md — that is this surface's signature failure, and it is
  a finding against the release, not against the config.
- The predicted grouping is the grouping the config describes.
- Nothing in the predicted body describes something this release did not do.
- Nothing in the predicted body is empty when the delta is not.
