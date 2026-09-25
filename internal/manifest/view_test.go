package manifest

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestShowIsolationCap(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Modules: []string{"widget"}, ClaimsDir: dir}
	if err := WriteMinimal(dir, "widget"); err != nil {
		t.Fatal(err)
	}
	claims := []model.Claim{{
		ID: "widget.contract.bound", Facet: "contract", Module: "widget",
		Status: model.StatusDraft, Body: strings.Repeat("x", MaxIsolationBytes),
	}}
	_, err := Show(claims, cfg, "widget", true, false, true)
	if !IsIsolationOversize(err) {
		t.Fatalf("want isolation oversize, got %v", err)
	}
	view, err := Show(claims, cfg, "widget", true, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if view.ConstitutionDigest.Status != ConstitutionDigestStatusPending {
		t.Fatalf("digest seam: %+v", view.ConstitutionDigest)
	}
	if view.Isolation == nil || len(view.Isolation.DraftHints.SuggestedProvides) != 1 {
		t.Fatalf("draft hints: %+v", view.Isolation)
	}
}

func TestListAndIntegration(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Modules: []string{"widget", "lock"}, ClaimsDir: dir}
	if err := WriteMinimal(dir, "widget"); err != nil {
		t.Fatal(err)
	}
	if err := WriteMinimal(dir, "lock"); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "widget/manifest.yaml", "summary: widget boundary.\nprovides:\n  - widget.contract.bound\ndepends_on:\n  - lock.contract.store\n")
	write(t, dir, "lock/manifest.yaml", "summary: lock store.\nprovides:\n  - lock.contract.store\ndepends_on: []\n")
	claims := []model.Claim{
		{ID: "widget.contract.bound", Facet: "contract", Module: "widget", Status: model.StatusDraft, Body: "bound"},
		{ID: "lock.contract.store", Facet: "contract", Module: "lock", Status: model.StatusDraft, Body: "store"},
	}
	view, err := Show(claims, cfg, "widget", false, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(view.Findings) != 0 {
		t.Fatalf("exported dependency must be clean: %+v", view.Findings)
	}
	if view.Integration == nil || len(view.Integration.Neighbors) != 1 || view.Integration.Neighbors[0].Module != "lock" {
		t.Fatalf("neighbors: %+v", view.Integration)
	}
	if len(view.Integration.Edges) != 1 || view.Integration.Edges[0].ToModule != "lock" {
		t.Fatalf("edges must be membership, not a graph walk: %+v", view.Integration.Edges)
	}
	cat := List(claims, cfg)
	if len(cat) != 2 {
		t.Fatalf("catalog: %+v", cat)
	}
}
