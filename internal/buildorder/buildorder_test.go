package buildorder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// mc is a small constructor for a locked, in-memory test claim — every
// buildorder test fixture claim needs Status/Module/BuildRole/RestsOn at
// minimum, so this keeps each test's fixture readable.
func mc(id, module string, role model.BuildRole, restsOn ...string) model.Claim {
	return model.Claim{
		ID:         id,
		Module:     module,
		Facet:      "contract",
		Status:     model.StatusLocked,
		BuildRole:  role,
		RestsOn:    restsOn,
		SourcePath: id + ".yaml",
	}
}

// ---------------------------------------------------------------------
// Completeness gate
// ---------------------------------------------------------------------

func TestPropose_CompletenessGate_ListsEveryNonLockedClaim(t *testing.T) {
	claims := []model.Claim{
		mc("m.contract.a", "m", model.BuildRoleSchema),
		{ID: "m.contract.b", Module: "m", Status: model.StatusDraft, BuildRole: model.BuildRoleBehavior},
		{ID: "m.contract.c", Module: "m", Status: model.StatusDraft, BuildRole: model.BuildRoleAPI},
	}

	_, err := Propose(claims, nil, "m")
	if err == nil {
		t.Fatalf("expected an error for a not-fully-locked module")
	}
	if !strings.Contains(err.Error(), "m.contract.b") || !strings.Contains(err.Error(), "m.contract.c") {
		t.Fatalf("expected both non-locked claim ids named in the error, got: %v", err)
	}
	if strings.Contains(err.Error(), "m.contract.a") {
		t.Fatalf("did not expect the already-locked claim named as a problem, got: %v", err)
	}
}

func TestPropose_UnknownModule_Errors(t *testing.T) {
	claims := []model.Claim{mc("m.contract.a", "m", model.BuildRoleSchema)}
	if _, err := Propose(claims, nil, "does-not-exist"); err == nil {
		t.Fatalf("expected an error proposing a build order for a module with no claims")
	}
}

// ---------------------------------------------------------------------
// BuildRole validation
// ---------------------------------------------------------------------

func TestPropose_MissingBuildRole_Errors(t *testing.T) {
	claims := []model.Claim{
		{ID: "m.contract.a", Module: "m", Status: model.StatusLocked, BuildRole: ""},
	}
	_, err := Propose(claims, nil, "m")
	if err == nil || !strings.Contains(err.Error(), "m.contract.a") {
		t.Fatalf("expected an error naming the claim missing build_role, got: %v", err)
	}
}

func TestPropose_InvalidBuildRole_Errors(t *testing.T) {
	claims := []model.Claim{
		mc("m.contract.a", "m", model.BuildRole("not-a-real-phase")),
	}
	_, err := Propose(claims, nil, "m")
	if err == nil || !strings.Contains(err.Error(), "invalid build_role") {
		t.Fatalf("expected an invalid-build_role error, got: %v", err)
	}
}

// ---------------------------------------------------------------------
// Excluded (out-of-scope) claims
// ---------------------------------------------------------------------

func TestPropose_OutOfScopeClaims_ExcludedNotPlaced(t *testing.T) {
	claims := []model.Claim{
		mc("m.contract.schema", "m", model.BuildRoleSchema),
		mc("m.contract.future", "m", model.BuildRoleOutOfScope),
	}
	a, err := Propose(claims, nil, "m")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if len(a.Excluded) != 1 || a.Excluded[0] != "m.contract.future" {
		t.Fatalf("expected excluded == [m.contract.future], got %v", a.Excluded)
	}
	for _, p := range a.Phases {
		for _, c := range p.Claims {
			if c.ID == "m.contract.future" {
				t.Fatalf("out-of-scope claim must never be placed in a phase, found in %q", p.Phase)
			}
		}
	}
}

// ---------------------------------------------------------------------
// Phase-order violation
// ---------------------------------------------------------------------

