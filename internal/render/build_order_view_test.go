package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/buildorder"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/components"
)

// TestClaimLabelAgreesWithComponents pins the deliberate duplicate:
// internal/buildorder/mermaid.go's label rule must equal
// components.ClaimLabel for three-segment, two-segment and hyphenated ids,
// so a diagram node and a claim card never label one claim differently. The
// render package may import both; buildorder may not import components.
func TestClaimLabelAgreesWithComponents(t *testing.T) {
	ids := []string{
		"widget.contract.schema",
		"widget.contract.retry-policy",
		"token-ledger.internals.the_thing",
		"only.two",
		"one",
		"a..b",
		"widget.contract.",
		"not an id at all",
	}
	for _, id := range ids {
		want := components.ClaimLabel(id)
		// NodeLabel wraps and escapes; unwrap and unescape the two things a
		// plain label can carry so the comparison is over the label rule.
		got := strings.NewReplacer("<br/>", " ", "#quot;", `"`, "#35;", "#", "#59;", ";", "#lt;", "<", "#gt;", ">", "#amp;", "&").Replace(buildorder.NodeLabel(id))
		if got != want {
			t.Errorf("label of %q: buildorder %q, components %q", id, got, want)
		}
	}
}

// TestRender_BuildOrderTab_SkipsAModuleWhoseArtifactDoesNotLoad pins the
// load-error policy's skip half: one locked artifact and a second module
// whose artifact file is truncated JSON — the tab renders for the first, no
// group exists for the second, and Render returns nil.
func TestRender_BuildOrderTab_SkipsAModuleWhoseArtifactDoesNotLoad(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfgPath := filepath.Join(dir, "project.config.yaml")
	writeFile(t, cfgPath, "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\n  - gadget\nclaims_dir: claims\n")
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	widget := buildOrderTestClaims("widget")
	gadget := buildOrderTestClaims("gadget")
	all := append(append([]model.Claim{}, widget...), gadget...)
	lockBuildOrder(t, cfg, widget, "widget")
	lockBuildOrder(t, cfg, gadget, "gadget")

	// Truncate gadget's artifact after locking.
	gadgetPath := buildorder.ArtifactPath(cfg, "gadget")
	raw, err := os.ReadFile(gadgetPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	writeFile(t, gadgetPath, string(raw[:len(raw)/2]))

	out := renderClaimsFor(t, cfg, all)
	if !strings.Contains(out, `id="dossierx-build-order-widget"`) {
		t.Error("widget's group must render")
	}
	if strings.Contains(out, `id="dossierx-build-order-gadget"`) || strings.Contains(out, `data-target="#dossierx-build-order-gadget"`) {
		t.Error("gadget's unreadable artifact must cost gadget's tab and nothing else")
	}
	if !strings.Contains(out, "gadget.contract.schema") {
		t.Error("gadget's ordinary claims must still render")
	}
}

// TestRender_BuildOrderTab_RetainsArtifactNodesWhenCatalogClaimIsMissing
// pins the stale-but-readable path: the locked artifact still supplies the
// node and Mermaid diagram, while the current catalog controls claim-card
// payload entries and therefore leaves the deleted claim as a client-side
// miss.
func TestRender_BuildOrderTab_RetainsArtifactNodesWhenCatalogClaimIsMissing(t *testing.T) {
	cfg := buildOrderTestConfig(t, "widget")
	claims := buildOrderTestClaims("widget")
	lockBuildOrder(t, cfg, claims, "widget")

	out := renderClaimsFor(t, cfg, claims[:1])
	if !strings.Contains(out, `id="dossierx-build-order-widget"`) {
		t.Fatal("a locked artifact with a missing catalog claim must keep the Build order module")
	}
	messageAt := strings.Index(out, `<p class="bo-missing-claim" role="status">Claim not found</p>`)
	phaseAt := strings.Index(out, `<section class="bo-phase"`)
	if messageAt < 0 || phaseAt < 0 || messageAt > phaseAt {
		t.Fatalf("missing-catalog message must be visible before the affected diagrams: message=%d phase=%d", messageAt, phaseAt)
	}
	if !strings.Contains(out, `data-phase="behavior"`) || !strings.Contains(out, "widget.contract.behavior") {
		t.Fatal("the artifact's missing behavior claim must remain drawable for a click miss")
	}
	if !strings.Contains(out, `id="dossierx-build-orders"`) || !strings.Contains(out, "__esbuild_esm_mermaid_nm") {
		t.Fatal("the retained module must keep the Mermaid payload and scripts")
	}
	if strings.Contains(out, `"widget.contract.behavior":{"facet"`) {
		t.Fatal("the missing catalog claim must not gain a fabricated claim-card payload entry")
	}
}

// TestRender_BuildOrderTab_SkipsAnArtifactThatContradictsThePhasesAndWarns
// pins the other half: a hand-edited locked artifact whose stored edges
// name a later phase costs THAT module its tab entry — never drawn (a
// diagram that lies about the order), never the whole viewer — and
// BuildOrderWarnings names the module and the reason for check's
// warnings[]. A second module's locked order renders untouched beside it.
func TestRender_BuildOrderTab_SkipsAnArtifactThatContradictsThePhasesAndWarns(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfgPath := filepath.Join(dir, "project.config.yaml")
	writeFile(t, cfgPath, "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\n  - gadget\nclaims_dir: claims\n")
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	widget := buildOrderTestClaims("widget")
	gadget := buildOrderTestClaims("gadget")
	all := append(append([]model.Claim{}, widget...), gadget...)
	lockBuildOrder(t, cfg, widget, "widget")
	lockBuildOrder(t, cfg, gadget, "gadget")

	if got := BuildOrderWarnings(cfg, all); len(got) != 0 {
		t.Fatalf("two sound locked orders must produce no warning, got %v", got)
	}

	path := buildorder.ArtifactPath(cfg, "widget")
	a, err := buildorder.LoadArtifact(path)
	if err != nil {
		t.Fatalf("LoadArtifact: %v", err)
	}
	a.Phases[0].Claims[0].RestsOn = []string{"widget.contract.behavior"}
	if err := buildorder.WriteArtifact(a, path); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}

	out := renderClaimsFor(t, cfg, all)
	if strings.Contains(out, `id="dossierx-build-order-widget"`) || strings.Contains(out, `data-target="#dossierx-build-order-widget"`) {
		t.Error("widget's contradicting artifact must not be drawn")
	}
	if !strings.Contains(out, `id="dossierx-build-order-gadget"`) {
		t.Error("gadget's sound order must still render: one module's artifact never costs another's tab")
	}
	if !strings.Contains(out, "widget.contract.schema") {
		t.Error("widget's ordinary claims must still render")
	}

	got := BuildOrderWarnings(cfg, all)
	if len(got) != 1 {
		t.Fatalf("BuildOrderWarnings = %v, want exactly one line for widget", got)
	}
	for _, want := range []string{`build order for module "widget" is not drawn`, "later phase", "dossierx build-order propose --module widget"} {
		if !strings.Contains(got[0], want) {
			t.Errorf("warning %q lacks %q", got[0], want)
		}
	}
}

