package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestBuild_Empty(t *testing.T) {
	cat, err := Build(nil, nil)
	if err != nil {
		t.Fatalf("Build(nil, nil) returned error: %v", err)
	}
	if cat == nil {
		t.Fatal("Build(nil, nil) returned nil catalog")
	}
	if len(cat.Claims) != 0 {
		t.Errorf("expected 0 claims, got %d", len(cat.Claims))
	}
	if len(cat.ByFacet) != 0 {
		t.Errorf("expected empty ByFacet, got %v", cat.ByFacet)
	}
	if len(cat.ByModule) != 0 {
		t.Errorf("expected empty ByModule, got %v", cat.ByModule)
	}

	doc := cat.Document()
	if doc.Claims == nil || len(doc.Claims) != 0 {
		t.Errorf("expected empty non-nil Claims in Document, got %#v", doc.Claims)
	}
	if doc.ByFacet == nil || doc.ByModule == nil {
		t.Errorf("expected non-nil ByFacet/ByModule maps in Document, got %#v / %#v", doc.ByFacet, doc.ByModule)
	}

	// WriteJSON must also succeed against an empty catalog and produce
	// valid, parseable JSON.
	path := filepath.Join(t.TempDir(), "nested", "out.catalog.json")
	if err := WriteJSON(cat, path); err != nil {
		t.Fatalf("WriteJSON on empty catalog: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading written catalog: %v", err)
	}
	var parsed Document
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("parsing written catalog: %v", err)
	}
	if len(parsed.Claims) != 0 {
		t.Errorf("expected 0 claims in written catalog, got %d", len(parsed.Claims))
	}
}

func TestBuild_NilCatalogDocumentAndWrite(t *testing.T) {
	var cat *Catalog
	doc := cat.Document()
	if doc == nil || len(doc.Claims) != 0 {
		t.Fatalf("expected empty non-nil Document from nil *Catalog, got %#v", doc)
	}

	path := filepath.Join(t.TempDir(), "out.catalog.json")
	if err := WriteJSON(nil, path); err != nil {
		t.Fatalf("WriteJSON(nil, path): %v", err)
	}
}

func TestDocument_EdgeSerialization(t *testing.T) {
	claims := []model.Claim{
		{
			ID:     "widget.contract.overview",
			Facet:  "contract",
			Module: "widget",
			Status: model.StatusLocked,
			Layout: model.LayoutCard,
			Governed: model.Governed{
				Type:   "none",
				Reason: "fixture claim, not backed by any real doctrine",
			},
		},
		{
			ID:     "widget.internals.fields",
			Facet:  "internals",
			Module: "widget",
			Status: model.StatusDraft,
			Rows: []model.Row{
				{"field": "id", "type": "string"},
			},
			Mirrors: []string{"widget.contract.overview"},
			RestsOn: []string{"widget.contract.overview", "widget.internals.other"},
			Governed: model.Governed{
				Type: "widget.doctrine.hub",
			},
		},
	}

	cat, err := Build(claims, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	doc := cat.Document()

	if len(doc.Claims) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(doc.Claims))
	}

	byID := map[string]Entry{}
	for _, e := range doc.Claims {
		byID[e.ID] = e
	}

	overview, ok := byID["widget.contract.overview"]
	if !ok {
		t.Fatal("missing widget.contract.overview entry")
	}
	if overview.Facet != "contract" || overview.Module != "widget" {
		t.Errorf("overview facet/module = %q/%q, want contract/widget", overview.Facet, overview.Module)
	}
	if overview.Status != model.StatusLocked {
		t.Errorf("overview status = %q, want locked", overview.Status)
	}
	if overview.Layout != model.LayoutCard {
		t.Errorf("overview layout = %q, want card", overview.Layout)
	}
	if overview.Edges.Mirrors != nil || overview.Edges.RestsOn != nil {
		t.Errorf("overview should have no mirrors/rests_on edges, got %#v", overview.Edges)
	}
	if overview.Edges.GovernedBy == nil || overview.Edges.GovernedBy.Type != "none" ||
		overview.Edges.GovernedBy.Reason != "fixture claim, not backed by any real doctrine" {
		t.Errorf("overview governed_by = %#v, want type=none with reason", overview.Edges.GovernedBy)
	}

	fields, ok := byID["widget.internals.fields"]
	if !ok {
		t.Fatal("missing widget.internals.fields entry")
	}
	if fields.Layout != model.LayoutTable {
		t.Errorf("fields layout = %q, want table (inferred from rows)", fields.Layout)
	}
	if len(fields.Edges.Mirrors) != 1 || fields.Edges.Mirrors[0] != "widget.contract.overview" {
		t.Errorf("fields mirrors = %v, want [widget.contract.overview]", fields.Edges.Mirrors)
	}
	if len(fields.Edges.RestsOn) != 2 {
		t.Errorf("fields rests_on = %v, want 2 entries", fields.Edges.RestsOn)
	}
	if fields.Edges.GovernedBy == nil || fields.Edges.GovernedBy.Type != "widget.doctrine.hub" || fields.Edges.GovernedBy.Reason != "" {
		t.Errorf("fields governed_by = %#v, want type=widget.doctrine.hub with no reason", fields.Edges.GovernedBy)
	}

	// Entries must be sorted by id regardless of input order.
	if doc.Claims[0].ID != "widget.contract.overview" || doc.Claims[1].ID != "widget.internals.fields" {
		t.Errorf("entries not sorted by id: got order %q, %q", doc.Claims[0].ID, doc.Claims[1].ID)
	}
}

