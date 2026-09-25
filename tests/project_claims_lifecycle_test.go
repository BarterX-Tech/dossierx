// project_claims_lifecycle_test.go drives a project claim (NIT-25: a
// project-claims/<slug>.yaml node with id project.<slug>, scope: project and
// no module or facet) through the whole pipeline, via the built binary, from
// the static fixture testdata/fixture-coverage/lifecycle/project-claims:
//
//   - check --validate on the fixture is lint-clean, with a module claim
//     resting on a project claim and a project claim resting on a module
//     *.contract.* claim;
//   - locking the module claim while its project-claim prerequisite is still
//     draft is locally admissible under policy v1 and leaves the dependent
//     unready, with the direct path [module, project] as the witness, and the
//     draft project claim resting on THAT module claim sees the same obstacle
//     through the unchanged intermediate;
//   - the written catalog carries the project claim as an entry with no
//     module and no facet, its rests_on edges intact, absent from by_module
//     and by_facet, and the viewer lists it under the constitution's
//     "Project claims" tab;
//   - locking the project claim clears the obstacle for every dependent;
//   - unlocking, rewriting and re-locking the project claim flips review on
//     its direct dependent (direct_dependency_change) and, through that
//     unchanged intermediate, on the project claim resting on it
//     (upstream_dependency_review).
//
// The fixture is committed all-draft with a locked roof, built by the real
// commands (`constitution lock`, `claim new`); every lock, check and rewrite
// below runs on a temp copy, because those commands mutate claim files and
// the ledger, which must never happen against a checked-in testdata directory.
//
// `check --staged` — the pre-commit hook's entry point — IS covered here too,
// by TestLifecycle_ProjectClaimsFixturePassesTheStagedGate: the fixture is
// copied into a disposable git repository (never the checked-in testdata
// directory), staged, and the gate must be as clean as plain check is, both
// as committed and once every claim is locked through the real approval
// path; a hand-edited locked project claim must then be refused. This used
// to be an exemption: --staged enumerated claims from claims_dir's pathspec
// only and never read project-claims/ out of the index, so a module claim
// resting on project.<slug> reported `dangling` and every locked project
// claim `lock-ledger-abandoned` from the hook, on a tree plain check
// accepted. The staged path now reads the project-claims store from the
// index exactly as it reads claims_dir (internal/check/staged.go,
// stagedProjectClaims), and the test below is what keeps it that way.
package tests

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// projectClaimsFixture copies the static fixture into a temp directory and
// returns the copy's root and config path. The committed fixture is asserted
// to be what this test believes it is — a project-claims store with two
// claims and a locked roof — so a fixture edit cannot quietly turn the run
// into a test of something else.
func projectClaimsFixture(t *testing.T) (root, cfgPath string) {
	t.Helper()
	src := filepath.Join(lifecycleFixturesRoot(t), "project-claims")
	for _, rel := range []string{
		"project.config.yaml",
		"constitution.yaml",
		filepath.Join("project-claims", "scope.yaml"),
		filepath.Join("project-claims", "retention.yaml"),
		filepath.Join("claims", "widget.contract.overview.yaml"),
		filepath.Join("claims", "widget.internals.fields.yaml"),
		filepath.Join("build", "ledger", "lock-store.json"),
	} {
		if _, err := os.Stat(filepath.Join(src, rel)); err != nil {
			t.Fatalf("the project-claims fixture is missing %s: %v", rel, err)
		}
	}
	root = filepath.Join(t.TempDir(), "project-claims")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	copyTree(t, src, root)
	return root, filepath.Join(root, "project.config.yaml")
}

// catalogEntry is the subset of a build/catalog/catalog.json entry this test
// reads. Readiness is decoded loosely on purpose: the assertions below name
// the kinds and paths they expect rather than pinning the whole assessment.
type catalogEntry struct {
	ID     string `json:"id"`
	Facet  string `json:"facet"`
	Module string `json:"module"`
	Status string `json:"status"`
	Edges  struct {
		RestsOn       []string `json:"rests_on"`
		RestsOnNone   bool     `json:"rests_on_none"`
		RestsOnReason string   `json:"rests_on_reason"`
	} `json:"edges"`
	Readiness *struct {
		LocalApproved        bool              `json:"local_approved"`
		DependencyReady      bool              `json:"dependency_ready"`
		Ready                bool              `json:"ready"`
		ReviewPending        bool              `json:"review_pending"`
		DependencyConditions []readinessRecord `json:"dependency_conditions"`
		ReviewCauses         []readinessRecord `json:"review_causes"`
	} `json:"readiness"`
}

