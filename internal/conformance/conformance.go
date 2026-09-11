// Package conformance compares project-neutral structured claim expectations
// with normalized observations produced by project-owned adapters.
package conformance

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/BarterX-Tech/dossierx/internal/atomicfile"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

const (
	FormatVersion  = 1
	MaxInputBytes  = 16 << 20
	MaxOutputBytes = 64 << 20
)

// ErrCapacityExceeded is returned before any generated artifact is replaced
// when a declared projection cannot be held inside the v1 output budget.
var ErrCapacityExceeded = errors.New("conformance capacity exceeded")

// ErrInputTooLarge lets worktree and git-index readers produce the same
// deterministic uncheckable reason without exposing path or git prose.
var ErrInputTooLarge = errors.New("observation input exceeds capacity")

// State is one current comparison outcome. Declared-none is represented by
// Result.Mode and is deliberately not a State.
type State string

const (
	StateMatched     State = "matched"
	StateOwed        State = "owed"
	StateMismatch    State = "mismatch"
	StateUncheckable State = "uncheckable"
)

// ReadFunc supplies observation bytes. Production passes os.ReadFile; staged
// validation passes its git-index reader so evidence is never mixed across
// snapshots.
type ReadFunc func(path string) ([]byte, error)

// Observation is one normalized adapter result in an Envelope.
type Observation struct {
	Adapter string                 `json:"adapter"`
	Target  string                 `json:"target"`
	Shape   model.ExpectationShape `json:"shape,omitempty"`
	Value   any                    `json:"value,omitempty"`
	Error   *ObservationError      `json:"error,omitempty"`

	shapePresent bool
	valuePresent bool
	errorPresent bool
}

// ObservationError is the mutually exclusive adapter-failure arm. Its
// presence proves the target was attempted and failed, so it is uncheckable
// rather than owed.
type ObservationError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// UnmarshalJSON keeps the observation union closed and records key presence,
// so a forbidden key cannot be smuggled into an arm with a null value.
func (o *Observation) UnmarshalJSON(raw []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	*o = Observation{}
	for name := range fields {
		if name != "adapter" && name != "target" && name != "shape" && name != "value" && name != "error" {
			return fmt.Errorf("json: unknown field %q", name)
		}
	}
	if value, ok := fields["adapter"]; ok {
		if err := json.Unmarshal(value, &o.Adapter); err != nil {
			return fmt.Errorf("adapter: %w", err)
		}
	}
	if value, ok := fields["target"]; ok {
		if err := json.Unmarshal(value, &o.Target); err != nil {
			return fmt.Errorf("target: %w", err)
		}
	}
	if value, ok := fields["shape"]; ok {
		o.shapePresent = true
		if err := json.Unmarshal(value, &o.Shape); err != nil {
			return fmt.Errorf("shape: %w", err)
		}
	}
	if value, ok := fields["value"]; ok {
		o.valuePresent = true
		switch o.Shape {
		case model.ExpectationShapeSet:
			var decoded []string
			if err := json.Unmarshal(value, &decoded); err != nil {
				return fmt.Errorf("value: must be an array of strings for shape %q: %w", o.Shape, err)
			}
			o.Value = decoded
		case model.ExpectationShapeScalar:
			var decoded string
			if err := json.Unmarshal(value, &decoded); err != nil {
				return fmt.Errorf("value: must be a string for shape %q: %w", o.Shape, err)
			}
			o.Value = decoded
		default:
			o.Value = nil
		}
	}
	if value, ok := fields["error"]; ok {
		o.errorPresent = true
		if string(value) != "null" {
			var observationError ObservationError
			if err := json.Unmarshal(value, &observationError); err != nil {
				return fmt.Errorf("error: %w", err)
			}
			o.Error = &observationError
		}
	}
	return nil
}

// UnmarshalJSON prevents custom decoding of Observation from weakening strict
// unknown-field rejection inside its error object.
func (e *ObservationError) UnmarshalJSON(raw []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	*e = ObservationError{}
	for name, value := range fields {
		switch name {
		case "code":
			if err := json.Unmarshal(value, &e.Code); err != nil {
				return fmt.Errorf("code: %w", err)
			}
		case "message":
			if err := json.Unmarshal(value, &e.Message); err != nil {
				return fmt.Errorf("message: %w", err)
			}
		default:
			return fmt.Errorf("json: unknown field %q", name)
		}
	}
	return nil
}

