package lint

import (
	"strconv"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func nClaims(module string, n int) []model.Claim {
	out := make([]model.Claim, n)
	for i := 0; i < n; i++ {
		out[i] = model.Claim{
			ID:     module + ".contract.item-" + strconv.Itoa(i+1),
			Module: module,
			Facet:  "contract",
		}
	}
	return out
}

func TestModuleClaimCapLint(t *testing.T) {
	t.Run("omit uses default 10: 10 claims pass", func(t *testing.T) {
		cfg := &config.Config{Modules: []string{"widget"}}
		findings := moduleClaimCapLint{}.Check(nClaims("widget", config.DefaultMaxClaimsPerModule), cfg)
		if len(findings) != 0 {
			t.Fatalf("at the default cap want 0 findings, got %+v", findings)
		}
	})
	t.Run("omit uses default 10: 11 claims fail once per claim", func(t *testing.T) {
		cfg := &config.Config{Modules: []string{"widget"}}
		findings := moduleClaimCapLint{}.Check(nClaims("widget", config.DefaultMaxClaimsPerModule+1), cfg)
		if len(findings) != config.DefaultMaxClaimsPerModule+1 {
			t.Fatalf("got %d findings, want %d: %+v", len(findings), config.DefaultMaxClaimsPerModule+1, findings)
		}
		for _, f := range findings {
			if f.LintName != "module-claim-cap" || f.Severity != SeverityError {
				t.Fatalf("unexpected finding: %+v", f)
			}
			if !strings.Contains(f.Message, "11") || !strings.Contains(f.Message, "10") {
				t.Fatalf("message must name count and cap: %q", f.Message)
			}
			if !strings.Contains(f.Message, "max_claims_per_module") {
				t.Fatalf("message must name the override key: %q", f.Message)
			}
		}
	})
	t.Run("project claims never count toward a module", func(t *testing.T) {
		cfg := &config.Config{Modules: []string{"widget"}}
		claims := nClaims("widget", config.DefaultMaxClaimsPerModule)
		// Even a malformed project claim that names a module stays out of the
		// count: project claims belong to no module (NIT-25).
		claims = append(claims, model.Claim{ID: "project.retention", Scope: model.ScopeProject, Module: "widget"})
		if findings := (moduleClaimCapLint{}).Check(claims, cfg); len(findings) != 0 {
			t.Fatalf("a project claim must not push a module over the cap, got %+v", findings)
		}
	})
	t.Run("nil config still applies the default", func(t *testing.T) {
		findings := moduleClaimCapLint{}.Check(nClaims("widget", config.DefaultMaxClaimsPerModule+1), nil)
		if len(findings) != config.DefaultMaxClaimsPerModule+1 {
			t.Fatalf("got %d findings, want %d", len(findings), config.DefaultMaxClaimsPerModule+1)
		}
	})
	t.Run("project override raises the cap", func(t *testing.T) {
		cfg := &config.Config{Modules: []string{"widget"}, MaxClaimsPerModule: intPtr(3)}
		if n := len(moduleClaimCapLint{}.Check(nClaims("widget", 3), cfg)); n != 0 {
			t.Fatalf("3 at cap 3: got %d findings", n)
		}
		if n := len(moduleClaimCapLint{}.Check(nClaims("widget", 4), cfg)); n != 4 {
			t.Fatalf("4 over cap 3: got %d findings, want 4", n)
		}
	})
	t.Run("only the fat module is reported", func(t *testing.T) {
		cfg := &config.Config{Modules: []string{"thin", "fat"}, MaxClaimsPerModule: intPtr(2)}
		claims := append(nClaims("thin", 2), nClaims("fat", 3)...)
		findings := moduleClaimCapLint{}.Check(claims, cfg)
		if len(findings) != 3 {
			t.Fatalf("got %d findings, want 3: %+v", len(findings), findings)
		}
		for _, f := range findings {
			if !strings.HasPrefix(f.ClaimID, "fat.") {
				t.Fatalf("finding on the thin module: %+v", f)
			}
		}
	})
	t.Run("empty module field is not counted", func(t *testing.T) {
		cfg := &config.Config{MaxClaimsPerModule: intPtr(1)}
		findings := moduleClaimCapLint{}.Check([]model.Claim{{ID: "x"}, {ID: "y"}}, cfg)
		if len(findings) != 0 {
			t.Fatalf("got %+v", findings)
		}
	})
}
