package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// A missing store is policy v1. A readable draft prerequisite is therefore a
// dependency condition, not a refusal of the child's local approval. This is
// the cross-command contract that issue #68 found claim show had drifted from.
func TestClaimShowV1DraftParentAgreesWithSingletonPreview(t *testing.T) {
	root := t.TempDir()
	cfgPath := claimWriteFixture(t, root)
	const id = "widget.contract.timeout-budget"
	before := snapshotFiles(t, root)

	previewEnv, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", id, "--dry-run", "--reason", "fixture review")
	if err != nil {
		t.Fatalf("lock preview: %v", err)
	}
	var preview policyLockPreviewData
	envData(t, previewEnv, &preview)
	if preview.Blocked || len(preview.Evaluation.Verdicts) != 1 || !preview.Evaluation.Verdicts[0].LocalAdmissible {
		t.Fatalf("fixture must be locally admissible under v1: %+v", preview)
	}
	if got := preview.Evaluation.Verdicts[0].Conditions; len(got) != 1 || got[0].Kind != "dependency_unapproved" {
		t.Fatalf("preview must retain the draft-parent condition: %+v", got)
	}

	showEnv, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "show", id)
	if err != nil {
		t.Fatalf("claim show JSON: %v", err)
	}
	var show claimShowData
	envData(t, showEnv, &show)
	actions := strings.Join(show.NextActions, "\n")
	if strings.Contains(actions, "rest-on-locked") || strings.Contains(actions, "block locking") {
		t.Fatalf("show must not turn a v1 condition into a blocker: %v", show.NextActions)
	}
	if !strings.Contains(actions, "ready for local approval") || !strings.Contains(actions, "--dry-run") {
		t.Fatalf("show must describe local admissibility and preserve preview-first approval: %v", show.NextActions)
	}
	if got := show.Readiness.DependencyConditions; len(got) != 1 || got[0].Kind != "dependency_unapproved" {
		t.Fatalf("show readiness must retain the draft-parent condition: %+v", got)
	}

	textOut, _, err := execCLI(t, "--config", cfgPath, "claim", "show", id)
	if err != nil {
		t.Fatalf("claim show text: %v", err)
	}
	if strings.Contains(textOut, "rest-on-locked") || !strings.Contains(textOut, "ready for local approval") {
		t.Fatalf("text and JSON advice disagree:\n%s", textOut)
	}
	if after := snapshotFiles(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("preview/show mutated missing-store fixture\nbefore=%v\nafter=%v", before, after)
	}
}

