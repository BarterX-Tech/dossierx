package main

import (
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
	if data.ConstitutionDigest.Status != manifest.ConstitutionDigestStatusPending {
		t.Fatalf("constitution digest seam = %+v", data.ConstitutionDigest)
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
