package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/manifest"
)

func init() {
	registerSurfacePayloadType("manifestShowData", manifestShowData{})
	registerSurfacePayloadType("manifestListData", manifestListData{})
	registerSurfacePayloadType("manifestFindingData", manifestFindingData{})
}

func TestManifestShowIsolationAndList(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(claimsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgBody := "schema_version: 1\nfacets:\n  - contract\n  - internals\nmodules:\n  - widget\n  - lock\nclaims_dir: claims\n"
	writeProjectConfigFile(t, filepath.Join(root, "project.config.yaml"), cfgBody)
	if err := os.WriteFile(filepath.Join(claimsDir, "widget", "manifest.yaml"), []byte(
		"summary: widget is the public boundary other modules call.\nprovides:\n  - widget.contract.retry-policy\ndepends_on:\n  - lock.contract.store\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	// lock exports what widget pins: a depends_on id must be in its
	// provider's provides.
	if err := os.WriteFile(filepath.Join(claimsDir, "lock", "manifest.yaml"), []byte(
		"summary: lock is the append-only store widget pins.\nprovides:\n  - lock.contract.store\ndepends_on: []\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claimsDir, "retry.yaml"), []byte(
		"id: widget.contract.retry-policy\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n"+
			"body: |\n  retry policy.\nrests_on:\n  none: true\n  reason: fixture\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claimsDir, "store.yaml"), []byte(
		"id: lock.contract.store\nfacet: contract\nmodule: lock\nstatus: draft\nlayout: card\n"+
			"body: |\n  store contract.\nrests_on:\n  none: true\n  reason: fixture\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(root, "project.config.yaml")
	env, _, err := execCLIJSON(t, "--config", cfgPath, "manifest", "show", "widget")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !env.OK {
		t.Fatalf("show failed: %+v", env.Error)
	}
	var data struct {
		ConstitutionDigest manifest.ConstitutionDigest `json:"constitution_digest"`
	}
	envData(t, env, &data)
	// No constitution.yaml in this project: the digest says so, and the
	// gate's state is carried through rather than left as a seam.
	if data.ConstitutionDigest.Present || data.ConstitutionDigest.State != "missing" || data.ConstitutionDigest.WordCap != 800 {
		t.Fatalf("constitution digest = %+v", data.ConstitutionDigest)
	}

	iso, _, err := execCLIJSON(t, "--config", cfgPath, "manifest", "show", "widget", "--isolation", "--integration")
	if err != nil {
		t.Fatalf("isolation: %v", err)
	}
	if !iso.OK {
		t.Fatalf("isolation failed: %+v", iso.Error)
	}
	var idata manifestShowData
	envData(t, iso, &idata)
	if idata.Isolation == nil || idata.Integration == nil {
		t.Fatalf("expected isolation+integration: %+v", idata)
	}

	unknown, _, err := execCLIJSON(t, "--config", cfgPath, "manifest", "show", "ghost")
	if err == nil || unknown.Error == nil || unknown.Error.Code != cliout.CodeUnknownModule {
		t.Fatalf("unknown module: err=%v env=%+v", err, unknown.Error)
	}

	list, _, err := execCLIJSON(t, "--config", cfgPath, "manifest", "list")
	if err != nil || !list.OK {
		t.Fatalf("list: err=%v env=%+v", err, list)
	}
}

func TestManifestShowRefusesMissing(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(filepath.Join(claimsDir, "widget"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfgBody := "schema_version: 1\nfacets:\n  - contract\n  - internals\nmodules:\n  - widget\nclaims_dir: claims\n"
	if err := os.WriteFile(filepath.Join(root, "project.config.yaml"), []byte(cfgBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(claimsDir, "main.yaml"), []byte(
		"id: widget.contract.main\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n"+
			"body: |\n  main.\nrests_on:\n  none: true\n  reason: fixture\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	env, _, err := execCLIJSON(t, "--config", filepath.Join(root, "project.config.yaml"), "manifest", "show", "widget")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeLintFailed {
		t.Fatalf("missing manifest: err=%v env=%+v", err, env.Error)
	}
	if !strings.Contains(env.Error.Message, "module-manifest") {
		t.Fatalf("message: %s", env.Error.Message)
	}
}

// A module over its isolation budget is refused with view_too_large, and
// the refusal names the module and carries the byte accounting. There is no
// --bodies flag to reach for.
func TestManifestShowIsolationModuleBudget(t *testing.T) {
	root := t.TempDir()
	claimsDir := filepath.Join(root, "claims")
	if err := os.MkdirAll(filepath.Join(claimsDir, "widget"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeProjectConfigFile(t, filepath.Join(root, "project.config.yaml"),
		"schema_version: 1\nfacets:\n  - contract\n  - internals\nmodules:\n  - widget\nclaims_dir: claims\nmax_claims_per_module: 30\n")
	if err := os.WriteFile(filepath.Join(claimsDir, "widget", "manifest.yaml"), []byte(
		"summary: widget is the public boundary other modules call.\nprovides: []\ndepends_on: []\n",
	), 0o644); err != nil {
		t.Fatal(err)
	}
	summary := strings.Repeat("s", 199) + "."
	for i := 0; i < 25; i++ {
		body := fmt.Sprintf("id: widget.contract.rule-%02d\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n"+
			"summary: %s\nbody: |\n  rule.\nrests_on:\n  none: true\n  reason: fixture\n", i, summary)
		if err := os.WriteFile(filepath.Join(claimsDir, fmt.Sprintf("r%02d.yaml", i)), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfgPath := filepath.Join(root, "project.config.yaml")
	env, _, err := execCLIJSON(t, "--config", cfgPath, "manifest", "show", "widget", "--isolation")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeViewTooLarge {
		t.Fatalf("want view_too_large: err=%v env=%+v", err, env.Error)
	}
	if !strings.Contains(env.Error.Message, `module "widget"`) || !strings.Contains(env.Error.Message, "25 claim summaries") {
		t.Fatalf("refusal must name the module and its claim count: %s", env.Error.Message)
	}
	details, ok := env.Error.Details.(map[string]any)
	if !ok || details["module"] != "widget" || details["module_budget"] != float64(manifest.ModuleBudgetBytes) {
		t.Fatalf("details: %+v", env.Error.Details)
	}

	// --bodies is gone. The refusal must be the flag parser's, not the budget's:
	// this module is over budget, so view_too_large would also be an error.
	env, _, err = execCLIJSON(t, "--config", cfgPath, "manifest", "show", "widget", "--isolation", "--bodies")
	if err == nil || env.Error == nil || env.Error.Code != cliout.CodeUsage || !strings.Contains(env.Error.Message, "unknown flag") {
		t.Fatalf("--bodies must be rejected as an unknown flag: err=%v env=%+v", err, env.Error)
	}
}
