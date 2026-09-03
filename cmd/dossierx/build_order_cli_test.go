// build_order_cli_test.go exercises "dossierx build-order propose|status|lock"
// in-process (see cli_inprocess_test.go's package doc comment for why
// in-process is the right model here: none of these RunE closures call
// os.Exit directly, unlike newDepsCmd/newReauditCmd, so there is nothing
// forcing a subprocess model for this command group). Covers: the
// completeness-gate refusal, a full propose -> status -> lock happy path
// against a synthetic fully-locked fixture spanning 3 phases, and stale
// detection after mutating a covered claim post-lock.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// boWriteConfig writes a minimal project.config.yaml (facets: [contract],
// modules: [module]) plus an empty claims/ dir under root.
func boWriteConfig(t *testing.T, root, module string) string {
	t.Helper()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - " + module + "\nclaims_dir: claims\n"
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}
	return cfgPath
}

// boWriteClaim writes one claim YAML file under root/claims.
func boWriteClaim(t *testing.T, root, filename, id, module, status, buildRole string, restsOn []string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("id: " + id + "\n")
	b.WriteString("facet: contract\nmodule: " + module + "\nstatus: " + status + "\n")
	if buildRole != "" {
		b.WriteString("build_role: " + buildRole + "\n")
	}
	b.WriteString("body: |\n  fixture claim for build-order CLI tests.\n")
	if len(restsOn) > 0 {
		b.WriteString("rests_on:\n")
		for _, r := range restsOn {
			b.WriteString("  - " + r + "\n")
		}
	}
	b.WriteString("governed_by:\n  type: none\n  reason: fixture claim, not backed by any real doctrine\n")

	path := filepath.Join(root, "claims", filename)
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write claim %s: %v", filename, err)
	}
	return path
}

// ---------------------------------------------------------------------
// completeness-gate refusal
// ---------------------------------------------------------------------

