// flag_test.go covers flag.go's one gate that is not about the flag's own
// arguments: flagStructuredLayout, which decides whether a claim's rendered
// content lives in Body — the only field a flag-sourced reaudit rewrites
// (internal/reaudit.ProposeFlagDiff/Apply) — and therefore whether "dossierx
// claim flag" may run against it at all (DX-AUD-11).
//
// The rest of the flag command's CLI surface is exercised in
// flag_link_cli_test.go and lifecycle_audit_cli_test.go; what is pinned HERE is
// the v0.4.1 hole those suites could not see. Issue #25 made raw_html legal on
// every layout, and this gate was keyed off the layout NAME, so a locked
// card/banner/list/tree claim carrying raw_html classified as "body-only, safe
// to flag" — and a confirmed reaudit would have cleared review_pending while
// the markup the viewer actually renders stayed stale.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// TestFlagStructuredLayoutKeysOffRawHTMLNotLayout is the unit-level statement of
// the gate's contract: a claim carrying raw_html is never body-only, on ANY
// layout, and a claim that carries none is classified exactly as before.
//
// The "layout-less raw_html infers card" case is the second half of the #25
// fix: the old code inferred model.LayoutMockup from a non-empty RawHTML, which
// was unreachable while lint required layout: mockup and became a live, WRONG
// answer the moment it did not — it reported a layout the author never wrote,
// disagreeing with internal/catalog.inferLayout (rows -> table, steps -> steps,
// otherwise card), which is the layout the catalog and the renderer use for
// that same claim.
func TestFlagStructuredLayoutKeysOffRawHTMLNotLayout(t *testing.T) {
	const markup = "<div>a rendered mock</div>"
	cases := []struct {
		name  string
		claim model.Claim
		want  model.Layout
	}{
		{"card carrying raw_html is not body-only",
			model.Claim{Layout: model.LayoutCard, Body: "b", RawHTML: markup}, model.LayoutCard},
		{"banner carrying raw_html is not body-only",
			model.Claim{Layout: model.LayoutBanner, Body: "b", RawHTML: markup}, model.LayoutBanner},
		{"list carrying raw_html is not body-only",
			model.Claim{Layout: model.LayoutList, Body: "b", RawHTML: markup}, model.LayoutList},
		{"tree carrying raw_html is not body-only",
			model.Claim{Layout: model.LayoutTree, Body: "b", RawHTML: markup}, model.LayoutTree},
		{"layout-less raw_html infers card, never mockup",
			model.Claim{Body: "b", RawHTML: markup}, model.LayoutCard},
		{"plain card is body-only",
			model.Claim{Layout: model.LayoutCard, Body: "b"}, ""},
		{"plain banner is body-only",
			model.Claim{Layout: model.LayoutBanner, Body: "b"}, ""},
		{"layout-less body-only claim stays flaggable",
			model.Claim{Body: "b"}, ""},
		{"explicit table is refused",
			model.Claim{Layout: model.LayoutTable, Rows: []model.Row{{"name": "alpha"}}}, model.LayoutTable},
		{"layout-less rows infer table",
			model.Claim{Rows: []model.Row{{"name": "alpha"}}}, model.LayoutTable},
		{"layout-less steps infer steps",
			model.Claim{Steps: []string{"first"}}, model.LayoutSteps},
		{"explicit mockup is refused with or without markup",
			model.Claim{Layout: model.LayoutMockup, Body: "b"}, model.LayoutMockup},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := flagStructuredLayout(tc.claim); got != tc.want {
				t.Fatalf("flagStructuredLayout = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestFlagRefusesARawHTMLCardEndToEnd drives the same claim through the real
// command: a locked layout: card claim carrying reviewed raw_html must be
// refused with structured_layout, must keep review_pending untouched, and the
// --dry-run preview must report the same verdict the write path reaches (the
// preview-honesty contract preview_honesty_cli_test.go states for the passing
// direction — this is its failing twin).
func TestFlagRefusesARawHTMLCardEndToEnd(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims: %v", err)
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nmockup_modules:\n  - widget\nclaims_dir: claims\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	const id = "widget.contract.card"
	claimPath := filepath.Join(claimsDir, "card.yaml")
	claim := "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: locked\nlayout: card\n" +
		"body: |\n  a card claim that also renders markup.\n" +
		"raw_html: \"<div>a rendered mock</div>\"\nraw_html_reviewed: true\n" +
		"governed_by:\n  type: none\n  reason: fixture\n"
	if err := os.WriteFile(claimPath, []byte(claim), 0o644); err != nil {
		t.Fatalf("write claim: %v", err)
	}

	// The preview first: it must already say no, and say why.
	dr := dryRunOf(t, "--config", cfgPath, "claim", "flag", id,
		"--claim-says", "a", "--now-does", "b", "--reason", "c")
	found := false
	for _, p := range dr.Preconditions {
		if p.Name != "claim_is_body_only" {
			continue
		}
		found = true
		if p.OK {
			t.Fatalf("a card claim carrying raw_html is not body-only; preview said it was: %+v", p)
		}
		if !strings.Contains(p.Detail, "raw_html") {
			t.Fatalf("the refusal detail must name raw_html as the reason, got %q", p.Detail)
		}
	}
	if !found {
		t.Fatalf("expected a claim_is_body_only precondition in the preview, got %+v", dr.Preconditions)
	}

	// Then the write path, which must agree with it.
	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "flag", id,
		"--claim-says", "a", "--now-does", "b", "--reason", "c")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeStructuredLayout {
		t.Fatalf("expected structured_layout: a flag-sourced reaudit rewrites body only and would leave this claim's raw_html stale; got err=%v env=%+v", err, env.Error)
	}
	if !strings.Contains(env.Error.Message, "raw_html") {
		t.Fatalf("the refusal must name raw_html, not just the layout: %q", env.Error.Message)
	}

	after, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatalf("read claim: %v", err)
	}
	if strings.Contains(string(after), "review_pending: true") {
		t.Fatalf("a refused flag must leave the claim file alone, got:\n%s", after)
	}
	if _, err := os.Stat(filepath.Join(root, "build", "ledger", "flag-store.json")); err == nil {
		t.Fatalf("a refused flag must not record a pending-flag entry")
	}
}