func TestPropose_PhaseOrderViolation_SameModule_Errors(t *testing.T) {
	claims := []model.Claim{
		// schema claim resting on a behavior claim: behavior is a LATER
		// phase than schema, a modeling error.
		mc("m.contract.schema", "m", model.BuildRoleSchema, "m.contract.behavior"),
		mc("m.contract.behavior", "m", model.BuildRoleBehavior),
	}
	_, err := Propose(claims, nil, "m")
	if err == nil {
		t.Fatalf("expected a phase-order violation error")
	}
	if !strings.Contains(err.Error(), "m.contract.schema") || !strings.Contains(err.Error(), "m.contract.behavior") {
		t.Fatalf("expected both claim ids named in the phase-order error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "schema") || !strings.Contains(err.Error(), "behavior") {
		t.Fatalf("expected both phase names in the phase-order error, got: %v", err)
	}
}

func TestPropose_CrossModuleEdge_NeverViolatesPhaseOrder(t *testing.T) {
	claims := []model.Claim{
		// A schema claim in module m resting on a behavior claim in a
		// DIFFERENT module: must be informational only, never a violation.
		mc("m.contract.schema", "m", model.BuildRoleSchema, "other.contract.behavior"),
		mc("other.contract.behavior", "other", model.BuildRoleBehavior),
	}
	a, err := Propose(claims, nil, "m")
	if err != nil {
		t.Fatalf("cross-module rests_on must never fail propose, got: %v", err)
	}
	// The cross-module edge is still recorded (informational), not dropped.
	found := false
	for _, p := range a.Phases {
		for _, c := range p.Claims {
			if c.ID == "m.contract.schema" {
				found = true
				if len(c.RestsOn) != 1 || c.RestsOn[0] != "other.contract.behavior" {
					t.Fatalf("expected the cross-module edge recorded verbatim, got %v", c.RestsOn)
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected m.contract.schema placed in the schema phase")
	}
}

func TestPropose_SameModuleEdgeToOutOfScopeTarget_NeverViolates(t *testing.T) {
	claims := []model.Claim{
		mc("m.contract.schema", "m", model.BuildRoleSchema, "m.contract.future"),
		mc("m.contract.future", "m", model.BuildRoleOutOfScope),
	}
	if _, err := Propose(claims, nil, "m"); err != nil {
		t.Fatalf("resting on an out-of-scope same-module claim must not be a phase-order violation, got: %v", err)
	}
}

// ---------------------------------------------------------------------
// Layered topological sort — linear chain, diamond, disconnected
// components, tie-break on stable display order.
// ---------------------------------------------------------------------

func idsOf(entries []ClaimEntry) []string {
	var ids []string
	for _, e := range entries {
		ids = append(ids, e.ID)
	}
	return ids
}

func indexOf(ids []string, id string) int {
	for i, x := range ids {
		if x == id {
			return i
		}
	}
	return -1
}

func TestPropose_LayeredSort_LinearChain(t *testing.T) {
	// c rests_on b, b rests_on a: build order must be a, b, c.
	claims := []model.Claim{
		mc("m.contract.c", "m", model.BuildRoleBehavior, "m.contract.b"),
		mc("m.contract.b", "m", model.BuildRoleBehavior, "m.contract.a"),
		mc("m.contract.a", "m", model.BuildRoleBehavior),
	}
	a, err := Propose(claims, nil, "m")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	ids := idsOf(onlyPhase(a, model.BuildRoleBehavior))
	if indexOf(ids, "m.contract.a") >= indexOf(ids, "m.contract.b") ||
		indexOf(ids, "m.contract.b") >= indexOf(ids, "m.contract.c") {
		t.Fatalf("expected linear order a, b, c; got %v", ids)
	}
}

func TestPropose_LayeredSort_Diamond(t *testing.T) {
	// d rests_on b and c; b and c both rest_on a. a first, d last, b/c in
	// between (either relative order, both are valid layer 1 members).
	claims := []model.Claim{
		mc("m.contract.d", "m", model.BuildRoleBehavior, "m.contract.b", "m.contract.c"),
		mc("m.contract.b", "m", model.BuildRoleBehavior, "m.contract.a"),
		mc("m.contract.c", "m", model.BuildRoleBehavior, "m.contract.a"),
		mc("m.contract.a", "m", model.BuildRoleBehavior),
	}
	a, err := Propose(claims, nil, "m")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	ids := idsOf(onlyPhase(a, model.BuildRoleBehavior))
	if ids[0] != "m.contract.a" {
		t.Fatalf("expected a placed first (layer 0), got order %v", ids)
	}
	if ids[len(ids)-1] != "m.contract.d" {
		t.Fatalf("expected d placed last (depends on both b and c), got order %v", ids)
	}
	if indexOf(ids, "m.contract.b") >= indexOf(ids, "m.contract.d") ||
		indexOf(ids, "m.contract.c") >= indexOf(ids, "m.contract.d") {
		t.Fatalf("expected both b and c placed before d, got %v", ids)
	}
}

func TestPropose_LayeredSort_DisconnectedComponents(t *testing.T) {
	// Two independent chains in the same phase: x2->x1, y2->y1. Each
	// chain's internal order must hold; the two chains may interleave
	// freely (Kahn's algorithm places by readiness, not by chain identity).
	claims := []model.Claim{
		mc("m.contract.x2", "m", model.BuildRoleBehavior, "m.contract.x1"),
		mc("m.contract.x1", "m", model.BuildRoleBehavior),
		mc("m.contract.y2", "m", model.BuildRoleBehavior, "m.contract.y1"),
		mc("m.contract.y1", "m", model.BuildRoleBehavior),
	}
	a, err := Propose(claims, nil, "m")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	ids := idsOf(onlyPhase(a, model.BuildRoleBehavior))
	if indexOf(ids, "m.contract.x1") >= indexOf(ids, "m.contract.x2") {
		t.Fatalf("expected x1 before x2, got %v", ids)
	}
	if indexOf(ids, "m.contract.y1") >= indexOf(ids, "m.contract.y2") {
		t.Fatalf("expected y1 before y2, got %v", ids)
	}
}

func TestPropose_LayeredSort_TieBreakOnOrderField(t *testing.T) {
	// Three independent (no rests_on among them) behavior claims, all in
	// layer 0: Order should decide their relative placement.
	c1 := mc("m.contract.first", "m", model.BuildRoleBehavior)
	c1.Order = 2
	c2 := mc("m.contract.second", "m", model.BuildRoleBehavior)
	c2.Order = 1
	c3 := mc("m.contract.third", "m", model.BuildRoleBehavior) // unordered

	a, err := Propose([]model.Claim{c1, c2, c3}, nil, "m")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	ids := idsOf(onlyPhase(a, model.BuildRoleBehavior))
	if indexOf(ids, "m.contract.second") >= indexOf(ids, "m.contract.first") {
		t.Fatalf("expected Order:1 (second) before Order:2 (first), got %v", ids)
	}
	if indexOf(ids, "m.contract.first") >= indexOf(ids, "m.contract.third") {
		t.Fatalf("expected every Order-set claim ahead of the unordered one, got %v", ids)
	}
}

func TestPropose_SamePhaseCycle_ErrorsRatherThanDroppingClaims(t *testing.T) {
	// p rests_on q, q rests_on p: a same-module, same-phase 2-cycle. This
	// must never be lint's problem alone — Propose has to fail loudly
	// rather than silently write an artifact missing both claims.
	claims := []model.Claim{
		mc("m.internals.p", "m", model.BuildRoleBehavior, "m.internals.q"),
		mc("m.internals.q", "m", model.BuildRoleBehavior, "m.internals.p"),
	}
	a, err := Propose(claims, nil, "m")
	if err == nil {
		t.Fatalf("expected a cycle error, got artifact: %+v", a)
	}
	if !strings.Contains(err.Error(), "m.internals.p") || !strings.Contains(err.Error(), "m.internals.q") {
		t.Fatalf("expected both cyclic claim ids named in the error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("expected the error to mention a cycle, got: %v", err)
	}
}

// ---------------------------------------------------------------------
// Phase sequencing across the full 5-phase list, plus ClaimEntry.File.
// ---------------------------------------------------------------------

func TestPropose_FullPhaseSequence_OrderedCorrectly(t *testing.T) {
	claims := []model.Claim{
		mc("m.contract.verify", "m", model.BuildRoleVerification),
		mc("m.contract.api", "m", model.BuildRoleAPI),
		mc("m.contract.behavior", "m", model.BuildRoleBehavior),
		mc("m.contract.schema", "m", model.BuildRoleSchema),
		mc("m.contract.orient", "m", model.BuildRoleOrientation),
	}
	a, err := Propose(claims, nil, "m")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if len(a.Phases) != 5 {
		t.Fatalf("expected all 5 phases present, got %d: %+v", len(a.Phases), a.Phases)
	}
	wantOrder := []string{"orientation", "schema", "behavior", "api", "verification"}
	for i, p := range a.Phases {
		if p.Phase != wantOrder[i] {
			t.Fatalf("phase[%d] = %q, want %q", i, p.Phase, wantOrder[i])
		}
	}
	if a.Phases[2].Claims[0].File != "m.contract.behavior.yaml" {
		t.Fatalf("expected ClaimEntry.File to carry SourcePath, got %q", a.Phases[2].Claims[0].File)
	}
}

func TestPropose_EmptyPhaseOmitted(t *testing.T) {
	// No orientation/verification claims at all: those phases should not
	// appear as empty entries in the artifact.
	claims := []model.Claim{
		mc("m.contract.schema", "m", model.BuildRoleSchema),
	}
	a, err := Propose(claims, nil, "m")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	if len(a.Phases) != 1 || a.Phases[0].Phase != "schema" {
		t.Fatalf("expected exactly one (schema) phase present, got %+v", a.Phases)
	}
}

// TestPropose_ClaimEntryFile_IsProjectRelativeNotAbsolute is a regression
// test: an earlier version stored model.Claim.SourcePath (always absolute
// — see internal/config.LoadConfig's ClaimsDir resolution) verbatim into
// ClaimEntry.File, which leaked the reviewing machine's absolute
// directory structure (home directory, username, etc.) into a
// shareable, published viewer artifact. Propose must instead render File
// relative to the project's own directory (cfg.Dir()).
func TestPropose_ClaimEntryFile_IsProjectRelativeNotAbsolute(t *testing.T) {
	dir := t.TempDir()
	claimsDir := filepath.Join(dir, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	cfgPath := filepath.Join(dir, "project.config.yaml")
	cfgYAML := "schema_version: 1\n" +
		"facets: [contract]\n" +
		"modules: [widget]\n" +
		"claims_dir: claims\n"
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	claim := mc("widget.contract.schema", "widget", model.BuildRoleSchema)
	claim.SourcePath = filepath.Join(claimsDir, "schema.yaml") // absolute, like the real loader produces

	a, err := Propose([]model.Claim{claim}, cfg, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	entries := onlyPhase(a, model.BuildRoleSchema)
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 schema claim, got %+v", entries)
	}
	if filepath.IsAbs(entries[0].File) {
		t.Fatalf("expected ClaimEntry.File to be project-relative, got absolute path %q", entries[0].File)
	}
	if want := filepath.Join("claims", "schema.yaml"); entries[0].File != want {
		t.Fatalf("ClaimEntry.File = %q, want %q", entries[0].File, want)
	}
}

// onlyPhase returns the ClaimEntry list for the named phase, or nil.
func onlyPhase(a *Artifact, role model.BuildRole) []ClaimEntry {
	for _, p := range a.Phases {
		if p.Phase == string(role) {
			return p.Claims
		}
	}
	return nil
}