func TestCLI_BuildOrderPropose_RefusedWhenNotFullyLocked(t *testing.T) {
	root := t.TempDir()
	cfgPath := boWriteConfig(t, root, "widget")
	boWriteClaim(t, root, "schema.yaml", "widget.contract.schema", "widget", "locked", "schema", nil)
	boWriteClaim(t, root, "behavior.yaml", "widget.contract.behavior", "widget", "draft", "behavior", nil)

	out, _, err := execCLI(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if err == nil {
		t.Fatalf("expected propose to refuse a module that isn't fully locked (out: %s)", out)
	}
	if !strings.Contains(err.Error(), "widget.contract.behavior") {
		t.Fatalf("expected the non-locked claim id named in the error, got: %v", err)
	}

	if _, statErr := os.Stat(filepath.Join(root, "build", "build-order", "widget.json")); statErr == nil {
		t.Fatalf("expected no artifact file to be written on a refused propose")
	}
}

func TestCLI_BuildOrderPropose_RequiresModuleFlag(t *testing.T) {
	root := t.TempDir()
	cfgPath := boWriteConfig(t, root, "widget")

	if _, _, err := execCLI(t, "--config", cfgPath, "build-order", "propose"); err == nil {
		t.Fatalf("expected propose without --module to fail")
	}
	if _, _, err := execCLI(t, "--config", cfgPath, "build-order", "status"); err == nil {
		t.Fatalf("expected status without --module to fail")
	}
	if _, _, err := execCLI(t, "--config", cfgPath, "build-order", "lock", "--reason", "test fixture"); err == nil {
		t.Fatalf("expected lock without --module to fail")
	}
}

func TestCLI_BuildOrderStatus_NotProposedYet(t *testing.T) {
	root := t.TempDir()
	cfgPath := boWriteConfig(t, root, "widget")
	boWriteClaim(t, root, "schema.yaml", "widget.contract.schema", "widget", "locked", "schema", nil)

	out, _, err := execCLI(t, "--config", cfgPath, "build-order", "status", "--module", "widget")
	if err != nil {
		t.Fatalf("status: %v (out: %s)", err, out)
	}
	if !strings.Contains(out, "not proposed yet") {
		t.Fatalf("expected a friendly not-proposed-yet message, got: %s", out)
	}
}

func TestCLI_BuildOrderLock_RefusedWithoutPropose(t *testing.T) {
	root := t.TempDir()
	cfgPath := boWriteConfig(t, root, "widget")

	if out, _, err := execCLI(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "test fixture"); err == nil {
		t.Fatalf("expected lock to refuse when nothing has been proposed (out: %s)", out)
	}
}

// ---------------------------------------------------------------------
// full propose -> status -> lock happy path, spanning 3 phases, plus
// stale detection after mutating a covered claim post-lock.
// ---------------------------------------------------------------------

func TestCLI_BuildOrderFullLifecycle_ProposeStatusLockStale(t *testing.T) {
	root := t.TempDir()
	cfgPath := boWriteConfig(t, root, "widget")

	// A small dependency graph spanning 3 phases: schema -> behavior -> api,
	// plus an out-of-scope claim that must be excluded, not placed.
	boWriteClaim(t, root, "schema.yaml", "widget.contract.schema", "widget", "locked", "schema", nil)
	boWriteClaim(t, root, "behavior.yaml", "widget.contract.behavior", "widget", "locked", "behavior", []string{"widget.contract.schema"})
	boWriteClaim(t, root, "api.yaml", "widget.contract.api", "widget", "locked", "api", []string{"widget.contract.behavior"})
	boWriteClaim(t, root, "future.yaml", "widget.contract.future", "widget", "locked", "out-of-scope", nil)

	// propose
	proposeOut, _, err := execCLI(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if err != nil {
		t.Fatalf("propose: %v (out: %s)", err, proposeOut)
	}
	if !strings.Contains(proposeOut, "schema") || !strings.Contains(proposeOut, "behavior") || !strings.Contains(proposeOut, "api") {
		t.Fatalf("expected propose summary to mention all 3 phases, got: %s", proposeOut)
	}
	if !strings.Contains(proposeOut, "excluded") {
		t.Fatalf("expected propose summary to show the excluded count, got: %s", proposeOut)
	}
	artifactPath := filepath.Join(root, "build", "build-order", "widget.json")
	if _, statErr := os.Stat(artifactPath); statErr != nil {
		t.Fatalf("expected build-order artifact file to exist: %v", statErr)
	}

	// status: proposed, not locked, coverage shows 3 of 4 covered (1 excluded).
	statusOut, _, err := execCLI(t, "--config", cfgPath, "build-order", "status", "--module", "widget")
	if err != nil {
		t.Fatalf("status: %v (out: %s)", err, statusOut)
	}
	if !strings.Contains(statusOut, "locked:   false") {
		t.Fatalf("expected locked: false before locking, got: %s", statusOut)
	}
	if !strings.Contains(statusOut, "3 of 4 claim(s) covered (1 excluded as out-of-scope)") {
		t.Fatalf("expected coverage to report 3 of 4 covered, 1 excluded, got: %s", statusOut)
	}

	// lock
	lockOut, _, err := execCLI(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "test fixture")
	if err != nil {
		t.Fatalf("lock: %v (out: %s)", err, lockOut)
	}
	if !strings.Contains(lockOut, "locked at") {
		t.Fatalf("expected a locked-at confirmation, got: %s", lockOut)
	}

	// Re-locking immediately (unchanged) is refused: nothing to relock.
	if _, _, err := execCLI(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "test fixture"); err == nil {
		t.Fatalf("expected relocking an unchanged, already-locked artifact to be refused")
	}

	// status after lock: locked true, stale false.
	statusAfterLock, _, err := execCLI(t, "--config", cfgPath, "build-order", "status", "--module", "widget")
	if err != nil {
		t.Fatalf("status after lock: %v", err)
	}
	if !strings.Contains(statusAfterLock, "locked:   true") {
		t.Fatalf("expected locked: true after lock, got: %s", statusAfterLock)
	}
	if !strings.Contains(statusAfterLock, "stale:    false") {
		t.Fatalf("expected stale: false immediately after lock, got: %s", statusAfterLock)
	}

	// Mutate the schema claim's body on disk (simulating a post-lock edit)
	// and confirm status now reports staleness naming that claim.
	schemaPath := filepath.Join(root, "claims", "schema.yaml")
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read schema claim: %v", err)
	}
	mutated := strings.Replace(string(raw), "fixture claim for build-order CLI tests.", "fixture claim, mutated after lock.", 1)
	if err := os.WriteFile(schemaPath, []byte(mutated), 0o644); err != nil {
		t.Fatalf("rewrite schema claim: %v", err)
	}

	statusAfterMutate, _, err := execCLI(t, "--config", cfgPath, "build-order", "status", "--module", "widget")
	if err != nil {
		t.Fatalf("status after mutate: %v", err)
	}
	if !strings.Contains(statusAfterMutate, "stale:    true") {
		t.Fatalf("expected stale: true after mutating a covered claim, got: %s", statusAfterMutate)
	}
	if !strings.Contains(statusAfterMutate, "widget.contract.schema") {
		t.Fatalf("expected the mutated claim id named in stale output, got: %s", statusAfterMutate)
	}

	// A bare relock of the stale artifact is refused (FIX-13): it would
	// otherwise freeze the OLD phase order while clearing staleness. The
	// refusal points at re-proposing first.
	if bareRelock, _, err := execCLI(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "test fixture"); err == nil {
		t.Fatalf("expected a bare relock of a stale artifact to be refused (out: %s)", bareRelock)
	} else if !strings.Contains(err.Error(), "propose") {
		t.Fatalf("expected the stale-relock refusal to direct a re-propose, got: %v", err)
	}

	// Re-propose regenerates the order against the mutated claims, then lock
	// succeeds and clears staleness — the SKILL's re-propose-then-lock flow.
	if reproposeOut, _, err := execCLI(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("re-propose after going stale: %v (out: %s)", err, reproposeOut)
	}
	relockOut, _, err := execCLI(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "test fixture")
	if err != nil {
		t.Fatalf("expected lock to succeed after re-propose, got: %v (out: %s)", err, relockOut)
	}
	statusAfterRelock, _, err := execCLI(t, "--config", cfgPath, "build-order", "status", "--module", "widget")
	if err != nil {
		t.Fatalf("status after relock: %v", err)
	}
	if !strings.Contains(statusAfterRelock, "stale:    false") {
		t.Fatalf("expected stale: false after re-propose + lock, got: %s", statusAfterRelock)
	}
}

// ---------------------------------------------------------------------
// build-order show
//
// The three renderings are one payload, so the tests below are deliberately
// cross-checks rather than three independent snapshots: the mermaid export is
// checked against the JSON envelope of the SAME fixture, and the drawn edges
// against the artifact on disk. A snapshot of each rendering on its own would
// pass happily while the three described different orders.
// ---------------------------------------------------------------------

// boWriteConfigModules is boWriteConfig for a project with more than one
// module, which is what a cross-module rests_on edge needs to exist without
// being a dangling one.
func boWriteConfigModules(t *testing.T, root string, modules ...string) string {
	t.Helper()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("mkdir claims dir: %v", err)
	}
	cfg := "schema_version: 1\nfacets:\n  - contract\nmodules:\n"
	for _, m := range modules {
		cfg += "  - " + m + "\n"
	}
	cfg += "claims_dir: claims\n"
	cfgPath := filepath.Join(root, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write project.config.yaml: %v", err)
	}
	return cfgPath
}