func TestClaimShowLegacyDraftParentStillBlocks(t *testing.T) {
	root := t.TempDir()
	cfgPath := claimWriteFixture(t, root)
	storeFile := filepath.Join(root, "build", "ledger", "lock-store.json")
	if err := os.MkdirAll(filepath.Dir(storeFile), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{"version":3,"policy_version":0,"hashes":{},"receipts":{},"locked_at":{},"ledger":{}}`
	if err := os.WriteFile(storeFile, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}

	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "show", "widget.contract.timeout-budget")
	if err != nil {
		t.Fatalf("claim show: %v", err)
	}
	var show claimShowData
	envData(t, env, &show)
	actions := strings.Join(show.NextActions, "\n")
	if !strings.Contains(actions, "rest-on-locked") || !strings.Contains(actions, "block locking") {
		t.Fatalf("legacy policy must keep the existing prerequisite gate: %v", show.NextActions)
	}
	if strings.Contains(actions, "ready for local approval") {
		t.Fatalf("legacy policy must not be silently migrated: %v", show.NextActions)
	}
}

func TestClaimShowCorruptStoreCannotClaimReadinessAndWritesNothing(t *testing.T) {
	root := t.TempDir()
	cfgPath := claimWriteFixture(t, root)
	storeFile := filepath.Join(root, "build", "ledger", "lock-store.json")
	if err := os.MkdirAll(filepath.Dir(storeFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(storeFile, []byte("{ truncated"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotFiles(t, root)

	env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "show", "widget.contract.timeout-budget")
	if err != nil {
		t.Fatalf("claim show remains inspectable with explicit unavailable advice: %v", err)
	}
	var show claimShowData
	envData(t, env, &show)
	actions := strings.Join(show.NextActions, "\n")
	if !strings.Contains(actions, "cannot be assessed") || !strings.Contains(actions, "unreadable") {
		t.Fatalf("corrupt store must produce explicit unavailable advice: %v", show.NextActions)
	}
	if strings.Contains(actions, "ready to lock") || strings.Contains(actions, "ready for local approval") {
		t.Fatalf("corrupt store must never guess readiness: %v", show.NextActions)
	}
	if after := snapshotFiles(t, root); !reflect.DeepEqual(after, before) {
		t.Fatalf("claim show mutated a corrupt-store fixture\nbefore=%v\nafter=%v", before, after)
	}
}

func TestClaimShowPersistedV1UsesActualSingletonRefusals(t *testing.T) {
	const baseConfig = "schema_version: 1\nfacets:\n  - contract\n  - doctrine\nmodules:\n  - widget\nclaims_dir: claims\ndoctrine_facet: doctrine\n"
	claimYAML := func(id, facet, status string, rests, mirrors []string) string {
		body := "id: " + id + "\nfacet: " + facet + "\nmodule: widget\nstatus: " + status + "\nlayout: card\nbuild_role: behavior\nbody: |\n  fixture claim.\n"
		if len(rests) > 0 {
			body += "rests_on:\n"
			for _, dep := range rests {
				body += "  - " + dep + "\n"
			}
		}
		if len(mirrors) > 0 {
			body += "mirrors:\n"
			for _, dep := range mirrors {
				body += "  - " + dep + "\n"
			}
		}
		return body + "governed_by:\n  type: none\n  reason: fixture\n"
	}
	cases := []struct {
		name        string
		files       map[string]string
		wantRefusal string
		wantAdvice  string
		admissible  bool
	}{
		{"doctrine rests_on", map[string]string{
			"claims/child.yaml": claimYAML("widget.contract.child", "contract", "draft", []string{"widget.doctrine.hub"}, nil),
			"claims/hub.yaml":   claimYAML("widget.doctrine.hub", "doctrine", "draft", nil, nil),
		}, "doctrine_dependency_not_locked", "dependency widget.doctrine.hub is doctrine", false},
		{"doctrine mirror", map[string]string{
			"claims/child.yaml": claimYAML("widget.contract.child", "contract", "draft", nil, []string{"widget.doctrine.hub"}),
			"claims/hub.yaml":   claimYAML("widget.doctrine.hub", "doctrine", "draft", nil, nil),
		}, "doctrine_dependency_not_locked", "dependency widget.doctrine.hub is doctrine", false},
		{"missing dependency", map[string]string{
			"claims/child.yaml": claimYAML("widget.contract.child", "contract", "draft", []string{"widget.contract.gone"}, nil),
		}, "missing_dependency", "missing_dependency", false},
		{"retired dependency", map[string]string{
			"claims/child.yaml":  claimYAML("widget.contract.child", "contract", "draft", []string{"widget.contract.parent"}, nil),
			"claims/parent.yaml": claimYAML("widget.contract.parent", "contract", "retired", nil, nil),
		}, "retired_dependency", "retired_dependency", false},
		{"unreadable dependency", map[string]string{
			"claims/child.yaml":  claimYAML("widget.contract.child", "contract", "draft", []string{"widget.contract.parent"}, nil),
			"claims/parent.yaml": claimYAML("widget.contract.parent", "contract", "migration-unknown", nil, nil),
		}, "unreadable_dependency", "unreadable_dependency", false},
		{"cycle", map[string]string{
			"claims/child.yaml":  claimYAML("widget.contract.child", "contract", "draft", []string{"widget.contract.parent"}, nil),
			"claims/parent.yaml": claimYAML("widget.contract.parent", "contract", "draft", []string{"widget.contract.child"}, nil),
		}, "dependency_cycle", "dependency_cycle", false},
		{"unrelated lint scoped out", map[string]string{
			"claims/child.yaml":     claimYAML("widget.contract.child", "contract", "draft", nil, nil),
			"claims/unrelated.yaml": claimYAML("widget.unknown.unrelated", "unknown", "draft", nil, nil),
		}, "", "ready for local approval", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			cfgPath := writeCheckFixture(t, root, baseConfig, tc.files)
			writePolicyStore(t, root, 1)
			previewEnv, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "lock", "widget.contract.child", "--dry-run", "--reason", "fixture review")
			if err != nil {
				t.Fatalf("preview: %v", err)
			}
			var preview policyLockPreviewData
			envData(t, previewEnv, &preview)
			if len(preview.Evaluation.Verdicts) != 1 || preview.Evaluation.Verdicts[0].LocalAdmissible != tc.admissible {
				t.Fatalf("unexpected actual evaluator verdict: %+v", preview.Evaluation)
			}
			if tc.wantRefusal != "" && !strings.Contains(strings.Join(preview.Evaluation.Verdicts[0].Refusals, "\n"), tc.wantRefusal) {
				t.Fatalf("actual evaluator did not return %q: %+v", tc.wantRefusal, preview.Evaluation.Verdicts[0])
			}
			env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "show", "widget.contract.child")
			if err != nil {
				t.Fatalf("show: %v", err)
			}
			var show claimShowData
			envData(t, env, &show)
			actions := strings.Join(show.NextActions, "\n")
			if !strings.Contains(actions, tc.wantAdvice) {
				t.Fatalf("show advice %q does not expose actual verdict as %q", actions, tc.wantAdvice)
			}
			if !tc.admissible && strings.Contains(actions, "ready for local approval") {
				t.Fatalf("refused verdict became ready advice: %q", actions)
			}
		})
	}
}

func writePolicyStore(t *testing.T, root string, version int) {
	t.Helper()
	storeFile := filepath.Join(root, "build", "ledger", "lock-store.json")
	if err := os.MkdirAll(filepath.Dir(storeFile), 0o755); err != nil {
		t.Fatal(err)
	}
	raw := fmt.Sprintf(`{"version":3,"policy_version":%d,"hashes":{},"receipts":{},"locked_at":{},"ledger":{}}`, version)
	if err := os.WriteFile(storeFile, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestPolicyVerdictAdviceNeverTurnsRefusalsIntoReady(t *testing.T) {
	cases := []struct {
		name    string
		verdict lock.CandidateVerdict
		want    string
	}{
		{"target lint", lock.CandidateVerdict{ClaimID: "child", Refusals: []string{"lint:build-role-required-for-locked"}, LintFindings: []lint.Finding{{LintName: "build-role-required-for-locked", ClaimID: "child", Message: "missing role"}}}, "build-role-required-for-locked"},
		{"own roll-up", lock.CandidateVerdict{ClaimID: "child", Refusals: []string{"lint:roll-up"}, LintFindings: []lint.Finding{{LintName: "roll-up", ClaimID: "child", Message: "draft sibling"}}}, "roll-up"},
		{"open comments", lock.CandidateVerdict{ClaimID: "child", Refusals: []string{"unresolved_comments"}, OpenThreads: []string{"c-1"}}, "open comment thread"},
		{"doctrine rests_on or mirror", lock.CandidateVerdict{ClaimID: "child", Refusals: []string{"doctrine_dependency_not_locked:doctrine:hub"}}, "dependency hub is doctrine"},
		{"missing prerequisite", lock.CandidateVerdict{ClaimID: "child", Refusals: []string{"missing_dependency:gone"}}, "missing_dependency:gone"},
		{"unreadable prerequisite", lock.CandidateVerdict{ClaimID: "child", Refusals: []string{"unreadable_dependency:bad"}}, "unreadable_dependency:bad"},
		{"cycle", lock.CandidateVerdict{ClaimID: "child", Refusals: []string{"dependency_cycle:parent"}}, "dependency_cycle:parent"},
		{"standing record", lock.CandidateVerdict{ClaimID: "child", Refusals: []string{"standing_ledger_record"}}, "standing_ledger_record"},
		{"deleted record", lock.CandidateVerdict{ClaimID: "child", Refusals: []string{"ledger_record_deleted"}}, "ledger_record_deleted"},
		{"comment digest", lock.CandidateVerdict{ClaimID: "child", Refusals: []string{"comment_digest_unrecorded"}}, "comment_digest_unrecorded"},
		{"unknown refusal", lock.CandidateVerdict{ClaimID: "child", Refusals: []string{"future_policy_refusal"}}, "future_policy_refusal"},
		{"unknown empty refusal", lock.CandidateVerdict{ClaimID: "child"}, "unknown reason"},
		{"distinct lints", lock.CandidateVerdict{ClaimID: "child", Refusals: []string{"lint:first", "lint:second"}, LintFindings: []lint.Finding{{LintName: "first", ClaimID: "child", Message: "one"}, {LintName: "second", ClaimID: "child", Message: "two"}}}, "second on child: two"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Join(policyVerdictNextActions(tc.verdict), "\n")
			if !strings.Contains(got, tc.want) || strings.Contains(got, "ready to lock") || strings.Contains(got, "ready for local approval") {
				t.Fatalf("refusal advice is unsafe: %q", got)
			}
			if tc.name == "distinct lints" && (strings.Count(got, "first on child: one") != 1 || strings.Count(got, "second on child: two") != 1) {
				t.Fatalf("each lint finding must appear once: %q", got)
			}
		})
	}
}

func snapshotFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[rel] = string(raw)
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot fixture: %v", err)
	}
	return out
}

func TestClaimShowPolicyEvaluationScaleBounds(t *testing.T) {
	cfg := &config.Config{Facets: []string{"contract"}, Modules: []string{"shape"}}
	store := &lock.Store{PolicyVersion: lock.PolicyLocalApprovalV1}
	type shape struct {
		name           string
		claims         []model.Claim
		root           string
		wantConditions int
	}
	claim := func(id string, deps ...string) model.Claim {
		return model.Claim{ID: id, Facet: "contract", Module: "shape", Status: model.StatusDraft, Layout: model.LayoutCard, BuildRole: model.BuildRoleBehavior, Body: "bounded fixture", RestsOn: deps, Governed: model.Governed{Type: "none", Reason: "fixture"}}
	}

	makeChain := func(size int) shape {
		claims := make([]model.Claim, size)
		for i := range claims {
			id := fmt.Sprintf("shape.contract.chain%03d", i)
			deps := []string{}
			if i+1 < len(claims) {
				deps = []string{fmt.Sprintf("shape.contract.chain%03d", i+1)}
			}
			claims[i] = claim(id, deps...)
		}
		return shape{fmt.Sprintf("chain-%d", size), claims, claims[0].ID, 1}
	}
	diamond := []model.Claim{
		claim("shape.contract.diamondroot", "shape.contract.left", "shape.contract.right"),
		claim("shape.contract.left", "shape.contract.leaf"),
		claim("shape.contract.right", "shape.contract.leaf"),
		claim("shape.contract.leaf"),
	}
	wide := []model.Claim{claim("shape.contract.wideroot")}
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("shape.contract.wide%03d", i)
		wide[0].RestsOn = append(wide[0].RestsOn, id)
		wide = append(wide, claim(id))
	}
	makeDense := func(layers, width int) shape {
		claims := []model.Claim{}
		for layer := 0; layer < layers; layer++ {
			deps := []string{}
			if layer+1 < layers {
				for node := 0; node < width; node++ {
					deps = append(deps, fmt.Sprintf("shape.contract.dense%02d%02d", layer+1, node))
				}
			}
			for node := 0; node < width; node++ {
				claims = append(claims, claim(fmt.Sprintf("shape.contract.dense%02d%02d", layer, node), deps...))
			}
		}
		return shape{fmt.Sprintf("dense-%dx%d", layers, width), claims, claims[0].ID, width}
	}

	shapes := []shape{
		makeChain(32), makeChain(64), makeChain(128),
		{"diamond", diamond, diamond[0].ID, 2},
		{"wide-100", wide, wide[0].ID, 100},
		makeDense(8, 5), makeDense(16, 5), makeDense(24, 5),
	}
	for _, s := range shapes {
		t.Run(s.name, func(t *testing.T) {
			start := time.Now()
			evaluation := lock.EvaluateSetWithSemanticConflicts(s.claims, []string{s.root}, cfg, store, nil)
			verdict := evaluation.Verdicts[0]
			actions := policyVerdictNextActions(verdict)
			elapsed := time.Since(start)
			allocs := testing.AllocsPerRun(5, func() {
				e := lock.EvaluateSetWithSemanticConflicts(s.claims, []string{s.root}, cfg, store, nil)
				_ = policyVerdictNextActions(e.Verdicts[0])
			})
			bytes := len(strings.Join(actions, "\n"))
			if !verdict.LocalAdmissible || len(verdict.Refusals) != 0 {
				t.Fatalf("valid shape was refused: %+v", verdict)
			}
			if len(verdict.Conditions) != s.wantConditions || len(actions) != s.wantConditions+1 {
				t.Fatalf("condition/advice records = %d/%d, want %d/%d", len(verdict.Conditions), len(actions), s.wantConditions, s.wantConditions+1)
			}
			if elapsed > 250*time.Millisecond {
				t.Fatalf("singleton evaluation took %v, budget 250ms", elapsed)
			}
			if allocs > 50000 {
				t.Fatalf("singleton evaluation allocated %.0f objects, budget 50000", allocs)
			}
			if bytes > 64*1024 {
				t.Fatalf("advice output was %d bytes, budget 64KiB", bytes)
			}
			t.Logf("claims=%d elapsed=%v allocs/run=%.0f advice_bytes=%d", len(s.claims), elapsed, allocs, bytes)
		})
	}
}