// Envelope is the strict, versioned observation input.
type Envelope struct {
	FormatVersion int           `json:"format_version"`
	Snapshot      string        `json:"snapshot,omitempty"`
	Observations  []Observation `json:"observations"`
}

// CheckResult is one deterministic check projection.
type CheckResult struct {
	ID                  string                 `json:"id"`
	Adapter             string                 `json:"adapter"`
	Target              string                 `json:"target"`
	Shape               model.ExpectationShape `json:"shape"`
	State               State                  `json:"state"`
	ImplementationReady bool                   `json:"implementation_ready"`
	Expected            any                    `json:"expected,omitempty"`
	Observed            any                    `json:"observed,omitempty"`
	Missing             []string               `json:"missing,omitempty"`
	Extra               []string               `json:"extra,omitempty"`
	Reason              string                 `json:"reason,omitempty"`
	ObservationError    *ObservationError      `json:"observation_error,omitempty"`
}

// UnmarshalJSON preserves the shape-specific concrete value types during a
// generated status round trip; encoding/json would otherwise turn sets held in
// an interface into []any.
func (r *CheckResult) UnmarshalJSON(raw []byte) error {
	type wire struct {
		ID                  string                 `json:"id"`
		Adapter             string                 `json:"adapter"`
		Target              string                 `json:"target"`
		Shape               model.ExpectationShape `json:"shape"`
		State               State                  `json:"state"`
		ImplementationReady bool                   `json:"implementation_ready"`
		Expected            json.RawMessage        `json:"expected"`
		Observed            json.RawMessage        `json:"observed"`
		Missing             []string               `json:"missing"`
		Extra               []string               `json:"extra"`
		Reason              string                 `json:"reason"`
		ObservationError    *ObservationError      `json:"observation_error"`
	}
	var decoded wire
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return err
	}
	*r = CheckResult{
		ID: decoded.ID, Adapter: decoded.Adapter, Target: decoded.Target,
		Shape: decoded.Shape, State: decoded.State, ImplementationReady: decoded.ImplementationReady,
		Missing: decoded.Missing, Extra: decoded.Extra, Reason: decoded.Reason, ObservationError: decoded.ObservationError,
	}
	var err error
	if len(decoded.Expected) != 0 && string(decoded.Expected) != "null" {
		r.Expected, err = decodeResultValue(decoded.Shape, decoded.Expected)
		if err != nil {
			return fmt.Errorf("expected: %w", err)
		}
	}
	if len(decoded.Observed) != 0 && string(decoded.Observed) != "null" {
		r.Observed, err = decodeResultValue(decoded.Shape, decoded.Observed)
		if err != nil {
			return fmt.Errorf("observed: %w", err)
		}
	}
	return nil
}

func decodeResultValue(shape model.ExpectationShape, raw json.RawMessage) (any, error) {
	switch shape {
	case model.ExpectationShapeSet:
		var value []string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return value, nil
	case model.ExpectationShapeScalar:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, err
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported shape %q", shape)
	}
}

// Result is the deterministic projection for one claim declaration.
type Result struct {
	ClaimID             string               `json:"claim_id"`
	Mode                model.EmbodimentMode `json:"mode"`
	ImplementationReady bool                 `json:"implementation_ready"`
	Checks              []CheckResult        `json:"checks,omitempty"`
	Reason              string               `json:"reason,omitempty"`
}

// Summary counts declared claims without claiming whole-project readiness.
type Summary struct {
	Declared     int `json:"declared"`
	DeclaredNone int `json:"declared_none"`
	Checks       int `json:"checks"`
	Matched      int `json:"matched"`
	Owed         int `json:"owed"`
	Mismatch     int `json:"mismatch"`
	Uncheckable  int `json:"uncheckable"`
	Ready        int `json:"implementation_ready"`
	Blocked      int `json:"implementation_blocked"`
}