func TestDocument_ByFacetByModuleSorted(t *testing.T) {
	claims := []model.Claim{
		{ID: "b.contract.z", Facet: "contract", Module: "b", Status: model.StatusDraft},
		{ID: "a.contract.a", Facet: "contract", Module: "a", Status: model.StatusDraft},
		{ID: "a.contract.m", Facet: "contract", Module: "a", Status: model.StatusDraft},
	}
	cat, err := Build(claims, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	doc := cat.Document()

	wantContract := []string{"a.contract.a", "a.contract.m", "b.contract.z"}
	if got := doc.ByFacet["contract"]; !equalStrings(got, wantContract) {
		t.Errorf("ByFacet[contract] = %v, want %v", got, wantContract)
	}

	wantModuleA := []string{"a.contract.a", "a.contract.m"}
	if got := doc.ByModule["a"]; !equalStrings(got, wantModuleA) {
		t.Errorf("ByModule[a] = %v, want %v", got, wantModuleA)
	}
}

// TestBuild_LargeListDeterminism builds a sizeable, deliberately
// out-of-order claim list twice and asserts both the in-memory Document and
// the bytes written by WriteJSON are identical across builds — Go map
// iteration order must never leak into either.
func TestBuild_LargeListDeterminism(t *testing.T) {
	const n = 500

	// Construct claims with ids/facets/modules that hash to varying bucket
	// orders, deliberately not pre-sorted.
	makeClaims := func() []model.Claim {
		claims := make([]model.Claim, 0, n)
		for i := n - 1; i >= 0; i-- {
			facet := fmt.Sprintf("facet-%d", i%7)
			module := fmt.Sprintf("module-%d", i%11)
			id := fmt.Sprintf("%s.%s.slug-%04d", module, facet, i)
			claims = append(claims, model.Claim{
				ID:     id,
				Facet:  facet,
				Module: module,
				Status: model.StatusDraft,
				Body:   "filler",
				Mirrors: []string{
					fmt.Sprintf("%s.%s.slug-%04d", module, facet, (i+1)%n),
				},
			})
		}
		return claims
	}

	dir := t.TempDir()
	pathA := filepath.Join(dir, "a.catalog.json")
	pathB := filepath.Join(dir, "b.catalog.json")

	catA, err := Build(makeClaims(), nil)
	if err != nil {
		t.Fatalf("Build (run A): %v", err)
	}
	if err := WriteJSON(catA, pathA); err != nil {
		t.Fatalf("WriteJSON (run A): %v", err)
	}

	catB, err := Build(makeClaims(), nil)
	if err != nil {
		t.Fatalf("Build (run B): %v", err)
	}
	if err := WriteJSON(catB, pathB); err != nil {
		t.Fatalf("WriteJSON (run B): %v", err)
	}

	bytesA, err := os.ReadFile(pathA)
	if err != nil {
		t.Fatalf("reading run A output: %v", err)
	}
	bytesB, err := os.ReadFile(pathB)
	if err != nil {
		t.Fatalf("reading run B output: %v", err)
	}

	if len(bytesA) != len(bytesB) {
		t.Fatalf("run A/B output length differs: %d vs %d", len(bytesA), len(bytesB))
	}
	for i := range bytesA {
		if bytesA[i] != bytesB[i] {
			t.Fatalf("run A/B output diverges at byte %d", i)
		}
	}

	docA := catA.Document()
	if len(docA.Claims) != n {
		t.Fatalf("expected %d entries, got %d", n, len(docA.Claims))
	}
	for i := 1; i < len(docA.Claims); i++ {
		if docA.Claims[i-1].ID >= docA.Claims[i].ID {
			t.Fatalf("entries not strictly sorted by id at index %d: %q >= %q",
				i, docA.Claims[i-1].ID, docA.Claims[i].ID)
		}
	}
}

func TestDocument_KindIsEffectiveKind(t *testing.T) {
	claims := []model.Claim{
		{ID: "w.contract.fact", Module: "w", Facet: "contract"},
		{ID: "w.contract.note", Module: "w", Facet: "contract", Kind: model.KindOrientationNote, Layout: model.LayoutBanner},
		{ID: "w.overview.router", Module: "w", Facet: "overview", Layout: model.LayoutBanner},
	}
	cat, err := Build(claims, nil)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	doc := cat.Document()

	got := map[string]model.Kind{}
	for _, e := range doc.Claims {
		got[e.ID] = e.Kind
	}
	want := map[string]model.Kind{
		"w.contract.fact":   model.KindFact,
		"w.contract.note":   model.KindOrientationNote,
		"w.overview.router": model.KindOrientationNote,
	}
	for id, k := range want {
		if got[id] != k {
			t.Errorf("Entry[%q].Kind = %q, want %q", id, got[id], k)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
