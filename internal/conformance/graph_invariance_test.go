package conformance

import (
	"fmt"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestEvaluationRecordCountIsIndependentOfDependencyGraph(t *testing.T) {
	graphs := map[string][]model.Claim{
		"single-edge": {
			{ID: "widget.contract.a", RestsOn: []string{"widget.contract.b"}},
			{ID: "widget.contract.b"},
		},
		"diamond": {
			{ID: "widget.contract.a", RestsOn: []string{"widget.contract.b", "widget.contract.c"}},
			{ID: "widget.contract.b", RestsOn: []string{"widget.contract.d"}},
			{ID: "widget.contract.c", RestsOn: []string{"widget.contract.d"}},
			{ID: "widget.contract.d"},
		},
		"cycle": {
			{ID: "widget.contract.a", RestsOn: []string{"widget.contract.b"}},
			{ID: "widget.contract.b", RestsOn: []string{"widget.contract.a"}},
		},
		"invalid-node": {{ID: "widget.contract.a", RestsOn: []string{"widget.contract.missing"}}},
	}
	deep := make([]model.Claim, 128)
	for i := range deep {
		deep[i].ID = fmt.Sprintf("widget.contract.deep-%03d", i)
		if i+1 < len(deep) {
			deep[i].RestsOn = []string{fmt.Sprintf("widget.contract.deep-%03d", i+1)}
		}
	}
	graphs["deep-128"] = deep
	dense := make([]model.Claim, 24)
	for i := range dense {
		dense[i].ID = fmt.Sprintf("widget.contract.dense-%02d", i)
		for j := i + 1; j < len(dense) && j <= i+5; j++ {
			dense[i].RestsOn = append(dense[i].RestsOn, fmt.Sprintf("widget.contract.dense-%02d", j))
		}
	}
	graphs["dense-24x5"] = dense

	raw := []byte(`{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://root","shape":"set","value":["ready"]}]}`)
	for name, claims := range graphs {
		t.Run(name, func(t *testing.T) {
			claims[0].Embodiment = &model.Embodiment{Mode: model.EmbodimentModeCompare, Checks: []model.EmbodimentCheck{{ID: "primary", Adapter: "neutral/v1", Target: "widget://root", Expectation: &model.EmbodimentExpectation{Shape: model.ExpectationShapeSet, Value: []string{"ready"}}}}}
			report, err := Evaluate(claims, "observations.json", func(string) ([]byte, error) { return raw, nil })
			if err != nil {
				t.Fatal(err)
			}
			if report == nil || len(report.Results) != 1 || len(report.Results[0].Checks) != 1 || report.Results[0].Checks[0].State != StateMatched {
				t.Fatalf("graph amplified or changed comparison: %+v", report)
			}
		})
	}
}
