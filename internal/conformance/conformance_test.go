package conformance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func compareClaim(id, adapter, target string, values ...string) model.Claim {
	return model.Claim{ID: id, Embodiment: &model.Embodiment{
		Mode: model.EmbodimentModeCompare,
		Checks: []model.EmbodimentCheck{{ID: "primary", Adapter: adapter, Target: target,
			Expectation: &model.EmbodimentExpectation{Shape: model.ExpectationShapeSet, Value: values}}},
	}}
}

func onlyCheck(t *testing.T, result Result) CheckResult {
	t.Helper()
	if len(result.Checks) != 1 {
		t.Fatalf("checks=%d, want 1: %+v", len(result.Checks), result)
	}
	return result.Checks[0]
}

func envelopeJSON(observations ...Observation) []byte {
	raw, err := json.Marshal(Envelope{FormatVersion: 1, Snapshot: "snapshot-1", Observations: observations})
	if err != nil {
		panic(err)
	}
	return raw
}

func mustEvaluate(t *testing.T, claims []model.Claim, path string, read ReadFunc) *Report {
	t.Helper()
	report, err := Evaluate(claims, path, read)
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func TestEvaluateAllV1StatesAndDeclaredNone(t *testing.T) {
	claims := []model.Claim{
		compareClaim("widget.contract.matched", "adapter/v1", "target/matched", "waiting", "ready"),
		compareClaim("widget.contract.owed", "adapter/v1", "target/owed", "ready"),
		compareClaim("widget.contract.mismatch", "adapter/v1", "target/mismatch", "waiting", "ready", "blocked"),
		compareClaim("widget.contract.uncheckable", "adapter/v1", "target/error", "ready"),
		{ID: "widget.contract.none", Embodiment: &model.Embodiment{Mode: model.EmbodimentModeNone, Reason: "documentation-only contract"}},
	}
	raw := envelopeJSON(
		Observation{Adapter: "adapter/v1", Target: "target/matched", Shape: model.ExpectationShapeSet, Value: []string{"ready", "waiting"}},
		Observation{Adapter: "adapter/v1", Target: "target/mismatch", Shape: model.ExpectationShapeSet, Value: []string{"ready", "paused"}},
		Observation{Adapter: "adapter/v1", Target: "target/error", Error: &ObservationError{Code: "input_unavailable", Message: "snapshot missing"}},
	)
	report := mustEvaluate(t, claims, "observations.json", func(string) ([]byte, error) { return raw, nil })
	if report == nil || len(report.Results) != 5 {
		t.Fatalf("report = %+v", report)
	}
	byID := map[string]Result{}
	for _, result := range report.Results {
		byID[result.ClaimID] = result
	}
	if got := byID["widget.contract.matched"]; onlyCheck(t, got).State != StateMatched || !got.ImplementationReady {
		t.Fatalf("matched = %+v", got)
	}
	if got := byID["widget.contract.owed"]; onlyCheck(t, got).State != StateOwed || got.ImplementationReady {
		t.Fatalf("owed = %+v", got)
	}
	if got := byID["widget.contract.mismatch"]; onlyCheck(t, got).State != StateMismatch || !reflect.DeepEqual(onlyCheck(t, got).Missing, []string{"blocked", "waiting"}) || !reflect.DeepEqual(onlyCheck(t, got).Extra, []string{"paused"}) {
		t.Fatalf("mismatch = %+v", got)
	}
	if got := byID["widget.contract.uncheckable"]; onlyCheck(t, got).State != StateUncheckable || onlyCheck(t, got).ObservationError == nil || onlyCheck(t, got).ObservationError.Code != "input_unavailable" || onlyCheck(t, got).ObservationError.Message != "snapshot missing" || onlyCheck(t, got).Reason != "adapter reported an observation error" {
		t.Fatalf("uncheckable = %+v", got)
	}
	if got := byID["widget.contract.none"]; len(got.Checks) != 0 || !got.ImplementationReady || got.Mode != model.EmbodimentModeNone {
		t.Fatalf("none = %+v", got)
	}
	if report.Summary != (Summary{Declared: 5, DeclaredNone: 1, Checks: 4, Matched: 1, Owed: 1, Mismatch: 1, Uncheckable: 1, Ready: 2, Blocked: 3}) {
		t.Fatalf("summary = %+v", report.Summary)
	}
}

func TestEvaluatePluralChecksAndScalarExactEquality(t *testing.T) {
	claim := model.Claim{ID: "widget.contract.combined", Embodiment: &model.Embodiment{
		Mode: model.EmbodimentModeCompare,
		Checks: []model.EmbodimentCheck{
			{ID: "z.scalar", Adapter: "neutral/v1", Target: "scalar", Expectation: &model.EmbodimentExpectation{Shape: model.ExpectationShapeScalar, Value: "07"}},
			{ID: "a.set", Adapter: "neutral/v1", Target: "set", Expectation: &model.EmbodimentExpectation{Shape: model.ExpectationShapeSet, Value: []string{"b", "a"}}},
		},
	}}
	raw := envelopeJSON(
		Observation{Adapter: "neutral/v1", Target: "set", Shape: model.ExpectationShapeSet, Value: []string{"a", "b"}},
		Observation{Adapter: "neutral/v1", Target: "scalar", Shape: model.ExpectationShapeScalar, Value: "7"},
	)
	report := mustEvaluate(t, []model.Claim{claim}, "observations.json", func(string) ([]byte, error) { return raw, nil })
	result := report.Results[0]
	if result.ImplementationReady || len(result.Checks) != 2 {
		t.Fatalf("result=%+v", result)
	}
	if result.Checks[0].ID != "a.set" || result.Checks[0].State != StateMatched || !result.Checks[0].ImplementationReady {
		t.Fatalf("set=%+v", result.Checks[0])
	}
	if result.Checks[1].ID != "z.scalar" || result.Checks[1].State != StateMismatch || result.Checks[1].Expected != "07" || result.Checks[1].Observed != "7" || len(result.Checks[1].Missing) != 0 || len(result.Checks[1].Extra) != 0 {
		t.Fatalf("scalar=%+v", result.Checks[1])
	}
	if report.Summary != (Summary{Declared: 1, Checks: 2, Matched: 1, Mismatch: 1, Blocked: 1}) {
		t.Fatalf("summary=%+v", report.Summary)
	}
}

func TestScalarObservationStrictStringsOnly(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		wantOK      bool
	}{
		{name: "string", value: `"7"`, wantOK: true},
		{name: "number", value: `7`},
		{name: "boolean", value: `true`},
		{name: "null", value: `null`},
		{name: "array", value: `["7"]`},
		{name: "empty", value: `""`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(fmt.Sprintf(`{"format_version":1,"observations":[{"adapter":"a","target":"t","shape":"scalar","value":%s}]}`, tc.value))
			_, err := DecodeEnvelope(raw)
			if tc.wantOK && err != nil {
				t.Fatal(err)
			}
			if !tc.wantOK && err == nil {
				t.Fatal("expected strict scalar rejection")
			}
		})
	}
}