// boShowFixture builds the project every "build-order show" test below runs
// against, proposes and locks widget's order, and returns the config path.
//
// It is one fixture rather than one per test because the shape is what makes
// the assertions mean anything, and it carries every edge classification the
// renderer distinguishes: an EMPTY phase (orientation), a phase with one claim
// and no edges (schema), a phase with two claims and a same-phase edge between
// them (behavior), two phases whose only edge leaves the phase and therefore
// draws a ghost (api, verification), a CROSS-MODULE edge that must be listed
// and never drawn, an edge to an OUT-OF-SCOPE claim that must be listed and
// never drawn, and an excluded block.
func boShowFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	cfgPath := boWriteConfigModules(t, root, "widget", "gadget")

	boWriteClaim(t, root, "gadget-x.yaml", "gadget.contract.x", "gadget", "locked", "schema", nil)

	boWriteClaim(t, root, "schema.yaml", "widget.contract.schema", "widget", "locked", "schema", nil)
	boWriteClaim(t, root, "behavior.yaml", "widget.contract.behavior", "widget", "locked", "behavior",
		[]string{"widget.contract.schema"})
	boWriteClaim(t, root, "report.yaml", "widget.contract.report", "widget", "locked", "behavior",
		[]string{"widget.contract.behavior", "gadget.contract.x", "widget.contract.later"})
	boWriteClaim(t, root, "api.yaml", "widget.contract.api", "widget", "locked", "api",
		[]string{"widget.contract.behavior"})
	boWriteClaim(t, root, "verify.yaml", "widget.contract.verify", "widget", "locked", "verification",
		[]string{"widget.contract.api"})
	boWriteClaim(t, root, "later.yaml", "widget.contract.later", "widget", "locked", "out-of-scope", nil)

	if _, out, err := execCLI(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("fixture setup: propose: %v (stderr: %s)", err, out)
	}
	if _, out, err := execCLI(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "fixture approval"); err != nil {
		t.Fatalf("fixture setup: lock: %v (stderr: %s)", err, out)
	}
	return cfgPath
}

// boShow runs "build-order show" under format and returns raw stdout.
func boShow(t *testing.T, cfgPath, format string, extra ...string) string {
	t.Helper()
	args := append([]string{"--config", cfgPath, "build-order", "show", "--module", "widget", "--format", format}, extra...)
	stdout, stderr, err := execCLI(t, args...)
	if err != nil {
		t.Fatalf("build-order show --format %s: %v (stderr: %s)", format, err, stderr)
	}
	return stdout
}

// boShowJSON runs "build-order show --format json" and decodes the payload.
func boShowJSON(t *testing.T, cfgPath string) buildOrderShowData {
	t.Helper()
	env, stderr, err := execCLIJSON(t, "--config", cfgPath, "build-order", "show", "--module", "widget")
	if err != nil {
		t.Fatalf("build-order show --format json: %v (stderr: %s)", err, stderr)
	}
	var data buildOrderShowData
	envData(t, env, &data)
	return data
}

// boNodeID re-derives the mermaid node-id sanitiser INDEPENDENTLY of
// internal/buildorder's, so a change to the rule shows up here as a mismatch
// rather than as two implementations agreeing about a new rule nobody decided.
func boNodeID(claimID string) string {
	return regexp.MustCompile(`[^A-Za-z0-9_]`).ReplaceAllString(claimID, "_")
}

