// Lint roll-up checks the one place a claim's status makes a claim about
// other claims: layout: banner. banner.html renders no edges footer at all
// (internal/render/components/banner.html) — it's meant to read as a
// standalone status callout for its whole module, mirroring the
// "draft · N/M locked" roll-up convention this project already uses for
// its own docs tabs (a roll-up only shows locked once every card under it
// is locked). So: a banner claim's Status must not be "locked" while any
// other claim sharing its Module is still "draft" — that would present a
// module as fully reviewed when it isn't. A banner with no module-mates at
// all has nothing to roll up and is never flagged.
//
// WHY THIS IS A WARNING AND NOT AN ERROR.
//
// It was an error, and as an error it DEADLOCKED any module in the ordinary
// shape: a locked banner, plus two draft claims. lint.RunAll is project-wide, so
// the one finding failed `check`, `check --validate`, `check --staged`, the
// pre-commit hook and CI simultaneously — and, because internal/lock.Lock lints
// the ABOUT-TO-BE-LOCKED form of the whole project, it also refused `claim lock`
// on BOTH drafts: locking either one leaves the other draft, so the banner's
// finding survives and the gate refuses. There was no legal move left. The only
// escape was `claim unlock` on the banner, which is a human-approved lifecycle
// action an agent may not invent a reason for — so the rule drove the agent
// toward the one thing the skills forbid.
//
// The deadlock is not a property of the rule's judgement, which is correct; it
// is a property of its SCOPE. The state it describes — a locked banner whose
// module gained a draft — is reached by editing DRAFT claims, and draft editing
// is deliberately unfrictioned in this release. A project-wide error is the
// wrong instrument for a condition that ordinary, permitted work creates.
//
// So the rule reports, and the ENFORCEMENT moves to the one place where it
// blocks nothing else: the banner's own lock attempt (see cmd/dossierx's
// evaluateLockGates, which escalates exactly this finding back to error severity
// when the claim being locked IS the banner it names). A banner still cannot be
// locked while its module holds a draft — the promise this rule exists for is
// intact — and a draft sibling can still be locked, which is what breaks the
// deadlock.
//
// There is precedent for this exact shape one file over in internal/lock: the
// comments-unresolved lint is a warning for the same reason ("a project-wide
// error-lint would freeze all locking and take render/check down with it") and
// the real refusal is candidate-scoped inside Lock.
package lint

import (
	"fmt"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, rollUpLint{})
}

type rollUpLint struct{}

func (rollUpLint) Name() string { return "roll-up" }

func (rollUpLint) Check(claims []model.Claim, _ *config.Config) []Finding {
	var findings []Finding
	for _, banner := range claims {
		if banner.Layout != model.LayoutBanner || banner.Status != model.StatusLocked {
			continue
		}
		for _, sibling := range claims {
			if sibling.ID == banner.ID || sibling.Module != banner.Module {
				continue
			}
			if sibling.Status != model.StatusLocked {
				// The message names BOTH ends of the problem — the banner and the
				// sibling holding it open — because the caller acts on the sibling,
				// not on the banner, and a finding that named only the claim it is
				// attached to left the agent with nothing to do next.
				findings = append(findings, Finding{
					LintName: "roll-up",
					ClaimID:  banner.ID,
					Message:  fmt.Sprintf("banner claim %q is locked but module %q still has a draft claim (%q); a roll-up must stay unlocked until every claim in the module is locked — lock %q (or unlock the banner) to clear this", banner.ID, banner.Module, sibling.ID, sibling.ID),
					Severity: SeverityWarning,
				})
				break
			}
		}
	}
	return findings
}
