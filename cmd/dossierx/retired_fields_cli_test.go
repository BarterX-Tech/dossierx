// retired_fields_cli_test.go pins the recovery a corpus from an older release
// meets at load: a claim still carrying a retired field (build_role,
// governed_by) or a config still setting doctrine_facet keeps its error code,
// and the hint names the retired field and sends the agent to the
// dossierx-upgrading skill, the only place the fold is written down.
package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
)

func TestRetiredFieldLoadRefusalNamesTheUpgradeFold(t *testing.T) {
	const baseConfig = "schema_version: 1\nfacets:\n  - contract\n  - internals\nmodules:\n  - widget\nclaims_dir: claims\n"
	const claimYAML = "id: widget.contract.retry-policy\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nsummary: Fixture claim used by the engine test corpus.\nbody: retries.\nrests_on:\n  none: true\n  reason: fixture claim\n"

	cases := []struct {
		name      string
		config    string
		claimLine string
		wantCode  cliout.Code
		wantHint  []string
	}{
		{"build_role", baseConfig, "build_role: schema\n", cliout.CodeInvalidClaim,
			[]string{"`build_role` is a retired claim field", "dossierx skills export", "dossierx-upgrading", `"build_role is gone"`}},
		{"governed_by", baseConfig, "governed_by:\n  - widget.contract.other\n", cliout.CodeInvalidClaim,
			[]string{"`governed_by` is a retired claim field", "dossierx skills export", "dossierx-upgrading", `"governed_by and the doctrine hub are gone"`}},
		{"doctrine_facet", baseConfig + "doctrine_facet: doctrine\n", "", cliout.CodeInvalidConfig,
			[]string{"`doctrine_facet` is a retired config field", "dossierx skills export", "dossierx-upgrading", `"governed_by and the doctrine hub are gone"`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			claimsDir := filepath.Join(root, "claims", "widget")
			if err := os.MkdirAll(claimsDir, 0o755); err != nil {
				t.Fatal(err)
			}
			cfgPath := filepath.Join(root, "project.config.yaml")
			writeProjectConfigFile(t, cfgPath, tc.config)
			if err := os.WriteFile(filepath.Join(claimsDir, "retry.yaml"), []byte(claimYAML+tc.claimLine), 0o644); err != nil {
				t.Fatal(err)
			}

			env, _, err := execCLIJSON(t, "--config", cfgPath, "claim", "list")
			if err == nil || env.Error == nil {
				t.Fatalf("a retired field must still refuse the load, got %+v", env)
			}
			if env.Error.Code != tc.wantCode {
				t.Fatalf("code = %q, want %q (message %q)", env.Error.Code, tc.wantCode, env.Error.Message)
			}
			for _, want := range tc.wantHint {
				if !strings.Contains(env.Error.Hint, want) {
					t.Fatalf("hint must carry %q, got %q", want, env.Error.Hint)
				}
			}
		})
	}

	// A field that was never part of the schema is not a retired one: the
	// refusal is the same, and no upgrade fold is invented for it.
	t.Run("unknown field", func(t *testing.T) {
		root := t.TempDir()
		claimsDir := filepath.Join(root, "claims", "widget")
		if err := os.MkdirAll(claimsDir, 0o755); err != nil {
			t.Fatal(err)
		}
		cfgPath := filepath.Join(root, "project.config.yaml")
		writeProjectConfigFile(t, cfgPath, baseConfig)
		if err := os.WriteFile(filepath.Join(claimsDir, "retry.yaml"), []byte(claimYAML+"totally_unknown: 1\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		env, _, _ := execCLIJSON(t, "--config", cfgPath, "claim", "list")
		if env.Error == nil || env.Error.Code != cliout.CodeInvalidClaim {
			t.Fatalf("want invalid_claim, got %+v", env)
		}
		if strings.Contains(env.Error.Hint, "dossierx-upgrading") {
			t.Fatalf("an unknown field is not a retired one, got hint %q", env.Error.Hint)
		}
	})
}