type readinessRecord struct {
	Kind string   `json:"kind"`
	Path []string `json:"path"`
}

type catalogDocument struct {
	Claims   []catalogEntry      `json:"claims"`
	ByFacet  map[string][]string `json:"by_facet"`
	ByModule map[string][]string `json:"by_module"`
}

func readCatalog(t *testing.T, root string) (catalogDocument, map[string]catalogEntry) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "build", "catalog", "catalog.json"))
	if err != nil {
		t.Fatalf("read the written catalog: %v", err)
	}
	var doc catalogDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("decode catalog.json: %v", err)
	}
	byID := make(map[string]catalogEntry, len(doc.Claims))
	for _, e := range doc.Claims {
		byID[e.ID] = e
	}
	return doc, byID
}

func hasRecord(records []readinessRecord, kind string, path ...string) bool {
	for _, r := range records {
		if r.Kind == kind && reflect.DeepEqual(r.Path, path) {
			return true
		}
	}
	return false
}

// mustCheck runs a writing `dossierx check` on the copy and returns the
// re-read catalog. A non-zero exit is fatal: every step below is reached
// from a state the previous step proved clean.
func mustCheck(t *testing.T, root, cfgPath, step string) (catalogDocument, map[string]catalogEntry) {
	t.Helper()
	stdout, stderr, code := run(t, root, "--config", cfgPath, "check")
	if code != 0 {
		t.Fatalf("%s: check exited %d\nstdout: %s\nstderr: %s", step, code, stdout, stderr)
	}
	return readCatalog(t, root)
}

func mustLock(t *testing.T, root, cfgPath, id string) {
	t.Helper()
	stdout, stderr, code := reviewedRun(t, root, "--config", cfgPath, "claim", "lock", id, "--reason", "fixture approval")
	if code != 0 {
		t.Fatalf("lock %s: exit %d\nstdout: %s\nstderr: %s", id, code, stdout, stderr)
	}
}

