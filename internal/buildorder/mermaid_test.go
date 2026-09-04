package buildorder

import (
	"reflect"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// viewFixture is a locked module with claims in three phases, one
// earlier-phase edge (behavior rests on schema), one same-phase edge, one
// cross-module edge and one edge to an excluded claim; the other module's
// claim is present in the catalog so the cross-module target is attributable.
func viewFixture(t *testing.T) (*Artifact, []model.Claim) {
	t.Helper()
	claims := []model.Claim{
		mc("w.contract.schema", "w", model.BuildRoleSchema),
		mc("w.contract.behavior", "w", model.BuildRoleBehavior, "w.contract.schema", "g.contract.other"),
		mc("w.internals.report", "w", model.BuildRoleBehavior, "w.contract.behavior", "w.contract.later"),
		mc("w.contract.later", "w", model.BuildRoleOutOfScope),
		mc("g.contract.other", "g", model.BuildRoleSchema),
	}
	a, err := Propose(claims, nil, "w")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	a.Locked = true
	return a, claims
}

func viewByName(t *testing.T, views []PhaseView, name string) PhaseView {
	t.Helper()
	for _, v := range views {
		if v.Name == name {
			return v
		}
	}
	t.Fatalf("no view named %q in %d views", name, len(views))
	return PhaseView{}
}

func TestViews_SixBlocksByNameWithZeroCountsForAbsentPhases(t *testing.T) {
	a, claims := viewFixture(t)
	views, nodeIDs, err := Views(a, claims)
	if err != nil {
		t.Fatalf("Views: %v", err)
	}
	if len(views) != 6 {
		t.Fatalf("got %d views, want 6", len(views))
	}
	wantNames := []string{"orientation", "schema", "behavior", "api", "verification", ExcludedPhaseName}
	for i, v := range views {
		if v.Name != wantNames[i] {
			t.Errorf("views[%d].Name = %q, want %q", i, v.Name, wantNames[i])
		}
		if v.Definition != model.BuildRoleDefinition(model.BuildRole(v.Name)) && v.Name != ExcludedPhaseName {
			t.Errorf("views[%d] definition is not BuildRoleDefinition(%s)", i, v.Name)
		}
		if v.Levels == nil || v.Ghosts == nil || v.CrossModule == nil || v.ExcludedDeps == nil || v.Claims == nil {
			t.Errorf("views[%d] has a nil slice/map; the payload must never carry null", i)
		}
	}
	// The artifact stores only schema and behavior (three claims) and one
	// excluded id; orientation, api and verification are absent from it.
	if len(a.Phases) != 2 {
		t.Fatalf("fixture precondition: artifact stores %d phases, want 2", len(a.Phases))
	}
	for _, name := range []string{"orientation", "api", "verification"} {
		if v := viewByName(t, views, name); v.Count() != 0 || len(v.Levels) != 0 {
			t.Errorf("%s: count %d, levels %d, want 0/0 for a phase the artifact lacks", name, v.Count(), len(v.Levels))
		}
	}
	if v := viewByName(t, views, "schema"); v.Number != 2 || v.Count() != 1 || v.Locked != 1 {
		t.Errorf("schema: number %d count %d locked %d", v.Number, v.Count(), v.Locked)
	}
	if v := viewByName(t, views, ExcludedPhaseName); v.Number != 0 || v.Count() != 1 || v.Claims[0].ID != "w.contract.later" || v.Definition != model.BuildRoleDefinition(model.BuildRoleOutOfScope) {
		t.Errorf("excluded block wrong: %+v", v)
	}
	// node_ids covers every drawn node, ghosts included, and inverts NodeID.
	for _, id := range []string{"w.contract.schema", "w.contract.behavior", "w.internals.report"} {
		if got := nodeIDs[NodeID(id)]; got != id {
			t.Errorf("nodeIDs[%q] = %q, want %q", NodeID(id), got, id)
		}
	}
	if _, ok := nodeIDs[NodeID("g.contract.other")]; ok {
		t.Error("a cross-module target must not be a drawn node")
	}
}

func TestViews_LevelsFlattenToTheStoredOrderAndMatchTheSort(t *testing.T) {
	a, claims := viewFixture(t)
	views, _, err := Views(a, claims)
	if err != nil {
		t.Fatalf("Views: %v", err)
	}
	for _, p := range a.Phases {
		v := viewByName(t, views, p.Phase)
		var flat []string
		for _, level := range v.Levels {
			flat = append(flat, level...)
		}
		var stored []string
		for _, c := range p.Claims {
			stored = append(stored, c.ID)
		}
		if !reflect.DeepEqual(flat, stored) {
			t.Errorf("%s: flattened levels %v != stored order %v", p.Phase, flat, stored)
		}
		// Boundaries equal layeredTopoSort's over the same entries.
		in := make([]model.Claim, 0, len(p.Claims))
		for _, c := range p.Claims {
			in = append(in, model.Claim{ID: c.ID, RestsOn: c.RestsOn})
		}
		layers, cyclic := layeredTopoSort(in)
		if len(cyclic) > 0 {
			t.Fatalf("%s: unexpected cycle %v", p.Phase, cyclic)
		}
		if len(layers) != len(v.Levels) {
			t.Fatalf("%s: %d layers from the sort, %d levels in the view", p.Phase, len(layers), len(v.Levels))
		}
		for i := range layers {
			var ids []string
			for _, c := range layers[i] {
				ids = append(ids, c.ID)
			}
			if !reflect.DeepEqual(ids, v.Levels[i]) {
				t.Errorf("%s level %d: view %v, sort %v", p.Phase, i, v.Levels[i], ids)
			}
		}
	}
	if v := viewByName(t, views, "behavior"); len(v.Levels) != 2 {
		t.Errorf("behavior has %d levels, want 2 (report rests on behavior)", len(v.Levels))
	}
}

func TestViews_GhostsCrossModuleAndExcludedDeps(t *testing.T) {
	a, claims := viewFixture(t)
	views, _, err := Views(a, claims)
	if err != nil {
		t.Fatalf("Views: %v", err)
	}
	schema := viewByName(t, views, "schema")
	behavior := viewByName(t, views, "behavior")
	if len(schema.Ghosts) != 0 {
		t.Errorf("schema block must carry no ghost, got %v", schema.Ghosts)
	}
	if !reflect.DeepEqual(behavior.Ghosts, []Ghost{{ID: "w.contract.schema", Phase: "schema"}}) {
		t.Errorf("behavior ghosts = %v, want exactly the schema claim once", behavior.Ghosts)
	}
	if !reflect.DeepEqual(behavior.CrossModule, map[string][]string{"g": {"g.contract.other"}}) {
		t.Errorf("behavior cross-module = %v", behavior.CrossModule)
	}
	if !reflect.DeepEqual(behavior.ExcludedDeps, []string{"w.contract.later"}) {
		t.Errorf("behavior excluded deps = %v", behavior.ExcludedDeps)
	}
	if got := behavior.CrossModuleNames(); !reflect.DeepEqual(got, []string{"g"}) {
		t.Errorf("CrossModuleNames = %v", got)
	}
	text := Mermaid(behavior, MermaidOptions{Palette: PaletteLiteral})
	if strings.Contains(text, NodeID("g.contract.other")) {
		t.Errorf("a cross-module target must not be drawn:\n%s", text)
	}
	if strings.Contains(text, NodeID("w.contract.later")) {
		t.Errorf("an excluded target must not be drawn:\n%s", text)
	}
	if !strings.Contains(text, "  w_contract_schema -.-> w_contract_behavior\n") {
		t.Errorf("expected a dotted ghost edge:\n%s", text)
	}
	if !strings.Contains(text, "  w_contract_behavior --> w_internals_report\n") {
		t.Errorf("expected a solid same-phase edge:\n%s", text)
	}
}

func TestViews_StatusAndFacetComeFromTheCatalog(t *testing.T) {
	a, claims := viewFixture(t)
	// The behavior claim was unlocked after the lock; the report claim is gone
	// from the catalog altogether.
	claims[1].Status = model.StatusDraft
	claims = claims[:2]
	views, _, err := Views(a, claims)
	if err != nil {
		t.Fatalf("Views: %v", err)
	}
	b := viewByName(t, views, "behavior")
	if b.Locked != 0 {
		t.Errorf("behavior locked = %d, want 0 (one unlocked, one missing)", b.Locked)
	}
	if got := b.Class("w.contract.behavior"); got != "draft_con" {
		t.Errorf("class of the unlocked contract claim = %q, want draft_con", got)
	}
	if got := b.Class("w.internals.report"); got != "draft_int" {
		t.Errorf("class of the missing internals claim = %q, want draft_int (facet from the id)", got)
	}
}

func TestViews_RefusesALaterPhaseDependency(t *testing.T) {
	a, claims := viewFixture(t)
	// Hand-edit the locked bytes: the schema claim now rests on a behavior
	// claim, which computePhases would have refused.
	a.Phases[0].Claims[0].RestsOn = []string{"w.contract.behavior"}
	if _, _, err := Views(a, claims); err == nil || !strings.Contains(err.Error(), "later phase") {
		t.Errorf("expected a later-phase error, got %v", err)
	}
}

// TestViews_CollidingClaimIDsGetDistinctStableNodeIDs: two claim ids that
// sanitise to one node id ("w.internals.report" and "w.internals-report" —
// a project whose module or facet names differ only in "-" vs "_" is
// lint-clean and reaches this) are BOTH drawn, under distinct node ids,
// both indexed, the edge between them written with the allocated ids, and
// the allocation is the same on every call. Not an error that costs the
// module its tab or the project its viewer, and never one node standing
// for two claims.
func TestViews_CollidingClaimIDsGetDistinctStableNodeIDs(t *testing.T) {
	a, claims := viewFixture(t)
	// The colliding claim joins the behavior block and rests on the claim it
	// collides with, so the diagram has to write an edge between the two.
	a.Phases[1].Claims = append(a.Phases[1].Claims, ClaimEntry{ID: "w.internals-report", RestsOn: []string{"w.internals.report"}})
	claims = append(claims, mc("w.internals-report", "w", model.BuildRoleBehavior, "w.internals.report"))

	views, nodeIDs, err := Views(a, claims)
	if err != nil {
		t.Fatalf("Views must not refuse a node-id collision, got: %v", err)
	}
	first, second := NodeID("w.internals.report"), nodeIDs["w_internals_report"]
	if second != "w.internals.report" {
		t.Fatalf("the first claim to reach the sanitised id keeps it: nodeIDs[%q] = %q", first, second)
	}
	var suffixed string
	for n, id := range nodeIDs {
		if id == "w.internals-report" {
			suffixed = n
		}
	}
	if suffixed == "" {
		t.Fatalf("the colliding claim is not in the index at all: %v", nodeIDs)
	}
	if suffixed == first || !strings.HasPrefix(suffixed, first+"_") || len(suffixed) != len(first)+1+nodeIDSuffixLen {
		t.Errorf("colliding claim's node id = %q, want %q plus \"_\" and %d hex characters", suffixed, first, nodeIDSuffixLen)
	}
	for _, r := range suffixed {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			t.Errorf("node id %q leaves the [A-Za-z0-9_] alphabet", suffixed)
		}
	}
	// Injective: exactly as many node ids as distinct drawn claims (in-block
	// claims and ghosts over the five phases).
	drawn := map[string]bool{}
	for _, v := range views {
		if v.Number == 0 {
			continue
		}
		for _, c := range v.Claims {
			drawn[c.ID] = true
		}
		for _, g := range v.Ghosts {
			drawn[g.ID] = true
		}
	}
	if len(nodeIDs) != len(drawn) {
		t.Errorf("index holds %d node ids for %d drawn claims: %v", len(nodeIDs), len(drawn), nodeIDs)
	}

	text := Mermaid(viewByName(t, views, "behavior"), MermaidOptions{Palette: PaletteLiteral})
	for _, want := range []string{
		"  " + first + "[\"",
		"  " + suffixed + "[\"",
		"  " + first + " --> " + suffixed + "\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("behavior diagram lacks %q:\n%s", want, text)
		}
	}
	if strings.Count(text, "\n  "+first+"[") != 1 {
		t.Errorf("the first claim's node line must appear exactly once:\n%s", text)
	}

	// Stable: a second call allocates the same ids.
	_, again, err := Views(a, claims)
	if err != nil {
		t.Fatalf("Views again: %v", err)
	}
	if !reflect.DeepEqual(nodeIDs, again) {
		t.Errorf("node ids moved between two calls over the same inputs:\n%v\n%v", nodeIDs, again)
	}
}

