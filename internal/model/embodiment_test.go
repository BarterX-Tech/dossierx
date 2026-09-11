package model

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func setCheck(id, adapter, target string, values ...string) EmbodimentCheck {
	return EmbodimentCheck{ID: id, Adapter: adapter, Target: target, Expectation: &EmbodimentExpectation{Shape: ExpectationShapeSet, Value: values}}
}

func TestValidateEmbodimentPluralAndScalar(t *testing.T) {
	valid := &Claim{Embodiment: &Embodiment{Mode: EmbodimentModeCompare, Checks: []EmbodimentCheck{
		{ID: "z.scalar", Adapter: "adapter/v1", Target: "target/scalar", Expectation: &EmbodimentExpectation{Shape: ExpectationShapeScalar, Value: "ready"}},
		setCheck("a.set", "adapter/v1", "target/set", "waiting", "blocked"),
	}}}
	if err := ValidateEmbodiment(valid); err != nil {
		t.Fatal(err)
	}
	if got := []string{valid.Embodiment.Checks[0].ID, valid.Embodiment.Checks[1].ID}; !reflect.DeepEqual(got, []string{"a.set", "z.scalar"}) {
		t.Fatalf("checks not canonicalized: %v", got)
	}
	if got := valid.Embodiment.Checks[0].Expectation.Value; !reflect.DeepEqual(got, []string{"blocked", "waiting"}) {
		t.Fatalf("set not canonicalized: %#v", got)
	}
}

func TestValidateEmbodimentRejectsInvalidDeclarations(t *testing.T) {
	cases := []struct {
		name       string
		embodiment *Embodiment
		want       string
	}{
		{name: "unsupported mode", embodiment: &Embodiment{Mode: "infer"}, want: "mode"},
		{name: "compare without checks", embodiment: &Embodiment{Mode: EmbodimentModeCompare}, want: "at least one"},
		{name: "missing id", embodiment: &Embodiment{Mode: EmbodimentModeCompare, Checks: []EmbodimentCheck{setCheck("", "a", "t", "x")}}, want: ".id"},
		{name: "duplicate id", embodiment: &Embodiment{Mode: EmbodimentModeCompare, Checks: []EmbodimentCheck{setCheck("same", "a", "one", "x"), setCheck("same", "a", "two", "x")}}, want: "duplicate id"},
		{name: "duplicate address", embodiment: &Embodiment{Mode: EmbodimentModeCompare, Checks: []EmbodimentCheck{setCheck("one", "a", "t", "x"), setCheck("two", "a", "t", "y")}}, want: "duplicate adapter/target"},
		{name: "missing adapter", embodiment: &Embodiment{Mode: EmbodimentModeCompare, Checks: []EmbodimentCheck{setCheck("one", "", "t", "x")}}, want: ".adapter"},
		{name: "missing target", embodiment: &Embodiment{Mode: EmbodimentModeCompare, Checks: []EmbodimentCheck{setCheck("one", "a", "", "x")}}, want: ".target"},
		{name: "missing expectation", embodiment: &Embodiment{Mode: EmbodimentModeCompare, Checks: []EmbodimentCheck{{ID: "one", Adapter: "a", Target: "t"}}}, want: "expectation is required"},
		{name: "unsupported shape", embodiment: &Embodiment{Mode: EmbodimentModeCompare, Checks: []EmbodimentCheck{{ID: "one", Adapter: "a", Target: "t", Expectation: &EmbodimentExpectation{Shape: "record"}}}}, want: "record"},
		{name: "empty set", embodiment: &Embodiment{Mode: EmbodimentModeCompare, Checks: []EmbodimentCheck{setCheck("one", "a", "t")}}, want: "at least one"},
		{name: "duplicate member", embodiment: &Embodiment{Mode: EmbodimentModeCompare, Checks: []EmbodimentCheck{setCheck("one", "a", "t", "x", "x")}}, want: "duplicate"},
		{name: "scalar wrong type", embodiment: &Embodiment{Mode: EmbodimentModeCompare, Checks: []EmbodimentCheck{
			{ID: "one", Adapter: "a", Target: "t", Expectation: &EmbodimentExpectation{Shape: ExpectationShapeScalar, Value: []string{"x"}}},
		}}, want: "must be a string"},
		{name: "empty scalar", embodiment: &Embodiment{Mode: EmbodimentModeCompare, Checks: []EmbodimentCheck{
			{ID: "one", Adapter: "a", Target: "t", Expectation: &EmbodimentExpectation{Shape: ExpectationShapeScalar, Value: " "}},
		}}, want: "non-empty"},
		{name: "compare with reason", embodiment: &Embodiment{Mode: EmbodimentModeCompare, Reason: "why", Checks: []EmbodimentCheck{setCheck("one", "a", "t", "x")}}, want: "reason"},
		{name: "none without reason", embodiment: &Embodiment{Mode: EmbodimentModeNone}, want: "reason"},
		{name: "none with checks", embodiment: &Embodiment{Mode: EmbodimentModeNone, Reason: "none", Checks: []EmbodimentCheck{setCheck("one", "a", "t", "x")}}, want: "cannot carry"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateEmbodiment(&Claim{Embodiment: tc.embodiment})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestEmbodimentYAMLStrictShapeAndNoSingularAlias(t *testing.T) {
	valid := `mode: compare
checks:
  - id: scalar
    adapter: neutral/v1
    target: target/scalar
    expectation:
      shape: scalar
      value: "7"
`
	var embodiment Embodiment
	if err := yaml.Unmarshal([]byte(valid), &embodiment); err != nil {
		t.Fatal(err)
	}
	if err := ValidateEmbodiment(&Claim{Embodiment: &embodiment}); err != nil {
		t.Fatal(err)
	}
	if got := embodiment.Checks[0].Expectation.Value; got != "7" {
		t.Fatalf("scalar=%#v", got)
	}
	for _, tc := range []struct{ name, raw, want string }{
		{name: "numeric scalar", raw: strings.Replace(valid, `"7"`, `7`, 1), want: "must be a string"},
		{name: "sequence scalar", raw: strings.Replace(valid, `"7"`, `["7"]`, 1), want: "must be a string"},
		{name: "singular syntax", raw: "mode: compare\nadapter: old\ntarget: old\nexpectation:\n  shape: set\n  value: [x]\n", want: "field adapter not found"},
		{name: "none empty checks key", raw: "mode: none\nreason: no software\nchecks: []\n", want: "cannot carry checks"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got Embodiment
			err := yaml.Unmarshal([]byte(tc.raw), &got)
			if err == nil {
				err = ValidateEmbodiment(&Claim{Embodiment: &got})
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestEmbodimentExpectationYAMLMappingOrderIndependent(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want any
	}{
		{
			name: "scalar value before shape",
			raw: `mode: compare
checks:
  - id: scalar
    adapter: neutral/v1
    target: target/scalar
    expectation:
      value: "7"
      shape: scalar
`,
			want: "7",
		},
		{
			name: "set value before shape",
			raw: `mode: compare
checks:
  - id: set
    adapter: neutral/v1
    target: target/set
    expectation:
      value: [waiting, blocked]
      shape: set
`,
			want: []string{"blocked", "waiting"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var embodiment Embodiment
			if err := yaml.Unmarshal([]byte(tc.raw), &embodiment); err != nil {
				t.Fatal(err)
			}
			if err := ValidateEmbodiment(&Claim{Embodiment: &embodiment}); err != nil {
				t.Fatal(err)
			}
			if got := embodiment.Checks[0].Expectation.Value; !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("expectation value=%#v, want %#v", got, tc.want)
			}
		})
	}
}