// Report is build/conformance/status.json. Results are sorted by claim id.
type Report struct {
	FormatVersion int      `json:"format_version"`
	Snapshot      string   `json:"snapshot,omitempty"`
	Summary       Summary  `json:"summary"`
	Results       []Result `json:"results"`
}

// Evaluate returns nil without reading when no claim declares embodiment.
// Otherwise it returns exactly one result per declaration, including
// mode:none. It rejects before retaining differences when an exact lower bound
// proves overflow; ambiguous cases are materialized and measured exactly.
func Evaluate(claims []model.Claim, observationsPath string, read ReadFunc) (*Report, error) {
	declared := make([]model.Claim, 0)
	compareCount := 0
	for _, claim := range claims {
		if claim.Embodiment == nil {
			continue
		}
		declared = append(declared, claim)
		if claim.Embodiment.Mode == model.EmbodimentModeCompare {
			compareCount++
		}
	}
	if len(declared) == 0 {
		return nil, nil
	}
	sort.Slice(declared, func(i, j int) bool { return declared[i].ID < declared[j].ID })

	report := &Report{FormatVersion: FormatVersion}
	var envelope *Envelope
	var inputErr error
	if compareCount > 0 {
		switch {
		case strings.TrimSpace(observationsPath) == "":
			inputErr = errors.New("observation input is not configured")
		case read == nil:
			inputErr = errors.New("observation input is unavailable")
		default:
			raw, err := read(observationsPath)
			if err != nil {
				if errors.Is(err, ErrInputTooLarge) {
					inputErr = errors.New("observation input is invalid")
				} else {
					inputErr = errors.New("observation input is unavailable")
				}
			} else {
				envelope, err = DecodeEnvelope(raw)
				if err != nil {
					inputErr = errors.New("observation input is invalid")
				}
			}
		}
	}
	if envelope != nil {
		report.Snapshot = envelope.Snapshot
	}

	index := make(map[observationKey]Observation)
	if envelope != nil {
		for _, observation := range envelope.Observations {
			index[observationKey{observation.Adapter, observation.Target}] = observation
		}
	}

	// A lower bound is refusal authority because every byte it counts must
	// occur in the encoded report. It counts exact encoded values plus only the
	// mandatory JSON structure emitted by MarshalIndent, catching both repeated
	// large values and many tiny records without mistaking padding for data.
	lower := projectionBudget{limit: MaxOutputBytes + 1}
	lower.add(minimalReportJSONBytes)
	for _, claim := range declared {
		estimateClaimProjectionLowerBound(&lower, claim, index, inputErr)
		if lower.exceeded || lower.total > MaxOutputBytes {
			return nil, fmt.Errorf("%w: conformance projection requires more than %d bytes; mandatory JSON content is at least %d bytes", ErrCapacityExceeded, MaxOutputBytes, MaxOutputBytes+1)
		}
	}

	// The conservative upper bound remains a cheap success proof. Exceeding it
	// only requests exact measurement; it is never itself a refusal.
	budget := projectionBudget{limit: MaxOutputBytes}
	budget.add(4096) // report envelope, summary, indentation, and punctuation
	budget.addString(report.Snapshot)
	for _, claim := range declared {
		estimateClaimProjection(&budget, claim, index, inputErr)
		if budget.exceeded {
			break
		}
	}
	measureExactly := budget.exceeded
	report.Results = make([]Result, 0, len(declared))

	for _, claim := range declared {
		result := evaluateClaim(claim, index, inputErr)
		report.Results = append(report.Results, result)
		addSummary(&report.Summary, result)
	}
	if measureExactly {
		if _, err := EncodeJSON(report); err != nil {
			return nil, err
		}
	}
	return report, nil
}

// minimalReportJSONBytes is the exact byte length (including the trailing
// newline) of an empty report with the mandatory summary fields. Non-zero
// counts and populated results can only add bytes.
const minimalReportJSONBytes = uint64(len(`{
  "format_version": 1,
  "summary": {
    "declared": 0,
    "declared_none": 0,
    "checks": 0,
    "matched": 0,
    "owed": 0,
    "mismatch": 0,
    "uncheckable": 0,
    "implementation_ready": 0,
    "implementation_blocked": 0
  },
  "results": []
}
`))

