// Package skills embeds the DossierX agent skill files — SKILL.md bundles
// teaching an agent what DossierX is, how to author/review claims, how to work
// one module at a time within the caps, how to keep the constitution, how to
// implement and ground code in claims, how to run the review-comment loop with a
// human, and how to fold a project across an upgrade — so they can be extracted into any
// consuming project via "dossierx skills export", without requiring this
// repository checked out alongside the installed binary.
//
// These files are the SINGLE SOURCE for every harness. The exporter in
// cmd/dossierx/skills_embed.go derives Claude Code's .claude/skills tree, the
// marker-delimited section it maintains in a repo's AGENTS.md (Codex), and the
// self-contained docs/dossierx-agent-guide.md (Pi, Kimi, an editor, a human)
// from exactly these bytes. Nothing downstream is hand-maintained, so a fix
// written here reaches every harness at once.
//
// The go:embed directive lives here, in a file inside skills/ itself, rather
// than in cmd/dossierx: embed patterns must not contain ".." path elements, so
// a file under cmd/dossierx/ cannot embed ../../skills directly.
package skills

import "embed"

// FS holds every skill directory, each containing one SKILL.md.
//
// "dossierx" is the ROUTER and is deliberately listed first: it is the one an
// agent loads always and first (the contract, the nine nouns, the two roles,
// which companion to load for what), and the companions are loaded only when it
// sends the agent there. RouterName below is the machine-readable half of that
// statement; the exporter uses it to decide what goes into an always-on
// AGENTS.md section, which has a much smaller budget than a loaded-on-demand
// skill file.
//
//go:embed dossierx dossierx-claims dossierx-modules dossierx-constitution dossierx-comments dossierx-code-links dossierx-upgrading
var FS embed.FS

// RouterName is the directory name of the router skill — the one form that is
// short enough, and general enough, to be worth injecting into a repo's
// always-on agent instructions.
const RouterName = "dossierx"

// Order is the READING order the derived forms present the bundles in, which is
// not the same as the order a directory walk returns them in.
//
// It lives here, beside the go:embed pattern, because it is a statement about the
// content and not about the exporter: the router first because it is loaded
// always and first, then claims (the thing every other skill assumes), then
// modules (the harness every read goes through, and the caps an author meets
// while writing), then the constitution (the roof that must be locked before any
// claim can lock, and the project claims beside it), then the human loop, then
// code-links, which only applies once claims are locked, and last upgrading,
// which an agent needs only when a new binary meets an old corpus. A lexical walk
// would open the guide in an order that is not this reading order, and teach the
// reader to begin in the middle.
//
// The exporter cross-checks this list against the embedded directories and fails
// when they disagree, so a new bundle cannot be added without a decision about
// where in the reading order it belongs.
var Order = []string{
	RouterName,
	"dossierx-claims",
	"dossierx-modules",
	"dossierx-constitution",
	"dossierx-comments",
	"dossierx-code-links",
	"dossierx-upgrading",
}