// The two-claim golden: schema resting on nothing, behavior resting on it,
// pinned per palette. The CSS golden is the literal golden with the two
// locked_* classDef lines removed and the other three reduced to their
// stroke-dasharray.
func TestMermaid_GoldenPerPalette(t *testing.T) {
	claims := []model.Claim{
		mc("widget.contract.schema", "widget", model.BuildRoleSchema),
		mc("widget.contract.behavior", "widget", model.BuildRoleBehavior, "widget.contract.schema"),
	}
	a, err := Propose(claims, nil, "widget")
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	views, _, err := Views(a, claims)
	if err != nil {
		t.Fatalf("Views: %v", err)
	}
	behavior := viewByName(t, views, "behavior")

	wantLiteral := "%% phase 3 of 5: behavior\n" +
		"%% " + model.BuildRoleDefinition(model.BuildRoleBehavior) + "\n" +
		"%% 1 claim · 1 level · 1 locked\n" +
		"%% solid arrow: rests on, same phase. dotted arrow: rests on an earlier phase (ghost node).\n" +
		"flowchart TD\n" +
		"  widget_contract_behavior[\"Behavior\"]:::locked_con\n" +
		"  widget_contract_schema([\"Schema<br/><i>schema</i>\"]):::ghost\n" +
		"  widget_contract_schema -.-> widget_contract_behavior\n" +
		"  classDef locked_con fill:#dfeee6,stroke:#287052,color:#14231b\n" +
		"  classDef draft_con fill:#f4f6f3,stroke:#287052,stroke-dasharray:4 3,color:#14231b\n" +
		"  classDef locked_int fill:#e3eaf0,stroke:#205b78,color:#14231b\n" +
		"  classDef draft_int fill:#f2f5f7,stroke:#205b78,stroke-dasharray:4 3,color:#14231b\n" +
		"  classDef ghost fill:#ffffff,stroke:#b9c2bc,stroke-dasharray:2 3,color:#6b776f\n"
	if got := Mermaid(behavior, MermaidOptions{Palette: PaletteLiteral}); got != wantLiteral {
		t.Errorf("literal palette golden mismatch\n got: %q\nwant: %q", got, wantLiteral)
	}
	wantCSS := "%% phase 3 of 5: behavior\n" +
		"%% " + model.BuildRoleDefinition(model.BuildRoleBehavior) + "\n" +
		"%% 1 claim · 1 level · 1 locked\n" +
		"%% solid arrow: rests on, same phase. dotted arrow: rests on an earlier phase (ghost node).\n" +
		"flowchart TD\n" +
		"  widget_contract_behavior[\"Behavior\"]:::locked_con\n" +
		"  widget_contract_schema([\"Schema<br/><i>schema</i>\"]):::ghost\n" +
		"  widget_contract_schema -.-> widget_contract_behavior\n" +
		"  classDef draft_con stroke-dasharray:4 3\n" +
		"  classDef draft_int stroke-dasharray:4 3\n" +
		"  classDef ghost stroke-dasharray:2 3\n"
	if got := Mermaid(behavior, MermaidOptions{Palette: PaletteCSS}); got != wantCSS {
		t.Errorf("css palette golden mismatch\n got: %q\nwant: %q", got, wantCSS)
	}
	if strings.Contains(wantCSS, "fill:") || strings.Count(wantCSS, "classDef") != 3 {
		t.Fatal("the CSS golden itself must carry three colourless classDef lines")
	}
	// A phase with no claims, and the excluded block, print nothing.
	for _, name := range []string{"orientation", "api", ExcludedPhaseName} {
		if got := Mermaid(viewByName(t, views, name), MermaidOptions{}); got != "" {
			t.Errorf("%s: expected no diagram, got %q", name, got)
		}
	}
}