func TestLifecycle_ProjectClaimsThroughThePipeline(t *testing.T) {
	root, cfgPath := projectClaimsFixture(t)
	const (
		scope     = "project.scope"
		retention = "project.retention"
		overview  = "widget.contract.overview"
		fields    = "widget.internals.fields"
	)

	// 1. The fixture is lint-clean as committed: rests-on-required and
	// rests-on-target accept project.<slug> targets, and id-shape accepts the
	// project id grammar.
	findings, code := runValidateFindings(t, root, cfgPath)
	if code != 0 || len(findings) != 0 {
		t.Fatalf("check --validate on the project-claims fixture: exit %d, findings %+v", code, findings)
	}

	// 2. A draft project claim beneath a module claim: the lock is locally
	// admissible under policy v1 and the preview names the obstacle.
	preview, stderr, code := run(t, root, "--config", cfgPath, "--format", "json", "claim", "lock", overview, "--reason", "fixture approval", "--dry-run")
	if code != 0 {
		t.Fatalf("lock preview: exit %d\n%s\n%s", code, preview, stderr)
	}
	var previewEnv struct {
		Data struct {
			Blocked    bool `json:"blocked"`
			Evaluation struct {
				Verdicts []struct {
					ClaimID              string `json:"claim_id"`
					LocalAdmissible      bool   `json:"local_admissible"`
					DependencyConditions []struct {
						DependencyID string   `json:"dependency_id"`
						Kind         string   `json:"kind"`
						Path         []string `json:"path"`
					} `json:"dependency_conditions"`
				} `json:"verdicts"`
			} `json:"evaluation"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(preview), &previewEnv); err != nil {
		t.Fatalf("decode lock preview: %v\n%s", err, preview)
	}
	if previewEnv.Data.Blocked || len(previewEnv.Data.Evaluation.Verdicts) != 1 || !previewEnv.Data.Evaluation.Verdicts[0].LocalAdmissible {
		t.Fatalf("locking a module claim against a readable draft project claim must be locally admissible under v1: %s", preview)
	}
	seen := false
	for _, c := range previewEnv.Data.Evaluation.Verdicts[0].DependencyConditions {
		if c.DependencyID == scope && c.Kind == "dependency_unapproved" && reflect.DeepEqual(c.Path, []string{overview, scope}) {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("the lock preview must name the draft project claim as an unapproved dependency with path [%s %s]: %s", overview, scope, preview)
	}
	mustLock(t, root, cfgPath, overview)

	// 3. The written catalog and viewer. The project claim is an entry with
	// no module and no facet, its edges intact, in no module or facet bucket;
	// the module claim is locally approved but not ready; the draft project
	// claim resting on it sees the same obstacle through the unchanged
	// intermediate.
	doc, byID := mustCheck(t, root, cfgPath, "after locking the module claim")
	for _, id := range []string{scope, retention, overview, fields} {
		if _, ok := byID[id]; !ok {
			t.Fatalf("catalog.json lacks %s; entries: %+v", id, doc.Claims)
		}
	}
	for _, id := range []string{scope, retention} {
		e := byID[id]
		if e.Module != "" || e.Facet != "" {
			t.Fatalf("project claim %s must carry no module or facet in the catalog: %+v", id, e)
		}
		for bucket, ids := range doc.ByModule {
			for _, member := range ids {
				if member == id {
					t.Fatalf("project claim %s must not appear under by_module[%q]", id, bucket)
				}
			}
		}
		for bucket, ids := range doc.ByFacet {
			for _, member := range ids {
				if member == id {
					t.Fatalf("project claim %s must not appear under by_facet[%q]", id, bucket)
				}
			}
		}
	}
	if !reflect.DeepEqual(doc.ByModule["widget"], []string{overview, fields}) {
		t.Fatalf("by_module[widget] = %v, want exactly the two module claims", doc.ByModule["widget"])
	}
	if e := byID[scope]; !e.Edges.RestsOnNone || strings.TrimSpace(e.Edges.RestsOnReason) == "" || len(e.Edges.RestsOn) != 0 || e.Status != "draft" {
		t.Fatalf("%s must be a draft leaf with rests_on: {none: true, reason}: %+v", scope, e)
	}
	if e := byID[retention]; !reflect.DeepEqual(e.Edges.RestsOn, []string{overview}) || e.Edges.RestsOnNone {
		t.Fatalf("%s must rest on exactly [%s] in the catalog: %+v", retention, overview, e)
	}
	if e := byID[overview]; !reflect.DeepEqual(e.Edges.RestsOn, []string{scope}) {
		t.Fatalf("%s must rest on exactly [%s] in the catalog: %+v", overview, scope, e)
	}
	ov := byID[overview]
	if ov.Status != "locked" || ov.Readiness == nil || !ov.Readiness.LocalApproved || ov.Readiness.DependencyReady || ov.Readiness.Ready {
		t.Fatalf("%s must be locked and locally approved but not ready while %s is draft: %+v", overview, scope, ov)
	}
	if !hasRecord(ov.Readiness.DependencyConditions, "dependency_unapproved", overview, scope) {
		t.Fatalf("%s must carry the direct unapproved-dependency witness [%s %s]: %+v", overview, overview, scope, ov.Readiness.DependencyConditions)
	}
	rt := byID[retention]
	if rt.Readiness == nil || rt.Readiness.DependencyReady || !hasRecord(rt.Readiness.DependencyConditions, "dependency_unapproved", retention, overview, scope) {
		t.Fatalf("%s must see the draft project claim through the unchanged intermediate, path [%s %s %s]: %+v", retention, retention, overview, scope, rt)
	}
	if sc := byID[scope]; sc.Readiness == nil || sc.Readiness.LocalApproved || !sc.Readiness.DependencyReady || sc.Readiness.Ready {
		t.Fatalf("a draft project claim resting on nothing is dependency-ready and not locally approved: %+v", sc)
	}
	viewer, err := os.ReadFile(filepath.Join(root, "build", "viewer", "index.html"))
	if err != nil {
		t.Fatalf("read the written viewer: %v", err)
	}
	for _, want := range []string{
		`<article class="project-claim"><h4>` + scope + `</h4>`,
		`<article class="project-claim"><h4>` + retention + `</h4>`,
		`<p class="constitution-meter">11 of 800 words · locked</p>`,
	} {
		if !strings.Contains(string(viewer), want) {
			t.Fatalf("the viewer must list the project claims under the constitution; missing %q", want)
		}
	}

	// 4. Locking the project claim clears the obstacle for both dependents,
	// and the project claim resting on the module claim then locks and is
	// ready itself.
	mustLock(t, root, cfgPath, scope)
	mustLock(t, root, cfgPath, retention)
	_, byID = mustCheck(t, root, cfgPath, "after locking both project claims")
	for _, id := range []string{scope, overview, retention} {
		e := byID[id]
		if e.Status != "locked" || e.Readiness == nil || !e.Readiness.Ready || e.Readiness.ReviewPending || len(e.Readiness.DependencyConditions) != 0 || len(e.Readiness.ReviewCauses) != 0 {
			t.Fatalf("%s must be locked and ready with no conditions or causes once the project claim is approved: %+v", id, e)
		}
	}
	if e := byID[fields]; e.Status != "draft" || e.Readiness == nil || !e.Readiness.DependencyReady || e.Readiness.Ready {
		t.Fatalf("%s stays a dependency-ready draft: %+v", fields, e)
	}

	// 5. A rewrite of the project claim, through the honest path (unlock,
	// edit, lock), propagates: the module claim resting on it directly is
	// review-pending on disk and in live readiness, and the project claim
	// resting on THAT module claim is review-pending in live readiness
	// through the unchanged intermediate.
	if stdout, stderr, code := run(t, root, "--config", cfgPath, "claim", "unlock", scope, "--reason", "rewrite the scope"); code != 0 {
		t.Fatalf("unlock %s: exit %d\n%s\n%s", scope, code, stdout, stderr)
	}
	scopePath := filepath.Join(root, "project-claims", "scope.yaml")
	raw, err := os.ReadFile(scopePath)
	if err != nil {
		t.Fatal(err)
	}
	rewritten := strings.Replace(string(raw), "under one roof", "under one shared roof", 1)
	if rewritten == string(raw) {
		t.Fatalf("the fixture body no longer carries the phrase this test rewrites:\n%s", raw)
	}
	if err := os.WriteFile(scopePath, []byte(rewritten), 0o644); err != nil {
		t.Fatal(err)
	}
	mustLock(t, root, cfgPath, scope)
	_, byID = mustCheck(t, root, cfgPath, "after rewriting and re-locking the project claim")
	if e := byID[scope]; !e.Readiness.Ready || e.Readiness.ReviewPending {
		t.Fatalf("the re-locked project claim is itself ready: %+v", e)
	}
	ov = byID[overview]
	if ov.Status != "locked" || !ov.Readiness.ReviewPending || ov.Readiness.Ready || !hasRecord(ov.Readiness.ReviewCauses, "direct_dependency_change", overview, scope) {
		t.Fatalf("%s must stay locked and turn review-pending with the direct_dependency_change witness [%s %s]: %+v", overview, overview, scope, ov)
	}
	if !strings.Contains(llReadFile(t, filepath.Join(root, "claims", "widget.contract.overview.yaml")), "review_pending: true") {
		t.Fatalf("the direct dependent's file must carry review_pending: true after its project-claim prerequisite was rewritten")
	}
	rt = byID[retention]
	if rt.Status != "locked" || !rt.Readiness.ReviewPending || rt.Readiness.Ready || !hasRecord(rt.Readiness.ReviewCauses, "upstream_dependency_review", retention, overview, scope) {
		t.Fatalf("%s must inherit the review through the unchanged intermediate, witness [%s %s %s]: %+v", retention, retention, overview, scope, rt)
	}
}

// stagedGitInFixture runs one git command in the disposable fixture copy with
// an isolated configuration, fatally: git here is scaffolding for the gate
// under test, never the thing under test. A machine without git fails rather
// than skips — the staged gate is the subject of this test, and a skip would
// be indistinguishable from a pass.
func stagedGitInFixture(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("git is required to exercise check --staged and was not found on PATH: %v", err)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s in the fixture copy: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// stagedEnvelope is the subset of the `check --staged` envelope this test
// reads: the verdict, the two flags that say the index was judged rather
// than skipped, and every finding either enforcing caller branches on.
type stagedEnvelope struct {
	OK   bool `json:"ok"`
	Data struct {
		ReadOnly       bool          `json:"read_only"`
		Staged         bool          `json:"staged"`
		Skipped        bool          `json:"skipped"`
		LintFindings   []lintFinding `json:"lint_findings"`
		LedgerFindings []struct {
			Rule    string `json:"rule"`
			ClaimID string `json:"claim_id"`
		} `json:"ledger_findings"`
	} `json:"data"`
	Error *struct {
		Code string `json:"code"`
	} `json:"error"`
}

// runStaged runs "dossierx check --staged --format json" in root against
// cfgPath and decodes the envelope. A skipped run is fatal on the spot: the
// fixture copy is a git repository with the project staged, so "nothing to
// evaluate" would mean the gate looked at the wrong thing.
func runStaged(t *testing.T, root, cfgPath, step string) (stagedEnvelope, int) {
	t.Helper()
	stdout, stderr, code := run(t, root, "--config", cfgPath, "--format", "json", "check", "--staged")
	var env stagedEnvelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("%s: check --staged output is not a single envelope: %v\nstdout: %s\nstderr: %s", step, err, stdout, stderr)
	}
	if !env.Data.ReadOnly || !env.Data.Staged {
		t.Fatalf("%s: check --staged must mark its payload read_only and staged; got: %s", step, stdout)
	}
	if env.Data.Skipped {
		t.Fatalf("%s: check --staged skipped the fixture copy instead of judging its index: %s", step, stdout)
	}
	return env, code
}

// The staged gate on the project-claims fixture, in a disposable git
// repository: clean where plain check is clean, both as committed (all
// drafts under a locked roof) and once every claim — the two project claims
// included — is locked through the real approval path; and armed, so a
// hand-edited locked project claim in the index is refused.
func TestLifecycle_ProjectClaimsFixturePassesTheStagedGate(t *testing.T) {
	root, cfgPath := projectClaimsFixture(t)
	const (
		scope     = "project.scope"
		retention = "project.retention"
		overview  = "widget.contract.overview"
	)
	stagedGitInFixture(t, root, "init", "-q")
	stagedGitInFixture(t, root, "add", "-A")

	// 1. As committed: plain check is clean, and so is the index.
	if findings, code := runValidateFindings(t, root, cfgPath); code != 0 || len(findings) != 0 {
		t.Fatalf("control precondition: check --validate on the fixture copy: exit %d, findings %+v", code, findings)
	}
	env, code := runStaged(t, root, cfgPath, "as committed")
	if code != 0 || !env.OK || len(env.Data.LintFindings) != 0 || len(env.Data.LedgerFindings) != 0 {
		t.Fatalf("check --staged must be as clean as check --validate on the fixture as committed: exit %d, %+v", code, env)
	}

	// 2. Every claim locked, the approvals staged with them: still clean.
	// The module claim locks first, against a draft project claim (locally
	// admissible under policy v1); then both project claims.
	for _, id := range []string{overview, scope, retention} {
		mustLock(t, root, cfgPath, id)
	}
	stagedGitInFixture(t, root, "add", "-A")
	if findings, code := runValidateFindings(t, root, cfgPath); code != 0 || len(findings) != 0 {
		t.Fatalf("control precondition: check --validate with every claim locked: exit %d, findings %+v", code, findings)
	}
	env, code = runStaged(t, root, cfgPath, "with every claim locked")
	if code != 0 || !env.OK || len(env.Data.LintFindings) != 0 || len(env.Data.LedgerFindings) != 0 {
		t.Fatalf("check --staged must accept the locked project claims and the module claim resting on one: exit %d, %+v", code, env)
	}

	// 3. And the second store is judged: a locked project claim rewritten in
	// the file and staged — no unlock, no new record — is refused by name.
	scopePath := filepath.Join(root, "project-claims", "scope.yaml")
	raw, err := os.ReadFile(scopePath)
	if err != nil {
		t.Fatal(err)
	}
	tampered := strings.Replace(string(raw), "under one roof", "under one shared roof", 1)
	if tampered == string(raw) {
		t.Fatalf("the fixture body no longer carries the phrase this test rewrites:\n%s", raw)
	}
	if err := os.WriteFile(scopePath, []byte(tampered), 0o644); err != nil {
		t.Fatal(err)
	}
	stagedGitInFixture(t, root, "add", "-A")
	env, code = runStaged(t, root, cfgPath, "with a hand-edited locked project claim staged")
	if code == 0 || env.OK || env.Error == nil || env.Error.Code != "integrity_failed" {
		t.Fatalf("a hand-edited locked project claim in the index must be refused as integrity_failed: exit %d, %+v", code, env)
	}
	refused := false
	for _, f := range env.Data.LedgerFindings {
		if f.Rule == "lock-content-drift" && f.ClaimID == scope {
			refused = true
		}
	}
	if !refused {
		t.Fatalf("the refusal must carry lock-content-drift on %s, got %+v", scope, env.Data.LedgerFindings)
	}
}
