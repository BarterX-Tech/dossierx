package loader

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

const validSetCheck = `checks:
  - id: public-values
    adapter: adapter/v1
    target: target
    expectation:
      shape: set
      value: [ready]
`

func TestLoadClaimsRejectsMalformedEmbodiment(t *testing.T) {
	cases := []struct{ name, embodiment, want string }{
		{name: "singular syntax has no alias", embodiment: "mode: compare\nadapter: adapter/v1\ntarget: target\nexpectation:\n  shape: set\n  value: [ready]", want: "field adapter not found"},
		{name: "unsupported shape", embodiment: "mode: compare\nchecks:\n  - id: one\n    adapter: adapter/v1\n    target: target\n    expectation:\n      shape: record\n      value: [ready]", want: "record"},
		{name: "duplicate set", embodiment: "mode: compare\nchecks:\n  - id: one\n    adapter: adapter/v1\n    target: target\n    expectation:\n      shape: set\n      value: [ready, ready]", want: "duplicate"},
		{name: "duplicate check id", embodiment: "mode: compare\nchecks:\n  - id: same\n    adapter: adapter/v1\n    target: one\n    expectation: {shape: set, value: [ready]}\n  - id: same\n    adapter: adapter/v1\n    target: two\n    expectation: {shape: set, value: [ready]}", want: "duplicate id"},
		{name: "duplicate address", embodiment: "mode: compare\nchecks:\n  - id: one\n    adapter: adapter/v1\n    target: same\n    expectation: {shape: set, value: [ready]}\n  - id: two\n    adapter: adapter/v1\n    target: same\n    expectation: {shape: scalar, value: ready}", want: "duplicate adapter/target"},
		{name: "unknown nested field", embodiment: "mode: none\nreason: no software surface\ninvented: true", want: "invented"},
		{name: "compare forbids null reason", embodiment: "mode: compare\nreason: null\n" + validSetCheck, want: "must be a string"},
		{name: "none forbids null checks", embodiment: "mode: none\nreason: no software surface\nchecks: null", want: "must be a sequence"},
		{name: "none forbids empty checks", embodiment: "mode: none\nreason: no software surface\nchecks: []", want: "cannot carry"},
		{name: "unknown check field", embodiment: "mode: compare\nchecks:\n  - id: one\n    adapter: adapter/v1\n    target: target\n    invented: true\n    expectation: {shape: set, value: [ready]}", want: "invented"},
		{name: "unknown expectation field", embodiment: "mode: compare\nchecks:\n  - id: one\n    adapter: adapter/v1\n    target: target\n    expectation:\n      shape: set\n      value: [ready]\n      invented: true", want: "invented"},
		{name: "numeric id", embodiment: "mode: compare\nchecks:\n  - id: 42\n    adapter: adapter/v1\n    target: target\n    expectation: {shape: set, value: [ready]}", want: "id must be a string"},
		{name: "numeric adapter", embodiment: "mode: compare\nchecks:\n  - id: one\n    adapter: 42\n    target: target\n    expectation: {shape: set, value: [ready]}", want: "adapter must be a string"},
		{name: "boolean target", embodiment: "mode: compare\nchecks:\n  - id: one\n    adapter: adapter/v1\n    target: true\n    expectation: {shape: set, value: [ready]}", want: "target must be a string"},
		{name: "mapped reason", embodiment: "mode: none\nreason: {why: none}", want: "reason must be a string"},
		{name: "null reason", embodiment: "mode: none\nreason: null", want: "reason must be a string"},
		{name: "numeric member", embodiment: "mode: compare\nchecks:\n  - id: one\n    adapter: adapter/v1\n    target: target\n    expectation: {shape: set, value: [ready, 1]}", want: "value[1] must be a string"},
		{name: "numeric scalar", embodiment: "mode: compare\nchecks:\n  - id: one\n    adapter: adapter/v1\n    target: target\n    expectation: {shape: scalar, value: 1}", want: "value must be a string"},
		{name: "sequence scalar", embodiment: "mode: compare\nchecks:\n  - id: one\n    adapter: adapter/v1\n    target: target\n    expectation: {shape: scalar, value: [ready]}", want: "value must be a string"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeClaim(t, dir, tc.embodiment)
			_, err := LoadClaims(dir)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoadClaimsCanonicalizesPluralChecksAndSetMembers(t *testing.T) {
	dir := t.TempDir()
	writeClaim(t, dir, `mode: compare
checks:
  - id: z.scalar
    adapter: adapter/v1
    target: scalar
    expectation: {shape: scalar, value: "07"}
  - id: a.set
    adapter: adapter/v1
    target: set
    expectation: {shape: set, value: [waiting, blocked]}`)
	claims, err := LoadClaims(dir)
	if err != nil {
		t.Fatal(err)
	}
	checks := claims[0].Embodiment.Checks
	if got := []string{checks[0].ID, checks[1].ID}; !reflect.DeepEqual(got, []string{"a.set", "z.scalar"}) {
		t.Fatalf("check order=%v", got)
	}
	if got := checks[0].Expectation.Value; !reflect.DeepEqual(got, []string{"blocked", "waiting"}) {
		t.Fatalf("set order=%#v", got)
	}
	if got := checks[1].Expectation.Value; got != "07" {
		t.Fatalf("scalar=%#v", got)
	}
}

func TestSaveClaimCreatesShapeDependentEmbodimentValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "claim.yaml")
	claim := model.Claim{
		ID: "widget.contract.state", Facet: "contract", Module: "widget", Status: model.StatusDraft,
		Layout: model.LayoutCard, Body: "State vocabulary.", SourcePath: path,
		Governed: model.Governed{Type: "none", Reason: "fixture"},
		Embodiment: &model.Embodiment{Mode: model.EmbodimentModeCompare, Checks: []model.EmbodimentCheck{
			{ID: "set", Adapter: "adapter/v1", Target: "set", Expectation: &model.EmbodimentExpectation{Shape: model.ExpectationShapeSet, Value: []string{"blocked", "ready"}}},
			{ID: "scalar", Adapter: "adapter/v1", Target: "scalar", Expectation: &model.EmbodimentExpectation{Shape: model.ExpectationShapeScalar, Value: "07"}},
		}},
	}
	if err := SaveClaim(claim); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadClaims(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded[0].Embodiment.Checks[0].Expectation.Value; got != "07" {
		t.Fatalf("scalar round trip=%#v", got)
	}
	if got := loaded[0].Embodiment.Checks[1].Expectation.Value; !reflect.DeepEqual(got, []string{"blocked", "ready"}) {
		t.Fatalf("set round trip=%#v", got)
	}
}

func writeClaim(t *testing.T, dir, embodiment string) {
	t.Helper()
	body := "id: widget.contract.state\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nbody: State vocabulary.\ngoverned_by:\n  type: none\n  reason: fixture\nembodiment:\n  " + strings.ReplaceAll(embodiment, "\n", "\n  ") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "claim.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
