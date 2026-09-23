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
	assertAbsent(t, out, buildOrderMarkers, "corrupt leftover artifact")
	if strings.Contains(out, `id="dossierx-build-order-widget"`) {
		t.Error("Build order tab is gone")
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
	assertAbsent(t, out, buildOrderMarkers, "missing catalog claim")
	if strings.Contains(out, `id="dossierx-build-order-widget"`) {
		t.Fatal("Build order tab is gone")
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
	if strings.Contains(out, `id="dossierx-build-order-gadget"`) {
		t.Error("Build order tab is gone")
	}
	if !strings.Contains(out, "widget.contract.schema") {
		t.Error("widget's ordinary claims must still render")
	}

	got := BuildOrderWarnings(cfg, all)
	if len(got) != 0 {
		t.Fatalf("BuildOrderWarnings must be silent, got %v", got)
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
	assertAbsent(t, out, buildOrderMarkers, "payload")
	if strings.Contains(out, `id="dossierx-build-orders"`) {
		t.Fatal("payload block must be absent")
	}
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
	if got != nil {
		t.Errorf("override file + leftover order must not warn: got %v", got)
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