const (
	// Result objects are elements at indentation depth two: five bytes place the
	// object on its line, fields are indented six spaces, and the close is
	// indented four. Values counted separately are left blank in the skeleton.
	resultBaseStructureBytes   = uint64(len(`{"claim_id":,"mode":,"implementation_ready":true}`) + 5 + 3*(1+6) + 3 + (1 + 4))
	resultChecksStructureBytes = uint64(len(`,"checks":[]`) + (1 + 6) + 1 + (1 + 6))
	resultReasonStructureBytes = uint64(len(`,"reason":`) + (1 + 6) + 1)

	// Check objects are elements at indentation depth four: nine bytes place the
	// object, fields are indented ten spaces, and the close is indented eight.
	checkBaseStructureBytes     = uint64(len(`{"id":,"adapter":,"target":,"shape":,"state":,"implementation_ready":true}`) + 9 + 6*(1+10) + 6 + (1 + 8))
	checkExpectedStructureBytes = uint64(len(`,"expected":`) + (1 + 10) + 1)
)

// projectionBudget is a saturating, allocation-free counter shared by the
// exact-string lower bound and conservative upper-bound diagnostic. The upper
// bound's string charge assumes every input byte needs a six-byte JSON escape;
// its padding can prove that output fits but is never refusal authority.
type projectionBudget struct {
	limit    uint64
	total    uint64
	exceeded bool
}

func (b *projectionBudget) add(n uint64) {
	if b.exceeded || n > b.limit || b.total > b.limit-n {
		b.exceeded = true
		b.total = b.limit
		return
	}
	b.total += n
}

func (b *projectionBudget) addString(value string) {
	n := uint64(len(value))
	if n > (b.limit-2)/6 {
		b.exceeded = true
		b.total = b.limit
		return
	}
	b.add(2 + 6*n)
}

func (b *projectionBudget) addExactJSONString(value string) {
	b.add(exactJSONStringBytes(value))
}