func TestNodeLabel_EscapesAndWraps(t *testing.T) {
	// A label that is an unshaped id renders VERBATIM (components.ClaimLabel's
	// rule), which is exactly the input most likely to carry syntax.
	hostile := `say "hi" <b>#1; & done`
	got := NodeLabel(hostile)
	// Every "#" and ";" in the output belongs to a mermaid entity; once those
	// are removed no syntax character survives.
	stripped := strings.NewReplacer("#quot;", "", "#35;", "", "#59;", "", "#lt;", "", "#gt;", "", "#amp;", "").Replace(got)
	if strings.ContainsAny(stripped, `"#;<>&`) {
		t.Errorf("unescaped syntax survives in %q", got)
	}
	// The id is unshaped, so it is verbatim: no DisplayCase; it is exactly 22
	// characters UNESCAPED, so it stays on one line — the wrap measures the
	// text the reader sees, not the entity-expanded bytes.
	if want := "say #quot;hi#quot; #lt;b#gt;#35;1#59; #amp; done"; got != want {
		t.Errorf("verbatim escaping wrong:\n got %q\nwant %q", got, want)
	}

	// A two-segment id equals components.ClaimLabel's verbatim path.
	if got := NodeLabel("only.two"); got != "only.two" {
		t.Errorf("two-segment id label = %q, want verbatim", got)
	}
	// A three-segment id is DisplayCase of the slug, wrapped at 22.
	long := "m.f.mechanism-scoped-observation-authority"
	if got := NodeLabel(long); got != "Mechanism Scoped<br/>Observation Authority" {
		t.Errorf("wrapped label = %q", got)
	}
	for _, line := range strings.Split(NodeLabel(long), "<br/>") {
		if len([]rune(line)) > labelWrap {
			t.Errorf("line %q exceeds %d characters", line, labelWrap)
		}
	}
}

