# Security Policy

## Supported Versions

DossierX is currently pre-1.0. Only the latest release is supported with security fixes;
earlier releases do not receive backports, including earlier patches of the current minor.

| Version | Supported |
|---|---|
| latest release | :white_check_mark: |
| anything earlier | :x: |

The table is deliberately written without a version number. It said `0.0.x` for three minor
series after that stopped being true, because a hard-coded series has to be remembered on every
release and nothing fails when it is not.

Once DossierX reaches 1.0, this will be updated to reflect an ongoing support policy across
minor versions.

## Reporting a Vulnerability

Please use GitHub's built-in **Private Vulnerability Reporting**: go to the repository's
**Security** tab and select **"Report a vulnerability."** This opens a private advisory
visible only to maintainers, where you can share details safely.

**Do not open public issues for security problems.** Public issues are visible to everyone,
including before a fix is available.

We aim for a **best-effort acknowledgment within 5 business days** of a report. This is not a
firm SLA — DossierX is maintained on a volunteer/best-effort basis — but we will do our best to
respond promptly, confirm the issue, and work with you on a fix and coordinated disclosure
timeline.