func TestCLI_BuildOrderShow_JSONCarriesEveryPhaseWithMermaid(t *testing.T) {
	cfgPath := boShowFixture(t)
	data := boShowJSON(t, cfgPath)

	if len(data.Phases) != 6 {
		t.Fatalf("data.phases has %d entries, want 6 (five phases in the fixed order, then the excluded block) — a consumer indexing by position depends on it", len(data.Phases))
	}
	wantOrder := []string{"orientation", "schema", "behavior", "api", "verification", "excluded"}
	for i, want := range wantOrder {
		if data.Phases[i].Phase != want {
			t.Errorf("data.phases[%d].phase = %q, want %q", i, data.Phases[i].Phase, want)
		}
	}
	if data.Phases[5].Number != 0 {
		t.Errorf("the excluded block's number is %d, want 0 — it is not one of the five phases", data.Phases[5].Number)
	}
	for i := 0; i < 5; i++ {
		if data.Phases[i].Number != i+1 {
			t.Errorf("data.phases[%d].number = %d, want %d", i, data.Phases[i].Number, i+1)
		}
		if data.Phases[i].Definition == "" {
			t.Errorf("data.phases[%d] (%s) carries no definition", i, data.Phases[i].Phase)
		}
	}

	if !data.Locked {
		t.Errorf("data.locked = false on an order the fixture locked")
	}
	if data.Stale {
		t.Errorf("data.stale = true immediately after locking")
	}
	if data.LockedAt == "" {
		t.Errorf("data.locked_at is empty on a locked order")
	}

	// Mermaid is present exactly where there is something to draw, and absent
	// exactly where there is not. Both halves are asserted: a generator that
	// emitted a header-only chunk for an empty phase would make a diagram that
	// FAILED to generate indistinguishable from one that had nothing to draw.
	for _, p := range data.Phases {
		hasDiagram := p.Mermaid != ""
		wantDiagram := p.Number != 0 && len(p.Claims) > 0
		if hasDiagram != wantDiagram {
			t.Errorf("phase %q: mermaid non-empty = %v, want %v (claims: %d, number: %d)", p.Phase, hasDiagram, wantDiagram, len(p.Claims), p.Number)
		}
		if hasDiagram && !strings.Contains(p.Mermaid, "flowchart TD") {
			t.Errorf("phase %q: a non-empty mermaid string with no \"flowchart TD\" in it is not a diagram:\n%s", p.Phase, p.Mermaid)
		}
	}

	// The levels are a REGROUPING of the claims, never a different set: a
	// diagram whose L0/L1 rows lost or invented a claim would be describing an
	// order the artifact does not contain.
	//
	// The five PHASES only. The excluded block is not an order — it is the list
	// of claims deliberately kept out of one — so it carries its claims and no
	// levels at all, which is asserted on its own terms below.
	for _, p := range data.Phases[:5] {
		var flat []string
		for _, layer := range p.Levels {
			flat = append(flat, layer...)
		}
		if len(flat) != len(p.Claims) {
			t.Errorf("phase %q: levels flatten to %d claim(s), claims lists %d", p.Phase, len(flat), len(p.Claims))
			continue
		}
		for i := range flat {
			if flat[i] != p.Claims[i] {
				t.Errorf("phase %q: levels flattened is %v, claims is %v — the two must be the same sequence", p.Phase, flat, p.Claims)
				break
			}
		}
	}

	// The excluded block: named claims, and every diagram-derived field empty.
	// Naming them is the point — a claim dropped from a build order with only a
	// count to show for it is the silent disappearance Artifact.Excluded exists
	// to prevent.
	excluded := data.Phases[5]
	if len(excluded.Claims) != 1 || excluded.Claims[0] != "widget.contract.later" {
		t.Errorf("the excluded block lists %v, want [widget.contract.later]", excluded.Claims)
	}
	if len(excluded.Levels) != 0 || len(excluded.Ghosts) != 0 || len(excluded.CrossModule) != 0 || len(excluded.ExcludedDeps) != 0 || excluded.Locked != 0 || excluded.Mermaid != "" {
		t.Errorf("the excluded block carries diagram fields it has no diagram for: %+v", excluded)
	}

	// The edge classifications the fixture exists to exercise.
	behavior := data.Phases[2]
	if got := behavior.CrossModule["gadget"]; len(got) != 1 || got[0] != "gadget.contract.x" {
		t.Errorf("behavior.cross_module[\"gadget\"] = %v, want [gadget.contract.x]", got)
	}
	if got := behavior.ExcludedDeps; len(got) != 1 || got[0] != "widget.contract.later" {
		t.Errorf("behavior.excluded_deps = %v, want [widget.contract.later]", got)
	}
	api := data.Phases[3]
	if len(api.Ghosts) != 1 || api.Ghosts[0].ID != "widget.contract.behavior" || api.Ghosts[0].Phase != "behavior" {
		t.Errorf("api.ghosts = %+v, want one ghost widget.contract.behavior in phase behavior", api.Ghosts)
	}

	// data.path names a file that exists, at an ABSOLUTE path. That it is the
	// same path the writing verb reported is the separate, stronger assertion
	// in TestCLI_BuildOrderShow_PathEqualsTheOneProposeReports.
	if !filepath.IsAbs(data.Path) {
		t.Errorf("data.path = %q, which is not absolute; an agent that opens it resolves it against its own cwd, which --config makes routinely different from the project directory", data.Path)
	}
	if _, statErr := os.Stat(data.Path); statErr != nil {
		t.Errorf("data.path = %q does not exist: %v", data.Path, statErr)
	}
}

// TestCLI_BuildOrderShow_PathEqualsTheOneProposeReports is the other half of
// the path assertion: show must name the SAME file propose wrote, byte for
// byte, or an agent following the two commands' output ends up looking at two
// different paths for one artifact.
func TestCLI_BuildOrderShow_PathEqualsTheOneProposeReports(t *testing.T) {
	root := t.TempDir()
	cfgPath := boWriteConfig(t, root, "widget")
	boWriteClaim(t, root, "schema.yaml", "widget.contract.schema", "widget", "locked", "schema", nil)

	env, stderr, err := execCLIJSON(t, "--config", cfgPath, "build-order", "propose", "--module", "widget")
	if err != nil {
		t.Fatalf("propose: %v (stderr: %s)", err, stderr)
	}
	var proposed buildOrderProposeData
	envData(t, env, &proposed)

	showEnv, _, showErr := execCLIJSON(t, "--config", cfgPath, "build-order", "show", "--module", "widget")
	if showErr != nil {
		t.Fatalf("show: %v", showErr)
	}
	var shown buildOrderShowData
	envData(t, showEnv, &shown)

	if shown.Path != proposed.Path {
		t.Fatalf("build-order show reports data.path %q and build-order propose reported %q for the same module", shown.Path, proposed.Path)
	}
	if shown.Locked {
		t.Errorf("show reports locked:true for an order that was proposed and never locked")
	}
}