func TestEvaluateNoDeclarationDoesNotRead(t *testing.T) {
	reads := 0
	report := mustEvaluate(t, []model.Claim{{ID: "widget.contract.plain"}}, "observations.json", func(string) ([]byte, error) { reads++; return nil, errors.New("must not read") })
	if report != nil || reads != 0 {
		t.Fatalf("report=%+v reads=%d, want nil and zero", report, reads)
	}
}

func TestEvaluateInputFailureIsUncheckable(t *testing.T) {
	claim := compareClaim("widget.contract.state", "adapter/v1", "target", "ready")
	for _, tc := range []struct {
		name string
		path string
		read ReadFunc
	}{
		{name: "not configured"},
		{name: "read failure", path: "missing.json", read: func(string) ([]byte, error) { return nil, os.ErrNotExist }},
		{name: "invalid", path: "bad.json", read: func(string) ([]byte, error) { return []byte(`{"format_version":1`), nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			report := mustEvaluate(t, []model.Claim{claim}, tc.path, tc.read)
			if onlyCheck(t, report.Results[0]).State != StateUncheckable {
				t.Fatalf("result = %+v", report.Results[0])
			}
		})
	}
}

func TestDecodeEnvelopeStrictFailures(t *testing.T) {
	oversized := bytes.Repeat([]byte{' '}, MaxInputBytes+1)
	cases := []struct {
		name string
		raw  []byte
		want string
	}{
		{name: "unknown field", raw: []byte(`{"format_version":1,"observations":[],"unknown":true}`), want: "unknown"},
		{name: "duplicate key", raw: []byte(`{"format_version":1,"format_version":1,"observations":[]}`), want: "duplicate"},
		{name: "unsupported version", raw: []byte(`{"format_version":2,"observations":[]}`), want: "unsupported"},
		{name: "missing observations", raw: []byte(`{"format_version":1}`), want: "observations is required"},
		{name: "null observations", raw: []byte(`{"format_version":1,"observations":null}`), want: "cannot be null"},
		{name: "null snapshot", raw: []byte(`{"format_version":1,"snapshot":null,"observations":[]}`), want: "snapshot must be a string"},
		{name: "numeric snapshot", raw: []byte(`{"format_version":1,"snapshot":7,"observations":[]}`), want: "snapshot must be a string"},
		{name: "object snapshot", raw: []byte(`{"format_version":1,"snapshot":{"id":"x"},"observations":[]}`), want: "snapshot must be a string"},
		{name: "unsupported shape", raw: []byte(`{"format_version":1,"observations":[{"adapter":"a","target":"t","shape":"record","value":["x"]}]}`), want: "record"},
		{name: "duplicate pair", raw: []byte(`{"format_version":1,"observations":[{"adapter":"a","target":"t","shape":"set","value":["x"]},{"adapter":"a","target":"t","shape":"set","value":["y"]}]}`), want: "duplicate adapter/target"},
		{name: "duplicate member", raw: []byte(`{"format_version":1,"observations":[{"adapter":"a","target":"t","shape":"set","value":["x","x"]}]}`), want: "duplicate member"},
		{name: "mixed error arm", raw: []byte(`{"format_version":1,"observations":[{"adapter":"a","target":"t","shape":"set","value":["x"],"error":{"code":"failed","message":"no"}}]}`), want: "cannot carry"},
		{name: "error with null shape", raw: []byte(`{"format_version":1,"observations":[{"adapter":"a","target":"t","shape":null,"error":{"code":"failed","message":"no"}}]}`), want: "cannot carry"},
		{name: "error with null value", raw: []byte(`{"format_version":1,"observations":[{"adapter":"a","target":"t","value":null,"error":{"code":"failed","message":"no"}}]}`), want: "cannot carry"},
		{name: "null error", raw: []byte(`{"format_version":1,"observations":[{"adapter":"a","target":"t","error":null}]}`), want: "must be an object"},
		{name: "value missing shape", raw: []byte(`{"format_version":1,"observations":[{"adapter":"a","target":"t","value":["x"]}]}`), want: "requires shape and value"},
		{name: "unknown error field", raw: []byte(`{"format_version":1,"observations":[{"adapter":"a","target":"t","error":{"code":"failed","message":"no","detail":true}}]}`), want: "unknown"},
		{name: "oversized", raw: oversized, want: "maximum"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := DecodeEnvelope(tc.raw)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestEvaluateReadFailureDoesNotExposePath(t *testing.T) {
	absPath := filepath.Join(string(filepath.Separator), "private", "observations.json")
	report := mustEvaluate(t, []model.Claim{compareClaim("widget.contract.state", "adapter/v1", "target", "ready")}, absPath, func(string) ([]byte, error) {
		return nil, errors.New("open " + absPath + ": permission denied")
	})
	result := onlyCheck(t, report.Results[0])
	if result.State != StateUncheckable || result.Reason != "observation input is unavailable" {
		t.Fatalf("result = %+v", result)
	}
	encoded, err := EncodeJSON(report)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte(absPath)) {
		t.Fatalf("serialized report exposed input path: %s", encoded)
	}
}

func TestWriteJSONDeterministic(t *testing.T) {
	report := mustEvaluate(t, []model.Claim{compareClaim("widget.contract.state", "adapter/v1", "target", "b", "a")}, "observations.json", func(string) ([]byte, error) {
		return envelopeJSON(Observation{Adapter: "adapter/v1", Target: "target", Shape: model.ExpectationShapeSet, Value: []string{"a", "b"}}), nil
	})
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.json"), filepath.Join(dir, "b.json")
	if err := WriteJSON(report, a); err != nil {
		t.Fatal(err)
	}
	if err := WriteJSON(report, b); err != nil {
		t.Fatal(err)
	}
	ab, err := os.ReadFile(a)
	if err != nil {
		t.Fatal(err)
	}
	bb, err := os.ReadFile(b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ab, bb) {
		t.Fatalf("outputs differ:\n%s\n%s", ab, bb)
	}
}

func TestReportJSONRoundTripPreservesShapeDependentValues(t *testing.T) {
	report := &Report{FormatVersion: 1, Results: []Result{{
		ClaimID: "widget.contract.values", Mode: model.EmbodimentModeCompare,
		Checks: []CheckResult{
			{ID: "set", Shape: model.ExpectationShapeSet, State: StateMismatch, Expected: []string{"a", "b"}, Observed: []string{"b", "c"}},
			{ID: "scalar", Shape: model.ExpectationShapeScalar, State: StateMismatch, Expected: "07", Observed: "7"},
		},
	}}}
	raw, err := EncodeJSON(report)
	if err != nil {
		t.Fatal(err)
	}
	var decoded Report
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, *report) {
		t.Fatalf("round trip mismatch:\n got: %#v\nwant: %#v", decoded, *report)
	}
}

func TestEncodeJSONRejectsOversizedStatus(t *testing.T) {
	report := &Report{FormatVersion: FormatVersion, Results: []Result{{ClaimID: "widget.contract.none", Mode: model.EmbodimentModeNone, ImplementationReady: true, Reason: strings.Repeat("x", MaxOutputBytes)}}}
	if _, err := EncodeJSON(report); err == nil || !errors.Is(err, ErrCapacityExceeded) || !strings.Contains(err.Error(), "maximum") {
		t.Fatalf("EncodeJSON oversized error = %v", err)
	}
}

func TestReadFileStopsAtCapPlusOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.json")
	if err := os.WriteFile(path, bytes.Repeat([]byte{'x'}, MaxInputBytes+4096), 0o644); err != nil {
		t.Fatal(err)
	}
	raw, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != MaxInputBytes+1 {
		t.Fatalf("read %d bytes, want %d", len(raw), MaxInputBytes+1)
	}
	if _, err := DecodeEnvelope(raw); err == nil || err.Error() != fmt.Sprintf("observation input exceeds maximum of %d bytes", MaxInputBytes) {
		t.Fatalf("DecodeEnvelope reported the capped probe length as the file size: %v", err)
	}
}

func TestReadFileRejectsSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires Unix permissions")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.json")
	link := filepath.Join(dir, "observations.json")
	if err := os.WriteFile(target, []byte(`{"format_version":1,"observations":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(link); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("ReadFile symlink error = %v", err)
	}
}

func TestReadFileOutsideRejectsParentSymlinkIntoBuild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires Unix permissions")
	}
	root := t.TempDir()
	build := filepath.Join(root, "build")
	if err := os.MkdirAll(build, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(build, "observations.json"), []byte(`{"format_version":1,"observations":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "input-alias")
	if err := os.Symlink(build, alias); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFileOutside(filepath.Join(alias, "observations.json"), build); err == nil || !strings.Contains(err.Error(), "resolves inside") {
		t.Fatalf("ReadFileOutside parent-symlink error = %v", err)
	}
	if _, err := ReadFileOutside(filepath.Join(root, "missing", "observations.json"), build); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing input error = %v, want os.ErrNotExist", err)
	}
}

func TestEvaluateBoundedScale5000ClaimsMixedPluralChecks(t *testing.T) {
	const claimsCount, checksPerClaim = 5000, 2
	claims := make([]model.Claim, 0, claimsCount)
	observations := make([]Observation, 0, claimsCount+3*claimsCount/4)
	for i := 0; i < claimsCount; i++ {
		setTarget := fmt.Sprintf("widget://target/%04d/set", i)
		scalarTarget := fmt.Sprintf("widget://target/%04d/scalar", i)
		claims = append(claims, model.Claim{ID: fmt.Sprintf("widget.contract.target-%04d", i), Embodiment: &model.Embodiment{
			Mode: model.EmbodimentModeCompare,
			Checks: []model.EmbodimentCheck{
				{ID: "set", Adapter: "neutral/v1", Target: setTarget, Expectation: &model.EmbodimentExpectation{Shape: model.ExpectationShapeSet, Value: []string{"ready", "waiting"}}},
				{ID: "scalar", Adapter: "neutral/v1", Target: scalarTarget, Expectation: &model.EmbodimentExpectation{Shape: model.ExpectationShapeScalar, Value: "ready"}},
			},
		}})
		observations = append(observations, Observation{Adapter: "neutral/v1", Target: setTarget, Shape: model.ExpectationShapeSet, Value: []string{"waiting", "ready"}})
		switch i % 4 {
		case 0:
			observations = append(observations, Observation{Adapter: "neutral/v1", Target: scalarTarget, Shape: model.ExpectationShapeScalar, Value: "ready"})
		case 1:
			// Deliberately owed: no scalar observation.
		case 2:
			observations = append(observations, Observation{Adapter: "neutral/v1", Target: scalarTarget, Shape: model.ExpectationShapeScalar, Value: "observed"})
		case 3:
			observations = append(observations, Observation{Adapter: "neutral/v1", Target: scalarTarget, Error: &ObservationError{Code: "unavailable", Message: "fixture"}})
		}
	}
	raw := envelopeJSON(observations...)
	if len(raw) > MaxInputBytes {
		t.Fatalf("observation input is %d bytes, cap %d", len(raw), MaxInputBytes)
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	started := time.Now()
	report := mustEvaluate(t, claims, "observations.json", func(string) ([]byte, error) { return raw, nil })
	elapsed := time.Since(started)
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	wantSummary := Summary{
		Declared: claimsCount, Checks: claimsCount * checksPerClaim,
		Matched: claimsCount + claimsCount/4, Owed: claimsCount / 4,
		Mismatch: claimsCount / 4, Uncheckable: claimsCount / 4,
		Ready: claimsCount / 4, Blocked: 3 * claimsCount / 4,
	}
	if report == nil {
		t.Fatal("Evaluate returned a nil report")
	}
	if len(report.Results) != claimsCount || report.Summary != wantSummary {
		t.Fatalf("results=%d summary=%+v, want results=%d summary=%+v", len(report.Results), report.Summary, claimsCount, wantSummary)
	}
	status, err := EncodeJSON(report)
	if err != nil {
		t.Fatal(err)
	}
	if len(status) >= MaxOutputBytes {
		t.Fatalf("encoded status is %d bytes, cap %d", len(status), MaxOutputBytes)
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("evaluation took %s, maximum is under 2s", elapsed)
	}
	if allocated >= 512<<20 {
		t.Fatalf("evaluation allocated %d bytes, maximum is under %d", allocated, 512<<20)
	}
	t.Logf("claims=%d checks=%d elapsed=%s TotalAlloc=%d input_bytes=%d status_bytes=%d", claimsCount, claimsCount*checksPerClaim, elapsed, allocated, len(raw), len(status))
}

func TestEvaluateLargeScalarReportMeasuresInsteadOfFalseRefusing(t *testing.T) {
	const claimCount = 22000
	claims := make([]model.Claim, claimCount)
	for i := range claims {
		id := fmt.Sprintf("widget.contract.c%05d", i)
		claims[i] = model.Claim{ID: id, Embodiment: &model.Embodiment{Mode: model.EmbodimentModeCompare, Checks: []model.EmbodimentCheck{{
			ID: "state", Adapter: "neutral/v1", Target: "widget://shared/scalar",
			Expectation: &model.EmbodimentExpectation{Shape: model.ExpectationShapeScalar, Value: "ready"},
		}}}}
	}
	observation := Observation{Adapter: "neutral/v1", Target: "widget://shared/scalar", Shape: model.ExpectationShapeScalar, Value: "ready"}
	index := map[observationKey]Observation{{observation.Adapter, observation.Target}: observation}
	upper := projectionBudget{limit: MaxOutputBytes}
	upper.add(4096)
	for _, claim := range claims {
		estimateClaimProjection(&upper, claim, index, nil)
		if upper.exceeded {
			break
		}
	}
	if !upper.exceeded {
		t.Fatal("regression fixture no longer reaches the ambiguous upper-bound path")
	}

	raw := []byte(`{"format_version":1,"observations":[{"adapter":"neutral/v1","target":"widget://shared/scalar","shape":"scalar","value":"ready"}]}`)
	report, err := Evaluate(claims, "observations.json", func(string) ([]byte, error) { return raw, nil })
	if err != nil {
		t.Fatalf("padded estimate falsely refused a valid report: %v", err)
	}
	if report == nil || len(report.Results) != claimCount || report.Summary != (Summary{Declared: claimCount, Checks: claimCount, Matched: claimCount, Ready: claimCount}) {
		t.Fatalf("report=%+v", report)
	}
	encoded, err := EncodeJSON(report)
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) >= MaxOutputBytes {
		t.Fatalf("regression fixture is invalid: measured=%d cap=%d", len(encoded), MaxOutputBytes)
	}
	t.Logf("ambiguous conformance estimate measured exactly: claims=%d status_bytes=%d cap=%d", claimCount, len(encoded), MaxOutputBytes)
}