// exactJSONStringBytes matches encoding/json's string escaping, including its
// HTML-safe escaping and invalid UTF-8 replacement. It includes both quotes.
func exactJSONStringBytes(value string) uint64 {
	n := uint64(2)
	for i := 0; i < len(value); {
		c := value[i]
		if c < utf8.RuneSelf {
			switch c {
			case '\\', '"', '\b', '\f', '\n', '\r', '\t':
				n += 2
			case '<', '>', '&':
				n += 6
			default:
				if c < 0x20 {
					n += 6
				} else {
					n++
				}
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(value[i:])
		if r == utf8.RuneError && size == 1 {
			n += 6
			i++
			continue
		}
		if r == '\u2028' || r == '\u2029' {
			n += 6
		} else {
			n += uint64(size)
		}
		i += size
	}
	return n
}

func estimateClaimProjectionLowerBound(budget *projectionBudget, claim model.Claim, observations map[observationKey]Observation, inputErr error) {
	e := claim.Embodiment
	budget.add(resultBaseStructureBytes)
	budget.addExactJSONString(claim.ID)
	if e == nil {
		return
	}
	budget.addExactJSONString(string(e.Mode))
	if e.Mode == model.EmbodimentModeNone {
		budget.add(resultReasonStructureBytes)
		budget.addExactJSONString(e.Reason)
		return
	}
	if len(e.Checks) > 0 {
		budget.add(resultChecksStructureBytes)
	}
	for _, check := range e.Checks {
		budget.add(checkBaseStructureBytes)
		for _, value := range []string{check.ID, check.Adapter, check.Target} {
			budget.addExactJSONString(value)
		}
		// State is always a non-empty JSON string in a projected check. Counting
		// only its quotes keeps the bound valid without duplicating evaluation.
		budget.add(2)
		if check.Expectation != nil {
			budget.addExactJSONString(string(check.Expectation.Shape))
			budget.add(checkExpectedStructureBytes)
			addProjectionValueLowerBound(budget, check.Expectation.Shape, check.Expectation.Value)
		}
		if inputErr != nil || budget.exceeded {
			continue
		}
		observation, ok := observations[observationKey{check.Adapter, check.Target}]
		if !ok {
			continue
		}
		if observation.Error != nil {
			budget.addExactJSONString(observation.Error.Code)
			budget.addExactJSONString(observation.Error.Message)
			continue
		}
		if check.Expectation != nil && observation.Shape == check.Expectation.Shape {
			addProjectionValueLowerBound(budget, observation.Shape, observation.Value)
		}
	}
}

func addProjectionValueLowerBound(budget *projectionBudget, shape model.ExpectationShape, value any) {
	switch shape {
	case model.ExpectationShapeSet:
		if members, ok := value.([]string); ok {
			budget.add(2) // array brackets; member commas are deliberately omitted
			for _, member := range members {
				budget.addExactJSONString(member)
				if budget.exceeded {
					return
				}
			}
		}
	case model.ExpectationShapeScalar:
		if scalar, ok := value.(string); ok {
			budget.addExactJSONString(scalar)
		}
	}
}

func (b *projectionBudget) addRepeatedMember(value string) {
	// Two quoted copies plus generous delimiter/indentation allowance.
	b.addString(value)
	b.addString(value)
	b.add(128)
}

func estimateClaimProjection(budget *projectionBudget, claim model.Claim, observations map[observationKey]Observation, inputErr error) {
	e := claim.Embodiment
	budget.add(2048)
	budget.addString(claim.ID)
	if e == nil {
		return
	}
	for _, value := range []string{string(e.Mode), e.Reason} {
		budget.addString(value)
		if budget.exceeded {
			return
		}
	}
	if e.Mode != model.EmbodimentModeCompare {
		return
	}
	for _, check := range e.Checks {
		budget.add(1024)
		for _, value := range []string{check.ID, check.Adapter, check.Target} {
			budget.addString(value)
		}
		if check.Expectation != nil {
			budget.addString(string(check.Expectation.Shape))
			addProjectionValue(budget, check.Expectation.Shape, check.Expectation.Value)
		}
		if budget.exceeded {
			return
		}
		if inputErr != nil {
			continue
		}
		observation, ok := observations[observationKey{check.Adapter, check.Target}]
		if !ok {
			continue
		}
		if observation.Error != nil {
			budget.addString(observation.Error.Code)
			budget.addString(observation.Error.Message)
			continue
		}
		budget.addString(string(observation.Shape))
		addProjectionValue(budget, observation.Shape, observation.Value)
	}
}

func addProjectionValue(budget *projectionBudget, shape model.ExpectationShape, value any) {
	switch shape {
	case model.ExpectationShapeSet:
		if members, ok := value.([]string); ok {
			for _, member := range members {
				budget.addRepeatedMember(member)
				if budget.exceeded {
					return
				}
			}
		}
	case model.ExpectationShapeScalar:
		if scalar, ok := value.(string); ok {
			// Called once for expected and once for observed; scalars have no
			// duplicated missing/extra projection.
			budget.addString(scalar)
		}
	}
}

type observationKey struct {
	adapter string
	target  string
}

func evaluateClaim(claim model.Claim, observations map[observationKey]Observation, inputErr error) Result {
	e := claim.Embodiment
	result := Result{ClaimID: claim.ID, Mode: e.Mode}
	if e.Mode == model.EmbodimentModeNone {
		result.ImplementationReady = true
		result.Reason = e.Reason
		return result
	}

	checks := append([]model.EmbodimentCheck(nil), e.Checks...)
	sort.Slice(checks, func(i, j int) bool { return checks[i].ID < checks[j].ID })
	result.Checks = make([]CheckResult, 0, len(checks))
	result.ImplementationReady = true
	for _, check := range checks {
		checkResult := evaluateCheck(check, observations, inputErr)
		result.Checks = append(result.Checks, checkResult)
		if !checkResult.ImplementationReady {
			result.ImplementationReady = false
		}
	}
	return result
}

func evaluateCheck(check model.EmbodimentCheck, observations map[observationKey]Observation, inputErr error) CheckResult {
	result := CheckResult{ID: check.ID, Adapter: check.Adapter, Target: check.Target}
	if check.Expectation != nil {
		result.Shape = check.Expectation.Shape
		result.Expected = canonicalValue(check.Expectation.Shape, check.Expectation.Value)
	}
	if inputErr != nil {
		result.State = StateUncheckable
		result.Reason = inputErr.Error()
		return result
	}

	observation, ok := observations[observationKey{check.Adapter, check.Target}]
	if !ok {
		result.State = StateOwed
		result.Reason = "no observation for the declared adapter and target"
		return result
	}
	if observation.Error != nil {
		result.State = StateUncheckable
		observationError := *observation.Error
		result.ObservationError = &observationError
		result.Reason = "adapter reported an observation error"
		return result
	}
	if check.Expectation == nil || observation.Shape != check.Expectation.Shape {
		result.State = StateUncheckable
		result.Reason = fmt.Sprintf("observation shape %q does not match expectation shape %q", observation.Shape, result.Shape)
		return result
	}

	result.Observed = observation.Value
	switch result.Shape {
	case model.ExpectationShapeSet:
		expected, expectedOK := result.Expected.([]string)
		observed, observedOK := result.Observed.([]string)
		if !expectedOK || !observedOK {
			result.State = StateUncheckable
			result.Reason = "observation value does not match its declared shape"
			return result
		}
		missingCount, extraCount := measureSetDifference(expected, observed)
		result.Missing, result.Extra = materializeSetDifference(expected, observed, missingCount, extraCount)
		if len(result.Missing) == 0 && len(result.Extra) == 0 {
			result.State = StateMatched
			result.ImplementationReady = true
			return result
		}
		result.State = StateMismatch
		result.Reason = "expected and observed sets differ"
		return result
	case model.ExpectationShapeScalar:
		expected, expectedOK := result.Expected.(string)
		observed, observedOK := result.Observed.(string)
		if !expectedOK || !observedOK {
			result.State = StateUncheckable
			result.Reason = "observation value does not match its declared shape"
			return result
		}
		if expected == observed {
			result.State = StateMatched
			result.ImplementationReady = true
			return result
		}
		result.State = StateMismatch
		result.Reason = "expected and observed scalars differ"
		return result
	default:
		result.State = StateUncheckable
		result.Reason = "expectation shape is unsupported"
		return result
	}
}

func addSummary(summary *Summary, result Result) {
	summary.Declared++
	if result.ImplementationReady {
		summary.Ready++
	} else {
		summary.Blocked++
	}
	if result.Mode == model.EmbodimentModeNone {
		summary.DeclaredNone++
		return
	}
	for _, check := range result.Checks {
		summary.Checks++
		switch check.State {
		case StateMatched:
			summary.Matched++
		case StateOwed:
			summary.Owed++
		case StateMismatch:
			summary.Mismatch++
		case StateUncheckable:
			summary.Uncheckable++
		}
	}
}

// DecodeEnvelope strictly decodes and validates one v1 observation envelope.
func DecodeEnvelope(raw []byte) (*Envelope, error) {
	if len(raw) > MaxInputBytes {
		return nil, fmt.Errorf("observation input exceeds maximum of %d bytes", MaxInputBytes)
	}
	if err := rejectDuplicateJSONKeys(raw); err != nil {
		return nil, fmt.Errorf("decode observation input: %w", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, fmt.Errorf("decode observation input: %w", err)
	}
	for _, required := range []string{"format_version", "observations"} {
		value, ok := top[required]
		if !ok || string(value) == "null" {
			return nil, fmt.Errorf("decode observation input: %s is required and cannot be null", required)
		}
	}
	if snapshot, ok := top["snapshot"]; ok {
		if string(snapshot) == "null" {
			return nil, errors.New("decode observation input: snapshot must be a string and cannot be null")
		}
		var value string
		if err := json.Unmarshal(snapshot, &value); err != nil {
			return nil, fmt.Errorf("decode observation input: snapshot must be a string: %w", err)
		}
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var envelope Envelope
	if err := dec.Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode observation input: %w", err)
	}
	if err := requireJSONEOF(dec); err != nil {
		return nil, fmt.Errorf("decode observation input: %w", err)
	}
	if envelope.FormatVersion != FormatVersion {
		return nil, fmt.Errorf("observation format_version %d is unsupported (v1 requires %d)", envelope.FormatVersion, FormatVersion)
	}
	seen := make(map[observationKey]bool, len(envelope.Observations))
	for i := range envelope.Observations {
		observation := &envelope.Observations[i]
		if strings.TrimSpace(observation.Adapter) == "" {
			return nil, fmt.Errorf("observations[%d].adapter must be non-empty", i)
		}
		if strings.TrimSpace(observation.Target) == "" {
			return nil, fmt.Errorf("observations[%d].target must be non-empty", i)
		}
		if observation.errorPresent {
			if observation.Error == nil {
				return nil, fmt.Errorf("observations[%d].error must be an object", i)
			}
			if observation.shapePresent || observation.valuePresent {
				return nil, fmt.Errorf("observations[%d] error arm cannot carry shape or value", i)
			}
			if strings.TrimSpace(observation.Error.Code) == "" {
				return nil, fmt.Errorf("observations[%d].error.code must be non-empty", i)
			}
			if strings.TrimSpace(observation.Error.Message) == "" {
				return nil, fmt.Errorf("observations[%d].error.message must be non-empty", i)
			}
		} else {
			if !observation.shapePresent || !observation.valuePresent {
				return nil, fmt.Errorf("observations[%d] value arm requires shape and value", i)
			}
			switch observation.Shape {
			case model.ExpectationShapeSet:
				values, ok := observation.Value.([]string)
				if !ok {
					return nil, fmt.Errorf("observations[%d].value must be an array of strings for shape %q", i, observation.Shape)
				}
				if err := validateMembers(values, fmt.Sprintf("observations[%d].value", i)); err != nil {
					return nil, err
				}
				sort.Strings(values)
				observation.Value = values
			case model.ExpectationShapeScalar:
				value, ok := observation.Value.(string)
				if !ok {
					return nil, fmt.Errorf("observations[%d].value must be a string for shape %q", i, observation.Shape)
				}
				if strings.TrimSpace(value) == "" {
					return nil, fmt.Errorf("observations[%d].value must be non-empty", i)
				}
			default:
				return nil, fmt.Errorf("observations[%d].shape %q is unsupported (v1 supports %q or %q)", i, observation.Shape, model.ExpectationShapeSet, model.ExpectationShapeScalar)
			}
		}
		key := observationKey{observation.Adapter, observation.Target}
		if seen[key] {
			return nil, fmt.Errorf("observations contains duplicate adapter/target pair %q / %q", observation.Adapter, observation.Target)
		}
		seen[key] = true
	}
	return &envelope, nil
}

func validateMembers(values []string, field string) error {
	if len(values) == 0 {
		return fmt.Errorf("%s must contain at least one member", field)
	}
	seen := make(map[string]bool, len(values))
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s[%d] must be non-empty", field, i)
		}
		if seen[value] {
			return fmt.Errorf("%s contains duplicate member %q", field, value)
		}
		seen[value] = true
	}
	return nil
}

func canonicalMembers(values []string) []string {
	if sort.StringsAreSorted(values) {
		return values
	}
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

func canonicalValue(shape model.ExpectationShape, value any) any {
	if shape != model.ExpectationShapeSet {
		return value
	}
	values, ok := value.([]string)
	if !ok {
		return value
	}
	return canonicalMembers(values)
}

func measureSetDifference(expected, observed []string) (missingCount, extraCount int) {
	expectedSet := make(map[string]bool, len(expected))
	observedSet := make(map[string]bool, len(observed))
	for _, value := range expected {
		expectedSet[value] = true
	}
	for _, value := range observed {
		observedSet[value] = true
	}
	for _, value := range expected {
		if !observedSet[value] {
			missingCount++
		}
	}
	for _, value := range observed {
		if !expectedSet[value] {
			extraCount++
		}
	}
	return missingCount, extraCount
}

func materializeSetDifference(expected, observed []string, missingCount, extraCount int) (missing, extra []string) {
	expectedSet := make(map[string]bool, len(expected))
	observedSet := make(map[string]bool, len(observed))
	for _, value := range expected {
		expectedSet[value] = true
	}
	for _, value := range observed {
		observedSet[value] = true
	}
	missing = make([]string, 0, missingCount)
	extra = make([]string, 0, extraCount)
	for _, value := range expected {
		if !observedSet[value] {
			missing = append(missing, value)
		}
	}
	for _, value := range observed {
		if !expectedSet[value] {
			extra = append(extra, value)
		}
	}
	return missing, extra
}

// EncodeJSON serializes report deterministically and applies the v1 artifact
// bound without touching the destination.
func EncodeJSON(report *Report) ([]byte, error) {
	if report == nil {
		return nil, nil
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("conformance: marshal: %w", err)
	}
	data = append(data, '\n')
	if len(data) > MaxOutputBytes {
		return nil, fmt.Errorf("%w: status output is %d bytes; maximum is %d", ErrCapacityExceeded, len(data), MaxOutputBytes)
	}
	return data, nil
}

// WriteEncoded atomically replaces one preflighted status artifact.
func WriteEncoded(path string, data []byte) error {
	if err := atomicfile.Write(path, data, 0o644); err != nil {
		return fmt.Errorf("conformance: write %q: %w", path, err)
	}
	return nil
}

// WriteJSON writes report deterministically. A nil report is a no-op.
func WriteJSON(report *Report, path string) error {
	data, err := EncodeJSON(report)
	if err != nil || report == nil {
		return err
	}
	return WriteEncoded(path, data)
}

// ReadFile consumes at most MaxInputBytes+1 bytes so a hostile or accidental
// oversized adapter output cannot be fully allocated before the cap is checked.
func ReadFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("observation input is not a regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, MaxInputBytes+1))
}

// ReadFileOutside reads one observation only after resolving existing symlink
// components in both paths and proving the input is physically outside the
// generated build directory. A missing input remains an ordinary read error,
// which Evaluate reports as uncheckable under the non-blocking v1 contract.
func ReadFileOutside(path, excludedDir string) ([]byte, error) {
	if err := CheckPathOutside(path, excludedDir); err != nil {
		return nil, err
	}
	return ReadFile(path)
}

// CheckPathOutside resolves existing symlink components in both paths and
// proves that path is physically outside excludedDir. It reads no file
// content, so index-backed callers can enforce the same containment boundary
// as ReadFileOutside without substituting worktree bytes for staged bytes.
func CheckPathOutside(path, excludedDir string) error {
	resolvedPath, err := resolvePhysicalPath(path)
	if err != nil {
		return err
	}
	resolvedExcluded, err := resolvePhysicalPath(excludedDir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(resolvedExcluded, resolvedPath)
	if err != nil {
		return err
	}
	if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
		return errors.New("observation input resolves inside the generated build directory")
	}
	return nil
}

// resolvePhysicalPath resolves symlinks in the longest existing prefix and
// then reattaches any missing suffix. This lets a missing observation remain a
// stable uncheckable read while still detecting a symlinked parent directory.
func resolvePhysicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	abs = filepath.Clean(abs)
	probe := abs
	var suffix []string
	for {
		if _, err := os.Lstat(probe); err == nil {
			resolved, err := filepath.EvalSymlinks(probe)
			if err != nil {
				return "", err
			}
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return abs, nil
		}
		suffix = append(suffix, filepath.Base(probe))
		probe = parent
	}
}

func requireJSONEOF(dec *json.Decoder) error {
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("more than one JSON value")
		}
		return err
	}
	return nil
}

func rejectDuplicateJSONKeys(raw []byte) error {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := scanJSONValue(dec); err != nil {
		return err
	}
	return requireJSONEOF(dec)
}

func scanJSONValue(dec *json.Decoder) error {
	token, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]bool{}
		for dec.More() {
			keyToken, err := dec.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("object key is not a string")
			}
			if seen[key] {
				return fmt.Errorf("duplicate object key %q", key)
			}
			seen[key] = true
			if err := scanJSONValue(dec); err != nil {
				return err
			}
		}
		end, err := dec.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return errors.New("unterminated object")
		}
	case '[':
		for dec.More() {
			if err := scanJSONValue(dec); err != nil {
				return err
			}
		}
		end, err := dec.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return errors.New("unterminated array")
		}
	default:
		return fmt.Errorf("unexpected delimiter %q", delim)
	}
	return nil
}
