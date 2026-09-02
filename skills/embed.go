// Package skills embeds the DossierX agent skill files — SKILL.md bundles
// teaching an agent what DossierX is, how to author/review claims, how to
// derive a Build Order, how to ground code in claims, and how to run the
// review-comment loop with a human — so they can be extracted into any
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
// which companion to load for what), and the other five are loaded only when it
// sends the agent there. RouterName below is the machine-readable half of that
// statement; the exporter uses it to decide what goes into an always-on
// AGENTS.md section, which has a much smaller budget than a loaded-on-demand
// skill file.
//
//go:embed dossierx dossierx-claims dossierx-build-order dossierx-code-links dossierx-comments dossierx-theme
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
// always and first, then claims (the thing every other skill assumes), then the
// human loop, then theming, then the two skills that only apply once claims are
// locked. A lexical walk would open the guide on build-order — a skill for work
// that has not started yet — and teach the reader to begin in the middle.
//
// dossierx-theme sits where it does because it is a PROJECT-SETUP concern and
// not a post-lock one: a human asks for the viewer to look like their product
// early, usually before there is much in it to look at, and the two bundles
// after it both begin "once the claims are locked". It is last-but-two rather
// than first because it is also the only bundle that changes nothing the corpus
// says, so an agent reading straight through should meet the lifecycle before
// the paint.
//
// The exporter cross-checks this list against the embedded directories and fails
// when they disagree, so a seventh bundle cannot be added without a decision
// about where in the reading order it belongs.
var Order = []string{
	RouterName,
	"dossierx-claims",
	"dossierx-comments",
	"dossierx-theme",
	"dossierx-build-order",
	"dossierx-code-links",
}