// TestRender_BuildOrderPayloadShape pins the JSON block the client reads:
// six phases, one module entry with the artifact verbatim, per-claim facts,
// six phase_views and the node_ids index — and that the page's diagram text
// equals the literal-palette export after every classDef line is dropped,
// which is the contract between the viewer and "build-order show".
func TestRender_BuildOrderPayloadShape(t *testing.T) {
	module := "widget"
	cfg := buildOrderTestConfig(t, module)
	claims := buildOrderTestClaims(module)
	lockBuildOrder(t, cfg, claims, module)
	out := renderClaimsFor(t, cfg, claims)
	if strings.Contains(out, `class="bo-missing-claim"`) {
		t.Fatal("a sound artifact must not render a missing-catalog warning")
	}

	open := `<script type="application/json" id="dossierx-build-orders">`
	at := strings.Index(out, open)
	if at < 0 {
		t.Fatal("payload block missing")
	}
	end := strings.Index(out[at:], "</script>")
	var payload struct {
		GeneratedAt string `json:"generated_at"`
		Phases      []struct {
			ID, Name, Definition string
			Number               int
		} `json:"phases"`
		Modules []struct {
			ID       string              `json:"id"`
			Module   string              `json:"module"`
			Label    string              `json:"label"`
			LockedAt string              `json:"locked_at"`
			Stale    bool                `json:"stale"`
			Artifact buildorder.Artifact `json:"artifact"`
			Claims   map[string]struct {
				Facet, Label, Status, Phase string
				Level                       int
			} `json:"claims"`
			PhaseViews []struct {
				Phase        string     `json:"phase"`
				Number       int        `json:"number"`
				Claims       []string   `json:"claims"`
				Levels       [][]string `json:"levels"`
				Ghosts       []struct{ ID, Phase string }
				CrossModule  map[string][]string `json:"cross_module"`
				ExcludedDeps []string            `json:"excluded_deps"`
				Locked       int                 `json:"locked"`
			} `json:"phase_views"`
			NodeIDs map[string]string `json:"node_ids"`
		} `json:"modules"`
	}
	if err := json.Unmarshal([]byte(out[at+len(open):at+end]), &payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if len(payload.Phases) != 6 || payload.Phases[0].ID != "orientation" || payload.Phases[5].ID != "out-of-scope" || payload.Phases[5].Name != "excluded" || payload.Phases[5].Number != 0 {
		t.Errorf("phases = %+v", payload.Phases)
	}
	if len(payload.Modules) != 1 {
		t.Fatalf("modules = %d, want 1", len(payload.Modules))
	}
	m := payload.Modules[0]
	if m.ID != "widget" || m.Label != "Widget" || !m.Artifact.Locked || len(m.Artifact.Phases) != 2 || m.LockedAt == "" {
		t.Errorf("module entry = %+v", m)
	}
	if len(m.PhaseViews) != 6 {
		t.Errorf("phase_views = %d, want 6 (filled by name)", len(m.PhaseViews))
	}
	for _, v := range m.PhaseViews {
		if v.Claims == nil || v.Levels == nil || v.CrossModule == nil || v.ExcludedDeps == nil {
			t.Errorf("phase_views[%s] carries a null where an empty array/object belongs", v.Phase)
		}
	}
	c, ok := m.Claims[module+".contract.behavior"]
	if !ok || c.Facet != "contract" || c.Label != "Behavior" || c.Status != "locked" || c.Phase != "behavior" || c.Level != 0 {
		t.Errorf("claims entry = %+v (present %v)", c, ok)
	}
	if got := m.NodeIDs["widget_contract_schema"]; got != module+".contract.schema" {
		t.Errorf("node_ids inverse = %q", got)
	}

	// Page text vs export text: identical after dropping classDef lines.
	a, err := buildorder.LoadArtifact(buildorder.ArtifactPath(cfg, module))
	if err != nil {
		t.Fatalf("LoadArtifact: %v", err)
	}
	views, _, err := buildorder.Views(a, claims)
	if err != nil {
		t.Fatalf("Views: %v", err)
	}
	for _, v := range views {
		export := buildorder.Mermaid(v, buildorder.MermaidOptions{Palette: buildorder.PaletteLiteral})
		pre := `<pre class="mermaid" data-module="widget" data-phase="` + v.Name + `">`
		pat := strings.Index(out, pre)
		if export == "" {
			if pat >= 0 {
				t.Errorf("%s: the page has a diagram the export does not", v.Name)
			}
			continue
		}
		if pat < 0 {
			t.Fatalf("%s: the export has a diagram the page does not", v.Name)
		}
		pend := strings.Index(out[pat:], "</pre>")
		page := strings.NewReplacer("&lt;", "<", "&gt;", ">", "&#34;", `"`, "&#39;", "'", "&amp;", "&").Replace(out[pat+len(pre) : pat+pend])
		if got, want := dropClassDefs(page), dropClassDefs(export); got != want {
			t.Errorf("%s: page and export differ beyond classDef lines\n page:   %q\n export: %q", v.Name, got, want)
		}
	}
}

func dropClassDefs(text string) string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "  classDef") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// TestStyleOverrideWarnings pins the one render-side warning check carries:
// present with a style.css override beside a locked order, absent once the
// order is unlocked or the file removed, absent with no override directory.
func TestStyleOverrideWarnings(t *testing.T) {
	dir := t.TempDir()
	for _, d := range []string{"claims", "tmpl"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	cfgPath := filepath.Join(dir, "project.config.yaml")
	writeFile(t, cfgPath, "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\nviewer:\n  template_overrides: tmpl\n")
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	claims := buildOrderTestClaims("widget")
	if got := StyleOverrideWarnings(cfg); got != nil {
		t.Errorf("no override file, no order: got %v", got)
	}
	writeFile(t, filepath.Join(dir, "tmpl", "style.css"), "body{}")
	if got := StyleOverrideWarnings(cfg); got != nil {
		t.Errorf("override file, no locked order: got %v", got)
	}
	lockBuildOrder(t, cfg, claims, "widget")
	got := StyleOverrideWarnings(cfg)
	if len(got) != 1 || !strings.HasPrefix(got[0], "viewer.template_overrides/style.css is in force") {
		t.Errorf("override file + locked order: got %v", got)
	}
	if err := os.Remove(filepath.Join(dir, "tmpl", "style.css")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if got := StyleOverrideWarnings(cfg); got != nil {
		t.Errorf("override removed: got %v", got)
	}
	if got := StyleOverrideWarnings(nil); got != nil {
		t.Errorf("nil cfg: got %v", got)
	}
}