func TestCLI_BuildOrderShow_MermaidPrintsOneFlowchartPerPhaseWithDefinitions(t *testing.T) {
	cfgPath := boShowFixture(t)
	data := boShowJSON(t, cfgPath)
	stdout := boShow(t, cfgPath, "mermaid")

	// The chunk count is derived from the SAME run's JSON, never from a
	// literal: a fixture edit that adds a phase then moves both numbers
	// together, and a generator that stopped emitting one moves only one.
	var withDiagram []buildOrderShowPhaseData
	for _, p := range data.Phases {
		if p.Mermaid != "" {
			withDiagram = append(withDiagram, p)
		}
	}
	if len(withDiagram) == 0 {
		t.Fatalf("the fixture produced no diagrams at all, so this test would assert nothing")
	}

	chunks := strings.Split(strings.TrimRight(stdout, "\n"), "\n\n")
	if len(chunks) != len(withDiagram) {
		t.Fatalf("--format mermaid printed %d blank-line-separated chunk(s) and --format json reports %d phase(s) with a diagram", len(chunks), len(withDiagram))
	}

	roleOf := map[string]model.BuildRole{
		"orientation":  model.BuildRoleOrientation,
		"schema":       model.BuildRoleSchema,
		"behavior":     model.BuildRoleBehavior,
		"api":          model.BuildRoleAPI,
		"verification": model.BuildRoleVerification,
	}

	for i, chunk := range chunks {
		p := withDiagram[i]
		lines := strings.Split(chunk, "\n")
		if len(lines) < 5 {
			t.Fatalf("chunk %d has %d line(s); a flowchart is four %%%% lines and then \"flowchart TD\":\n%s", i, len(lines), chunk)
		}
		for n := 0; n < 4; n++ {
			if !strings.HasPrefix(lines[n], "%% ") {
				t.Fatalf("chunk %d line %d is %q, want a %%%% comment line", i, n+1, lines[n])
			}
		}
		if lines[4] != "flowchart TD" {
			t.Fatalf("chunk %d line 5 is %q, want \"flowchart TD\"", i, lines[4])
		}
		if strings.HasPrefix(lines[5], "%%") {
			t.Fatalf("chunk %d has a fifth %%%% line; the header is exactly four:\n%s", i, chunk)
		}

		wantHead := "%% phase " + strconv.Itoa(p.Number) + " of 5: " + p.Phase
		if lines[0] != wantHead {
			t.Errorf("chunk %d line 1 = %q, want %q", i, lines[0], wantHead)
		}

		// Line 2 is the phase definition BYTE FOR BYTE, read from
		// internal/model rather than restated here: this line is the one place
		// the export tells a reader what the phase means, and a paraphrase in
		// this test would let the export drift to a paraphrase too.
		role, known := roleOf[p.Phase]
		if !known {
			t.Fatalf("chunk %d names phase %q, which is not one of the five", i, p.Phase)
		}
		if want := "%% " + model.BuildRoleDefinition(role); lines[1] != want {
			t.Errorf("chunk %d line 2 is not the phase definition verbatim.\n  got:  %s\n  want: %s", i, lines[1], want)
		}

		// Line 3 is the counts, reconstructed from the SAME run's JSON
		// counters, so the diagram's summary cannot say one thing while the
		// machine payload says another.
		wantCounts := "%% " + boCountsLine(len(p.Claims), len(p.Levels), p.Locked)
		if lines[2] != wantCounts {
			t.Errorf("chunk %d line 3 = %q, want %q (claims %d, levels %d, locked %d)", i, lines[2], wantCounts, len(p.Claims), len(p.Levels), p.Locked)
		}

		wantLegend := "%% solid arrow: rests on, same phase. dotted arrow: rests on an earlier phase (ghost node)."
		if lines[3] != wantLegend {
			t.Errorf("chunk %d line 4 = %q, want the fixed legend %q", i, lines[3], wantLegend)
		}

		// And the chunk is exactly what the envelope carried for that phase.
		if strings.TrimRight(p.Mermaid, "\n") != chunk {
			t.Errorf("chunk %d differs from data.phases[%q].mermaid; --format mermaid must print the envelope's strings and nothing else", i, p.Phase)
		}
	}
}

// boCountsLine re-derives the counts summary from the machine payload's own
// counters, independently of internal/buildorder's PhaseView.Counts.
func boCountsLine(claims, levels, locked int) string {
	if claims == 0 {
		return "0 claims"
	}
	word := func(n int, w string) string {
		if n == 1 {
			return w
		}
		return w + "s"
	}
	return strconv.Itoa(claims) + " " + word(claims, "claim") +
		" · " + strconv.Itoa(levels) + " " + word(levels, "level") +
		" · " + strconv.Itoa(locked) + " locked"
}

func TestCLI_BuildOrderShow_TextTable(t *testing.T) {
	cfgPath := boShowFixture(t)
	out := boShow(t, cfgPath, "text")

	for _, want := range []string{
		"build-order show: widget (locked ",
		", not stale)",
		"phase 1 of 5  orientation    0 claims",
		"phase 2 of 5  schema         1 claim · 1 level · 1 locked",
		"phase 3 of 5  behavior       2 claims · 2 levels · 2 locked",
		"L0  widget.contract.behavior",
		"L1  widget.contract.report",
		"claims/report.yaml",
		"rests on: widget.contract.schema (schema)",
		"rests on: widget.contract.behavior",
		"cross-module: gadget (1): gadget.contract.x",
		"rests on out-of-scope (1): widget.contract.later",
		"excluded      1 claim: widget.contract.later",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("--format text output is missing %q:\n%s", want, out)
		}
	}

	// The table is prose, not the machine surface: an envelope's braces here
	// would mean the leaf ignored its own --format.
	if strings.Contains(out, "\"ok\"") {
		t.Errorf("--format text emitted a JSON envelope:\n%s", out)
	}
	// A row must never trail into whitespace, which is invisible to the reader
	// and loud in every diff and golden the output lands in.
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line != strings.TrimRight(line, " \t") {
			t.Errorf("a row ends in whitespace: %q", line)
		}
	}
}