func TestEvaluateRejectsManyShortChecksOnMandatoryStructureBeforeMaterialization(t *testing.T) {
	const checkCount = 320000
	expectation := &model.EmbodimentExpectation{Shape: model.ExpectationShapeScalar, Value: "x"}
	checks := make([]model.EmbodimentCheck, checkCount)
	stringsOnly := projectionBudget{limit: MaxOutputBytes + 1}
	stringsOnly.addExactJSONString("c")
	stringsOnly.addExactJSONString(string(model.EmbodimentModeCompare))
	for i := range checks {
		id := fmt.Sprintf("c%06d", i)
		target := fmt.Sprintf("t%06d", i)
		checks[i] = model.EmbodimentCheck{ID: id, Adapter: "a", Target: target, Expectation: expectation}
		for _, value := range []string{id, "a", target, string(model.ExpectationShapeScalar), "x"} {
			stringsOnly.addExactJSONString(value)
		}
		stringsOnly.add(2) // the projected state string's quotes
	}
	if stringsOnly.exceeded || stringsOnly.total >= MaxOutputBytes {
		t.Fatalf("fixture strings alone reached the cap: %+v", stringsOnly)
	}
	claim := model.Claim{ID: "c", Embodiment: &model.Embodiment{Mode: model.EmbodimentModeCompare, Checks: checks}}
	lower := projectionBudget{limit: MaxOutputBytes + 1}
	lower.add(minimalReportJSONBytes)
	estimateClaimProjectionLowerBound(&lower, claim, nil, errors.New("observation input is not configured"))
	if !lower.exceeded && lower.total <= MaxOutputBytes {
		t.Fatalf("mandatory structure did not prove overflow: strings=%d structural_lower=%d", stringsOnly.total, lower.total)
	}

	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	started := time.Now()
	report, err := Evaluate([]model.Claim{claim}, "", nil)
	elapsed := time.Since(started)
	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc
	if report != nil || !errors.Is(err, ErrCapacityExceeded) || !strings.Contains(err.Error(), "mandatory JSON content") {
		t.Fatalf("report=%v err=%v", report, err)
	}
	if allocated >= 512<<20 {
		t.Fatalf("structural preflight allocated %d bytes; maximum is under %d", allocated, 512<<20)
	}
	t.Logf("short-check structural refusal: checks=%d string_bytes=%d elapsed=%s TotalAlloc=%d", checkCount, stringsOnly.total, elapsed, allocated)
}

