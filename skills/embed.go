// Package skills embeds the DossierX Claude Code skill files (SKILL.md
// bundles teaching an agent how to author/review claims, derive Build
// Order, and ground code in claims) so they can be extracted into any
// consuming project via "dossierx skills export <dir>", without requiring
// this repository checked out alongside the installed binary.
//
// The go:embed directive lives here, in a file inside skills/ itself,
// rather than in cmd/dossierx: embed patterns must not contain ".." path
// elements, so a file under cmd/dossierx/ cannot embed ../../skills
// directly. cmd/dossierx/skills_embed.go imports this package instead.
package skills

import "embed"

// FS holds every skill directory (dossierx-claims, dossierx-build-order,
// dossierx-code-links), each containing one SKILL.md.
//
//go:embed dossierx-claims dossierx-build-order dossierx-code-links
var FS embed.FS
