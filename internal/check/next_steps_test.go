// next_steps_test.go pins the one property every next_actions line is supposed
// to have and one of them did not: the command it names must be a command that
// would SUCCEED as printed.
//
// That requirement got sharper in v0.3.0. The agent skills now instruct an agent
// to read next_actions and error.hint rather than re-deriving the lifecycle for
// itself, so a hint naming a command that then refuses is not a cosmetic wart —
// it is wrong advice an agent will act on, and the refusal it earns carries
// recoveries for a state it is not in.
package check_test

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/check"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// draftWithOpenThread is a draft claim carrying an unresolved comment thread —
// the lock gate's third refusal, and one the "still draft" hint cannot see.
func draftWithOpenThread(id string) string {
	return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
		"body: |\n  a draft claim.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n" +
		"comments:\n" +
		"  - id: c-aaa111\n    status: open\n    author: human\n" +
		"    created: \"2026-07-26T10:00:00Z\"\n    body: does this still hold?\n    edited: false\n"
}

func draftHint(t *testing.T, res check.Result) string {
	t.Helper()
	for _, h := range res.NextSteps {
		if strings.Contains(h, "still draft") {
			return h
		}
	}
	t.Fatalf("no \"still draft\" next step in %v", res.NextSteps)
	return ""
}

// The example must be a draft that would actually lock. draftIDs[0] is whichever
// draft sorts first, which is not the same thing: a draft carrying an open
// comment thread is refused by a gate the hint cannot see, and naming it
// produces a command that exists, reads as recommended, and then fails.
func TestNextSteps_DraftExampleSkipsAThreadBlockedClaim(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		// "blocked" sorts before "ok", so the old code picked it.
		"claims/blocked.yaml": draftWithOpenThread("widget.contract.blocked"),
		"claims/ok.yaml":      draftClaim("widget.contract.ok"),
	})

	hint := draftHint(t, check.Status(claims, cfg))
	if !strings.Contains(hint, "widget.contract.ok") {
		t.Fatalf("the example must name a lockable draft, got %q", hint)
	}
	if strings.Contains(hint, "widget.contract.blocked") {
		t.Fatalf("the example named a claim whose lock would be refused: %q", hint)
	}
	// The COUNT still covers every draft — the hint narrows the example, not the
	// tally, or a reader would think a blocked claim had stopped being draft.
	if !strings.Contains(hint, "2 claim(s) still draft") {
		t.Fatalf("the count must still cover every draft, got %q", hint)
	}
}

// A doctrine-facet dependency that is still draft is hub gating's refusal, the
// other gate the hint cannot see.
func TestNextSteps_DraftExampleSkipsAnUnlockedDoctrineDependency(t *testing.T) {
	const doctrineConfig = "schema_version: 1\nfacets:\n  - doctrine\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n" +
		"doctrine_facet: doctrine\n"

	cfg, claims := project(t, doctrineConfig, map[string]string{
		"claims/hub.yaml": "id: widget.doctrine.hub\nfacet: doctrine\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  the doctrine hub, still draft.\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		// Sorts first, and rests on the still-draft hub, so hub gating refuses it.
		"claims/blocked.yaml": "id: widget.contract.blocked\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  rests on a draft doctrine claim.\n" +
			"rests_on:\n  - widget.doctrine.hub\n" +
			"governed_by:\n  type: widget.doctrine.hub\n  reason: governed by the hub\n",
	})
	if !cfg.HubGatingEnabled() {
		t.Fatalf("fixture precondition: hub gating must be on")
	}

	hint := draftHint(t, check.Status(claims, cfg))
	if strings.Contains(hint, "widget.contract.blocked") {
		t.Fatalf("the example named a claim hub gating would refuse: %q", hint)
	}
}

// When NOTHING is lockable the hint must say so rather than invent an example.
// Silence about the reason would leave an agent retrying the same refused
// command against every draft in turn.
func TestNextSteps_NoLockableDraftSaysSoInsteadOfNamingOne(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/one.yaml": draftWithOpenThread("widget.contract.one"),
	})

	hint := draftHint(t, check.Status(claims, cfg))
	if strings.Contains(hint, "e.g.") {
		t.Fatalf("no draft is lockable; the hint must not name an example: %q", hint)
	}
	if !strings.Contains(hint, "none is lockable yet") {
		t.Fatalf("the hint must say why it named none, got %q", hint)
	}
}