func TestEvaluateBoundedPluralCheckCount(t *testing.T) {
	const claimCount, checksPerClaim = 200, 12
	claims := make([]model.Claim, claimCount)
	observations := make([]Observation, 0, claimCount*checksPerClaim)
	for i := range claims {
		claims[i] = model.Claim{ID: fmt.Sprintf("widget.contract.plural-%03d", i), Embodiment: &model.Embodiment{Mode: model.EmbodimentModeCompare}}
		for j := 0; j < checksPerClaim; j++ {
			target := fmt.Sprintf("widget://plural/%03d/%02d", i, j)
			check := model.EmbodimentCheck{ID: fmt.Sprintf("check-%02d", j), Adapter: "neutral/v1", Target: target}
			observation := Observation{Adapter: "neutral/v1", Target: target}
			if j%2 == 0 {
				check.Expectation = &model.EmbodimentExpectation{Shape: model.ExpectationShapeScalar, Value: "ready"}
				observation.Shape, observation.Value = model.ExpectationShapeScalar, "ready"
			} else {
				check.Expectation = &model.EmbodimentExpectation{Shape: model.ExpectationShapeSet, Value: []string{"ready", "waiting"}}
				observation.Shape, observation.Value = model.ExpectationShapeSet, []string{"waiting", "ready"}
			}
			claims[i].Embodiment.Checks = append(claims[i].Embodiment.Checks, check)
			observations = append(observations, observation)
		}
	}
	raw := envelopeJSON(observations...)
	started := time.Now()
	report := mustEvaluate(t, claims, "observations.json", func(string) ([]byte, error) { return raw, nil })
	if got, want := report.Summary.Checks, claimCount*checksPerClaim; got != want || report.Summary.Matched != want || report.Summary.Ready != claimCount {
		t.Fatalf("summary=%+v, want %d matched checks and %d ready claims", report.Summary, want, claimCount)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("plural evaluation took %s, maximum is 2s", elapsed)
	}
}