func TestNodeID_Sanitises(t *testing.T) {
	if got := NodeID("widget.contract.the-thing_x/1"); got != "widget_contract_the_thing_x_1" {
		t.Errorf("NodeID = %q", got)
	}
}

func TestPhaseCounts(t *testing.T) {
	v := PhaseView{Claims: []ClaimEntry{{ID: "a"}}, Levels: [][]string{{"a"}}, Locked: 1}
	if got := v.Counts(); got != "1 claim · 1 level · 1 locked" {
		t.Errorf("Counts = %q", got)
	}
	v = PhaseView{Claims: []ClaimEntry{{ID: "a"}, {ID: "b"}}, Levels: [][]string{{"a", "b"}}, Locked: 0}
	if got := v.Counts(); got != "2 claims · 1 level · 0 locked" {
		t.Errorf("Counts = %q", got)
	}
	if PhaseNumber(model.BuildRoleVerification) != 5 || PhaseNumber(model.BuildRoleOutOfScope) != 0 {
		t.Error("PhaseNumber wrong")
	}
	if PhaseDefinition(model.BuildRoleSchema) != model.BuildRoleDefinition(model.BuildRoleSchema) {
		t.Error("PhaseDefinition must delegate to model.BuildRoleDefinition")
	}
}

// A hand-edited artifact whose phase block carries a name the engine does
// not know, or carries one name twice, is refused: Views reads stored blocks
// by the five names in Phases only, so either would silently drop that
// block's claims from every diagram and every count.
func TestViews_RefusesAnUnknownOrDuplicatedPhaseName(t *testing.T) {
	a, claims := viewFixture(t)
	a.Phases = append(a.Phases, PhaseBlock{Phase: "bogus", Claims: []ClaimEntry{{ID: "w.contract.lost"}}})
	_, _, err := Views(a, claims)
	if err == nil {
		t.Fatal("an unknown phase name rendered a short order with no error")
	}
	for _, want := range []string{`"bogus" is not a phase`, "1 claim(s) no diagram would draw"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}

	a, claims = viewFixture(t)
	a.Phases = append(a.Phases, PhaseBlock{Phase: "schema", Claims: []ClaimEntry{{ID: "w.contract.twice"}}})
	_, _, err = Views(a, claims)
	if err == nil || !strings.Contains(err.Error(), `"schema" is stored twice`) {
		t.Errorf("expected a duplicate-phase error, got %v", err)
	}

	// The unedited fixture still renders: the check is on the stored names,
	// not on the count of blocks (an artifact may omit an empty phase).
	if _, _, err := Views(viewFixture(t)); err != nil {
		t.Fatalf("the unedited fixture: %v", err)
	}
}