// TestCLI_BuildOrderShow_TextTableSeparatesALongFileFromItsEdges pins the one
// property boShowFixture structurally cannot test.
//
// Every claim in that fixture lives at claims/<name>.yaml, at most 26
// characters, so every file column pads and the columns look separated no
// matter how the row is formatted. A project that nests its claims — the shape
// claims_dir plus subdirectories produces, and the shape the reference client
// has — pushes the path past the column width, and a format string that relies
// on the PAD to separate the columns then emits "behavior.yamlrests on:" as one
// token: the reader cannot see where the path ends, and neither can anything
// parsing the table.
//
// The assertion is on the join specifically, not on the two halves. The
// existing table test checks strings.Contains for the file and for "rests on:"
// separately, and both of those pass on the glued row.
func TestCLI_BuildOrderShow_TextTableSeparatesALongFileFromItsEdges(t *testing.T) {
	root := t.TempDir()
	cfgPath := boWriteConfig(t, root, "widget")

	// 46 characters, comfortably past the file column, and a real directory
	// layout rather than a padded name.
	nested := filepath.Join("deeply", "nested", "subdirectory")
	if err := os.MkdirAll(filepath.Join(root, "claims", nested), 0o755); err != nil {
		t.Fatalf("mkdir nested claims dir: %v", err)
	}
	boWriteClaim(t, root, "schema.yaml", "widget.contract.schema", "widget", "locked", "schema", nil)
	boWriteClaim(t, root, filepath.Join(nested, "behavior.yaml"), "widget.contract.behavior", "widget", "locked", "behavior",
		[]string{"widget.contract.schema"})

	if _, stderr, err := execCLI(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("propose: %v (stderr: %s)", err, stderr)
	}
	if _, stderr, err := execCLI(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "fixture approval"); err != nil {
		t.Fatalf("lock: %v (stderr: %s)", err, stderr)
	}

	out := boShow(t, cfgPath, "text")

	// The path as the table prints it: slash-separated regardless of host,
	// because displayPath renders a claim's source relative to the project in
	// the catalog's own separator.
	wantFile := "claims/deeply/nested/subdirectory/behavior.yaml"
	if !strings.Contains(out, wantFile) {
		t.Fatalf("the table does not carry the nested claim's path %q, so this test asserts nothing:\n%s", wantFile, out)
	}
	if len(wantFile) < 30 {
		t.Fatalf("the fixture path is %d characters, short enough to pad; this test only means something past the file column's width", len(wantFile))
	}
	if strings.Contains(out, wantFile+"rests on:") {
		t.Errorf("the file column runs straight into the edge list — a reader cannot see where the path ends:\n%s", out)
	}

	// Stated positively as well, so a future format change that separated them
	// with something other than a space still has to say what it does.
	if !strings.Contains(out, wantFile+" rests on: widget.contract.schema (schema)") {
		t.Errorf("the row does not read %q followed by a space and its edges:\n%s", wantFile, out)
	}

	// The trailing-whitespace rule the table test fixes still holds: the row
	// with no drawn edge (the schema claim) must not end in the separator.
	for _, line := range strings.Split(strings.TrimRight(out, "\n"), "\n") {
		if line != strings.TrimRight(line, " \t") {
			t.Errorf("a row ends in whitespace: %q", line)
		}
	}
}

// TestCLI_BuildOrderShow_NotProposed pins a DELIBERATE divergence from
// "build-order status", which answers this same state with ok:true,
// proposed:false and exit 0.
//
// The two verbs are asked different questions. status asks "is there one?", for
// which no is a complete and successful answer. show asks "give it to me", and
// under --format text or --format mermaid an empty stdout at exit 0 is
// indistinguishable from a module that genuinely has nothing in it — a caller
// redirecting the export into a file would write an empty .mmd and never learn
// why. The refusal is also what names the unproposed module, which the
// back-compat check for this release relies on.
func TestCLI_BuildOrderShow_NotProposed(t *testing.T) {
	root := t.TempDir()
	cfgPath := boWriteConfig(t, root, "widget")
	boWriteClaim(t, root, "schema.yaml", "widget.contract.schema", "widget", "locked", "schema", nil)

	env, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "show", "--module", "widget")
	if err == nil {
		t.Fatalf("expected show to refuse a module with no build order; envelope: %+v", env)
	}
	if env.OK {
		t.Errorf("the envelope reports ok:true on a refusal")
	}
	if env.Error == nil || env.Error.Code != cliout.CodeNotProposed {
		t.Fatalf("expected error.code %q, got %+v", cliout.CodeNotProposed, env.Error)
	}
	if !strings.Contains(env.Error.Message, "widget") {
		t.Errorf("the refusal does not name the module: %q", env.Error.Message)
	}
	if !strings.Contains(env.Error.Hint, "build-order propose --module widget") {
		t.Errorf("the refusal's hint does not name the recovery: %q", env.Error.Hint)
	}

	// The sibling verb, in the same state, on purpose: this assertion is the
	// record that the two answers differ by decision and not by accident.
	statusEnv, _, statusErr := execCLIJSON(t, "--config", cfgPath, "build-order", "status", "--module", "widget")
	if statusErr != nil {
		t.Fatalf("build-order status must still answer the unproposed module successfully, got: %v", statusErr)
	}
	if !statusEnv.OK {
		t.Fatalf("build-order status reported ok:false for an unproposed module; the divergence this test pins runs the other way")
	}
}

func TestCLI_BuildOrderShow_BadFormatIsUnsupportedFormat(t *testing.T) {
	cfgPath := boShowFixture(t)

	env, _, err := execCLIJSON(t, "--config", cfgPath, "build-order", "show", "--module", "widget", "--format", "svg")
	if err == nil {
		t.Fatalf("expected --format svg to be refused; envelope: %+v", env)
	}
	if env.Error == nil || env.Error.Code != cliout.CodeUnsupportedFormat {
		t.Fatalf("expected error.code %q, got %+v", cliout.CodeUnsupportedFormat, env.Error)
	}
	for _, want := range []string{"json", "text", "mermaid", "svg"} {
		if !strings.Contains(env.Error.Message, want) {
			t.Errorf("the refusal does not name %q: %q", want, env.Error.Message)
		}
	}

	// The third value belongs to this leaf and nowhere else: the root's
	// persistent --format still accepts two, so a caller who learned "mermaid"
	// here must not be able to spend it on another verb and get prose.
	if _, _, rootErr := execCLI(t, "--config", cfgPath, "--format", "mermaid", "claim", "list"); rootErr == nil {
		t.Errorf("--format mermaid was accepted by a leaf that has no diagram to draw")
	}
}