func TestEvaluateBoundedRejectsSharedTargetFanoutBeforeDifferenceCopies(t *testing.T) {
	const claimsCount = 96
	claims := make([]model.Claim, claimsCount)
	for i := range claims {
		claims[i] = compareClaim(fmt.Sprintf("widget.contract.shared-%03d", i), "neutral/v1", "widget://shared", fmt.Sprintf("expected-%03d", i))
	}
	largeMember := strings.Repeat("x", 1<<20)
	raw := envelopeJSON(Observation{Adapter: "neutral/v1", Target: "widget://shared", Shape: model.ExpectationShapeSet, Value: []string{largeMember}})
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	started := time.Now()
	report, err := Evaluate(claims, "observations.json", func(string) ([]byte, error) { return raw, nil })
	elapsed := time.Since(started)
	runtime.ReadMemStats(&after)
	if err == nil || report != nil || !strings.Contains(err.Error(), "projection requires more than") || !strings.Contains(err.Error(), "at least") {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("shared target preflight took %s", elapsed)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 64<<20 {
		t.Fatalf("shared target preflight allocated %d bytes; per-claim observed copies escaped the budget", allocated)
	}
}

func TestProjectionUpperBoundCoversIndentedAdversarialReport(t *testing.T) {
	members := make([]string, 4000)
	for i := range members {
		members[i] = fmt.Sprintf("member-%04d-\x00-<&>-\\-\"", i)
	}
	claim := compareClaim("widget.contract.escaping", "neutral/<&>", "widget://escaping", members...)
	observation := Observation{Adapter: "neutral/<&>", Target: "widget://escaping", Shape: model.ExpectationShapeSet, Value: append([]string(nil), members...)}
	raw := envelopeJSON(observation)
	report := mustEvaluate(t, []model.Claim{claim}, "observations.json", func(string) ([]byte, error) { return raw, nil })
	encoded, err := EncodeJSON(report)
	if err != nil {
		t.Fatal(err)
	}
	index := map[observationKey]Observation{{observation.Adapter, observation.Target}: observation}
	lower := projectionBudget{limit: 1 << 62}
	estimateClaimProjectionLowerBound(&lower, claim, index, nil)
	if lower.exceeded || lower.total > uint64(len(encoded)) {
		t.Fatalf("encoded report is %d bytes, lower bound was %+v", len(encoded), lower)
	}
	budget := projectionBudget{limit: MaxOutputBytes}
	budget.add(4096)
	estimateClaimProjection(&budget, claim, index, nil)
	if budget.exceeded {
		t.Fatal("adversarial fixture unexpectedly exceeded projection budget")
	}
	if uint64(len(encoded)) > budget.total {
		t.Fatalf("encoded report is %d bytes, conservative bound was only %d", len(encoded), budget.total)
	}
}

func TestEvaluateCapacityStopsHighCardinalitySharedAccountingEarly(t *testing.T) {
	const claimCount = 5000
	claims := make([]model.Claim, claimCount)
	for i := range claims {
		claims[i] = compareClaim(fmt.Sprintf("widget.contract.fanout-%04d", i), "neutral/v1", "widget://fanout", "expected")
	}
	members := make([]string, 16000)
	for i := range members {
		members[i] = fmt.Sprintf("%05d-%s", i, strings.Repeat("x", 240))
	}
	raw := envelopeJSON(Observation{Adapter: "neutral/v1", Target: "widget://fanout", Shape: model.ExpectationShapeSet, Value: members})
	if len(raw) > MaxInputBytes {
		t.Fatalf("fixture is %d bytes, above input cap", len(raw))
	}
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	started := time.Now()
	report, err := Evaluate(claims, "observations.json", func(string) ([]byte, error) { return raw, nil })
	runtime.ReadMemStats(&after)
	if report != nil || !errors.Is(err, ErrCapacityExceeded) {
		t.Fatalf("report=%v err=%v", report, err)
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("capacity accounting did not stop early: %s", elapsed)
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 128<<20 {
		t.Fatalf("capacity accounting allocated %d bytes", allocated)
	}
}

func TestProjectionBudgetSaturatesWithoutOverflow(t *testing.T) {
	b := projectionBudget{limit: MaxOutputBytes}
	b.add(^uint64(0))
	if !b.exceeded || b.total != MaxOutputBytes {
		t.Fatalf("budget after saturation = %+v", b)
	}
}