// A same-module rests_on target the artifact neither places nor excludes is
// refused, not listed under .bo-cross as a dependency of the module on
// itself. Propose places every non-excluded module claim, so only a
// hand-edited artifact reaches this.
func TestViews_RefusesADanglingSameModuleTarget(t *testing.T) {
	a, claims := viewFixture(t)
	a.Phases[0].Claims[0].RestsOn = []string{"w.contract.vanished"}
	_, _, err := Views(a, claims)
	if err == nil {
		t.Fatal("a dangling same-module target was rendered as a cross-module dependency")
	}
	for _, want := range []string{`"w.contract.vanished"`, "same module", "neither places"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not say %q", err, want)
		}
	}
	// The same shape pointing at ANOTHER module is the cross-module list.
	a, claims = viewFixture(t)
	a.Phases[0].Claims[0].RestsOn = []string{"g.contract.vanished"}
	views, _, err := Views(a, claims)
	if err != nil {
		t.Fatalf("a dangling cross-module target must list, not refuse: %v", err)
	}
	if got := viewByName(t, views, "schema").CrossModule["g"]; len(got) != 1 || got[0] != "g.contract.vanished" {
		t.Errorf("cross_module[g] = %v", got)
	}
}

// A PhaseView built by hand (no classes map, which only Views fills) still
// draws every node with a class, draft_int, rather than an empty `:::` that
// mermaid rejects as a syntax error.
func TestMermaid_HandBuiltViewDrawsEveryNodeDraftInt(t *testing.T) {
	v := PhaseView{Number: 1, Name: "schema", Definition: "d", Claims: []ClaimEntry{{ID: "w.contract.a"}, {ID: "w.contract.b"}}}
	out := Mermaid(v, MermaidOptions{})
	for _, want := range []string{"  w_contract_a[\"A\"]:::draft_int\n", "  w_contract_b[\"B\"]:::draft_int\n"} {
		if !strings.Contains(out, want) {
			t.Errorf("hand-built view output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, ":::\n") {
		t.Errorf("hand-built view emits an empty node class:\n%s", out)
	}
}