// TestCLI_BuildOrderShow_EdgeSetEqualsSameModuleEdges holds the drawn edges to
// the artifact's own rests_on lists.
//
// It reads the ARTIFACT rather than the payload, because the payload
// deliberately does not carry per-claim rests_on: the point is that the diagram
// is a rendering of the locked bytes, and a comparison against a second
// derivation of those bytes would only prove the renderer agrees with itself.
func TestCLI_BuildOrderShow_EdgeSetEqualsSameModuleEdges(t *testing.T) {
	cfgPath := boShowFixture(t)
	data := boShowJSON(t, cfgPath)

	raw, err := os.ReadFile(data.Path)
	if err != nil {
		t.Fatalf("read the artifact at %s: %v", data.Path, err)
	}
	var artifact struct {
		Phases []struct {
			Phase  string `json:"phase"`
			Claims []struct {
				ID      string   `json:"id"`
				RestsOn []string `json:"rests_on"`
			} `json:"claims"`
		} `json:"phases"`
	}
	if err := json.Unmarshal(raw, &artifact); err != nil {
		t.Fatalf("decode the artifact: %v", err)
	}

	phaseOf := map[string]string{}
	phaseNumber := map[string]int{}
	order := []string{"orientation", "schema", "behavior", "api", "verification"}
	numberOf := map[string]int{}
	for i, name := range order {
		numberOf[name] = i + 1
	}
	for _, block := range artifact.Phases {
		for _, c := range block.Claims {
			phaseOf[c.ID] = block.Phase
			phaseNumber[c.ID] = numberOf[block.Phase]
		}
	}

	// Expected: every same-module rests_on edge whose target is in this phase
	// (solid) or in an earlier one (dotted). Everything else is listed, never
	// drawn.
	wantSolid := map[string]bool{}
	wantDotted := map[string]bool{}
	for _, block := range artifact.Phases {
		for _, c := range block.Claims {
			for _, dep := range c.RestsOn {
				edge := boNodeID(dep) + "->" + boNodeID(c.ID)
				switch {
				case phaseOf[dep] == block.Phase:
					wantSolid[block.Phase+"|"+edge] = true
				case phaseOf[dep] != "" && phaseNumber[dep] < numberOf[block.Phase]:
					wantDotted[block.Phase+"|"+edge] = true
				}
			}
		}
	}
	if len(wantSolid) == 0 || len(wantDotted) == 0 {
		t.Fatalf("the fixture produced %d solid and %d dotted expected edge(s); a comparison with neither kind present asserts nothing", len(wantSolid), len(wantDotted))
	}

	gotSolid := map[string]bool{}
	gotDotted := map[string]bool{}
	solidRE := regexp.MustCompile(`^\s+([A-Za-z0-9_]+) --> ([A-Za-z0-9_]+)$`)
	dottedRE := regexp.MustCompile(`^\s+([A-Za-z0-9_]+) -\.-> ([A-Za-z0-9_]+)$`)
	for _, p := range data.Phases {
		for _, line := range strings.Split(p.Mermaid, "\n") {
			if m := solidRE.FindStringSubmatch(line); m != nil {
				gotSolid[p.Phase+"|"+m[1]+"->"+m[2]] = true
			}
			if m := dottedRE.FindStringSubmatch(line); m != nil {
				gotDotted[p.Phase+"|"+m[1]+"->"+m[2]] = true
			}
		}
	}

	for edge := range wantSolid {
		if !gotSolid[edge] {
			t.Errorf("missing solid edge %s", edge)
		}
	}
	for edge := range gotSolid {
		if !wantSolid[edge] {
			t.Errorf("drawn solid edge %s has no same-phase rests_on behind it", edge)
		}
	}
	for edge := range wantDotted {
		if !gotDotted[edge] {
			t.Errorf("missing dotted (ghost) edge %s", edge)
		}
	}
	for edge := range gotDotted {
		if !wantDotted[edge] {
			t.Errorf("drawn dotted edge %s has no earlier-phase rests_on behind it", edge)
		}
	}

	// The two edges the fixture carries that must NOT be drawn anywhere: a
	// cross-module target and an out-of-scope one. A renderer that drew either
	// would be inventing a node for a claim that is not in this phase.
	for _, p := range data.Phases {
		for _, forbidden := range []string{boNodeID("gadget.contract.x"), boNodeID("widget.contract.later")} {
			if strings.Contains(p.Mermaid, forbidden) {
				t.Errorf("phase %q draws %s, which is listed (cross_module / excluded_deps) and never drawn:\n%s", p.Phase, forbidden, p.Mermaid)
			}
		}
	}
}