// The ordinary case must not regress: with nothing blocking, the first draft is
// still the example.
func TestNextSteps_PlainDraftsStillGetAnExample(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": draftClaim("widget.contract.a"),
		"claims/b.yaml": draftClaim("widget.contract.b"),
	})

	hint := draftHint(t, check.Status(claims, cfg))
	if !strings.Contains(hint, "e.g. widget.contract.a") {
		t.Fatalf("expected the first lockable draft as the example, got %q", hint)
	}
}

// Guard against the fixtures above silently stopping to exercise what they
// claim: if these two stop disagreeing, the tests above become tautologies.
func TestNextSteps_FixturePreconditions(t *testing.T) {
	var blocked model.Claim
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/blocked.yaml": draftWithOpenThread("widget.contract.blocked"),
	})
	for _, c := range claims {
		if c.ID == "widget.contract.blocked" {
			blocked = c
		}
	}
	if !blocked.HasOpenThreads() {
		t.Fatalf("fixture precondition: the blocked claim must carry an open thread")
	}
	var _ *config.Config = cfg
}

// TestNextSteps_DraftExampleSkipsALintBlockedClaim is the case the other two
// gates cannot catch, and the ordinary one.
//
// The lint suite is evaluated against the ABOUT-TO-BE-LOCKED form: rest-on-
// locked says a LOCKED claim's rests_on targets must themselves be locked. So a
// module drafted alongside its own dependencies — the normal way a module gets
// written — lints completely clean while every claim in it that rests on a
// still-draft sibling would be refused by `claim lock` with lint_failed.
//
// "aaa" rests on "zzz" and sorts first, so the un-gated example was always the
// one command that could not work.
func TestNextSteps_DraftExampleSkipsALintBlockedClaim(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/a.yaml": "id: widget.contract.aaa\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  rests on a still-draft sibling.\n" +
			"rests_on:\n  - widget.contract.zzz\n" +
			"governed_by:\n  type: none\n  reason: fixture\n",
		"claims/z.yaml": draftClaim("widget.contract.zzz"),
	})

	// Precondition: the project itself lints clean, which is what makes this
	// hint reachable at all — check prints next steps only on a passing run.
	if res := check.Status(claims, cfg); len(res.LintErrors) != 0 {
		t.Fatalf("fixture precondition: the project must lint clean, got %d error(s)", len(res.LintErrors))
	}

	hint := draftHint(t, check.Status(claims, cfg))
	if strings.Contains(hint, "widget.contract.aaa") {
		t.Fatalf("the example named a claim whose lock would fail lint_failed: %q", hint)
	}
	if !strings.Contains(hint, "widget.contract.zzz") {
		t.Fatalf("the dependency IS lockable and must be the example, got %q", hint)
	}
}

// lockedWithOpenThread is a LOCKED claim carrying an unresolved thread. It is
// review_pending because an open thread is one of the three triggers.
func lockedWithOpenThread(id string) string {
	return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: locked\nreview_pending: true\nlayout: card\n" +
		"body: |\n  a locked claim.\n" +
		"governed_by:\n  type: none\n  reason: fixture\n" +
		"comments:\n" +
		"  - id: c-406a9f\n    status: open\n    author: human\n" +
		"    created: \"2026-07-26T10:00:00Z\"\n    body: I am not sure about this.\n    edited: false\n"
}

// The open-comment hint counted only LOCKED claims, so it disagreed with the two
// counts printed beside it in the same envelope. Reproduced with one open thread
// on a locked claim and one on a draft: open_comments said {"widget": 2}, the
// comments-unresolved lint fired on both, and next_steps said "1 claim(s) with
// open comment thread(s)" naming only the locked one — while `claim lock` on the
// draft then refuses with unresolved_comments.
func TestNextSteps_OpenCommentHintCountsDraftsToo(t *testing.T) {
	cfg, claims := project(t, baseConfig, map[string]string{
		"claims/draft.yaml":  draftWithOpenThread("widget.contract.draft"),
		"claims/locked.yaml": lockedWithOpenThread("widget.contract.overview"),
	})

	res := check.Status(claims, cfg)
	var hint string
	for _, h := range res.NextSteps {
		if strings.Contains(h, "open comment thread(s)") {
			hint = h
		}
	}
	if hint == "" {
		t.Fatalf("expected an open-comment next step, got %v", res.NextSteps)
	}
	if !strings.Contains(hint, "2 claim(s)") {
		t.Fatalf("the hint must count every claim a human has to act on, got %q", hint)
	}
	// It must agree with the count in the same envelope, which is the property
	// that was broken: two sources of truth, one report.
	total := 0
	for _, n := range res.OpenComments {
		total += n
	}
	if total != 2 {
		t.Fatalf("fixture precondition: open_comments must report 2, got %v", res.OpenComments)
	}
}