// TestCLI_BuildOrderShow_StaleOrderIsShownWithAWarning pins the third artifact
// state this leaf reports on.
//
// A stale order is still the order that was approved, so it is SHOWN rather
// than refused — a reader asking what was approved is entitled to the answer.
// What must not happen is showing it silently: the machine surface says so in
// warnings[] and the human surface says so in the header, because an
// implementer following a sequence whose claims have moved underneath it is the
// harm this whole staleness concept exists to prevent.
//
// ALL THREE formats are asserted, --format mermaid included. That one carried
// no staleness signal on any stream: stdout is the diagrams by contract, and
// nothing was written to stderr either, so the export loop in the acceptance
// procedure (`... --format mermaid > out/<m>.mmd` over every module) wrote a
// file for a stale module byte-indistinguishable from a current one and told
// nobody. Its stdout must stay exactly the diagrams — the parse harness splits
// on blank lines and rejects a chunk with no diagram keyword — so the assertion
// below is deliberately two-sided: the note is on stderr, and stdout is
// unchanged from what a non-stale run of the same module emits.
func TestCLI_BuildOrderShow_StaleOrderIsShownWithAWarning(t *testing.T) {
	cfgPath := boShowFixture(t)
	root := filepath.Dir(cfgPath)

	// Captured while the order is still fresh, so the stdout comparison below
	// is against this binary's own non-stale export rather than a transcription.
	freshMermaid := boShow(t, cfgPath, "mermaid")
	if !strings.Contains(freshMermaid, "flowchart TD") {
		t.Fatalf("the fixture's mermaid export carries no diagram, so the stdout half of this test would assert nothing:\n%s", freshMermaid)
	}

	schemaPath := filepath.Join(root, "claims", "schema.yaml")
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read the schema claim: %v", err)
	}
	mutated := strings.Replace(string(raw), "fixture claim for build-order CLI tests.", "fixture claim, edited after the order was locked.", 1)
	if mutated == string(raw) {
		t.Fatalf("the fixture claim body did not change, so this test would assert nothing about staleness")
	}
	if err := os.WriteFile(schemaPath, []byte(mutated), 0o644); err != nil {
		t.Fatalf("rewrite the schema claim: %v", err)
	}

	env, stderr, err := execCLIJSON(t, "--config", cfgPath, "build-order", "show", "--module", "widget")
	if err != nil {
		t.Fatalf("show on a stale order must still succeed: %v (stderr: %s)", err, stderr)
	}
	if !env.OK {
		t.Fatalf("show reported ok:false for a stale order; a stale order is shown, not refused")
	}
	var data buildOrderShowData
	envData(t, env, &data)
	if !data.Stale {
		t.Fatalf("data.stale = false after a covered claim was edited post-lock")
	}
	found := false
	for _, w := range env.Warnings {
		if strings.Contains(w, "stale") && strings.Contains(w, "re-propose") {
			found = true
		}
	}
	if !found {
		t.Errorf("no warning names the staleness or its recovery: %v", env.Warnings)
	}

	out := boShow(t, cfgPath, "text")
	if !strings.Contains(out, "STALE") {
		t.Errorf("--format text does not say the order is stale:\n%s", strings.SplitN(out, "\n", 2)[0])
	}

	staleMermaid, mermaidErr, err := execCLI(t, "--config", cfgPath, "build-order", "show", "--module", "widget", "--format", "mermaid")
	if err != nil {
		t.Fatalf("show --format mermaid on a stale order must still succeed: %v (stderr: %s)", err, mermaidErr)
	}
	if !strings.Contains(mermaidErr, "stale") || !strings.Contains(mermaidErr, "re-propose") {
		t.Errorf("--format mermaid on a stale order says nothing about staleness on stderr, so the .mmd file it just wrote reads as the current approved order; stderr was: %q", mermaidErr)
	}
	if !strings.Contains(mermaidErr, "widget") {
		t.Errorf("the stderr note does not name the module, which an export loop over many modules needs: %q", mermaidErr)
	}
	if staleMermaid != freshMermaid {
		t.Errorf("--format mermaid stdout changed when the order went stale; the export's contract is that stdout is exactly the diagrams, so the note must be on stderr only.\nfresh:\n%s\nstale:\n%s", freshMermaid, staleMermaid)
	}
}

// TestCLI_BuildOrderShow_EmptyMermaidExportSaysSoOnStderr covers the one state
// in which this leaf legitimately writes zero bytes to stdout at exit 0: a
// locked order in which every claim is out-of-scope, so no phase has anything
// to draw.
//
// That is the shape the not_proposed refusal exists to avoid producing — an
// empty .mmd file with nothing said — and it cannot be fixed on stdout, because
// the export's contract is that it carries exactly the diagrams and a banner
// would be a chunk with no diagram keyword in it. So the fact goes to stderr,
// where a redirect does not capture it and the person at the terminal still
// reads it.
func TestCLI_BuildOrderShow_EmptyMermaidExportSaysSoOnStderr(t *testing.T) {
	root := t.TempDir()
	cfgPath := boWriteConfig(t, root, "widget")
	boWriteClaim(t, root, "later.yaml", "widget.contract.later", "widget", "locked", "out-of-scope", nil)

	if _, stderr, err := execCLI(t, "--config", cfgPath, "build-order", "propose", "--module", "widget"); err != nil {
		t.Fatalf("propose: %v (stderr: %s)", err, stderr)
	}
	if _, stderr, err := execCLI(t, "--config", cfgPath, "build-order", "lock", "--module", "widget", "--reason", "r"); err != nil {
		t.Fatalf("lock: %v (stderr: %s)", err, stderr)
	}

	stdout, stderr, err := execCLI(t, "--config", cfgPath, "build-order", "show", "--module", "widget", "--format", "mermaid")
	if err != nil {
		t.Fatalf("show --format mermaid on an all-excluded order must succeed: %v", err)
	}
	if stdout != "" {
		t.Errorf("stdout is not empty; the export carries exactly the diagrams and there are none:\n%s", stdout)
	}
	if !strings.Contains(stderr, "no diagram to draw") || !strings.Contains(stderr, "widget") {
		t.Errorf("stderr does not say why the export is empty: %q", stderr)
	}

	// The other two renderings do carry the answer, which is what stderr points
	// the reader at.
	text := boShow(t, cfgPath, "text")
	if !strings.Contains(text, "excluded      1 claim: widget.contract.later") {
		t.Errorf("--format text does not name the excluded claim:\n%s", text)
	}
}
