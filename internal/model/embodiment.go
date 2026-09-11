package model

import (
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type EmbodimentMode string

const (
	EmbodimentModeCompare EmbodimentMode = "compare"
	EmbodimentModeNone    EmbodimentMode = "none"
)

type ExpectationShape string

const (
	ExpectationShapeSet    ExpectationShape = "set"
	ExpectationShapeScalar ExpectationShape = "scalar"
)

// EmbodimentExpectation carries []string for set and string for scalar.
type EmbodimentExpectation struct {
	Shape ExpectationShape `yaml:"shape"`
	Value any              `yaml:"value"`
}

// EmbodimentCheck is one independently addressed comparison. ID is stable
// authored identity; Adapter and Target are opaque observation addresses.
type EmbodimentCheck struct {
	ID          string                 `yaml:"id"`
	Adapter     string                 `yaml:"adapter"`
	Target      string                 `yaml:"target"`
	Expectation *EmbodimentExpectation `yaml:"expectation"`
}

// Embodiment is one optional implementation declaration carried by a claim.
type Embodiment struct {
	Mode   EmbodimentMode    `yaml:"mode"`
	Checks []EmbodimentCheck `yaml:"checks,omitempty"`
	Reason string            `yaml:"reason,omitempty"`

	checksPresent bool
	reasonPresent bool
}

func (e *Embodiment) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("embodiment must be a mapping")
	}
	*e = Embodiment{}
	seen := map[string]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if seen[key.Value] {
			return fmt.Errorf("embodiment contains duplicate field %q", key.Value)
		}
		seen[key.Value] = true
		switch key.Value {
		case "mode":
			decoded, err := strictYAMLString(value, "embodiment.mode")
			if err != nil {
				return err
			}
			e.Mode = EmbodimentMode(decoded)
		case "checks":
			e.checksPresent = true
			if value.Kind != yaml.SequenceNode || value.Tag != "!!seq" {
				return fmt.Errorf("embodiment.checks must be a sequence")
			}
			if err := value.Decode(&e.Checks); err != nil {
				return fmt.Errorf("embodiment.checks: %w", err)
			}
		case "reason":
			e.reasonPresent = true
			decoded, err := strictYAMLString(value, "embodiment.reason")
			if err != nil {
				return err
			}
			e.Reason = decoded
		default:
			return fmt.Errorf("field %s not found in type model.Embodiment", key.Value)
		}
	}
	return nil
}

func (c *EmbodimentCheck) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("embodiment check must be a mapping")
	}
	*c = EmbodimentCheck{}
	seen := map[string]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if seen[key.Value] {
			return fmt.Errorf("embodiment check contains duplicate field %q", key.Value)
		}
		seen[key.Value] = true
		switch key.Value {
		case "id":
			decoded, err := strictYAMLString(value, "check.id")
			if err != nil {
				return err
			}
			c.ID = decoded
		case "adapter":
			decoded, err := strictYAMLString(value, "check.adapter")
			if err != nil {
				return err
			}
			c.Adapter = decoded
		case "target":
			decoded, err := strictYAMLString(value, "check.target")
			if err != nil {
				return err
			}
			c.Target = decoded
		case "expectation":
			if value.Tag != "!!null" {
				var expectation EmbodimentExpectation
				if err := value.Decode(&expectation); err != nil {
					return fmt.Errorf("check.expectation: %w", err)
				}
				c.Expectation = &expectation
			}
		default:
			return fmt.Errorf("field %s not found in type model.EmbodimentCheck", key.Value)
		}
	}
	return nil
}

func (e *EmbodimentExpectation) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("expectation must be a mapping")
	}
	*e = EmbodimentExpectation{}
	seen := map[string]*yaml.Node{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if seen[key.Value] != nil {
			return fmt.Errorf("expectation contains duplicate field %q", key.Value)
		}
		if key.Value != "shape" && key.Value != "value" {
			return fmt.Errorf("field %s not found in type model.EmbodimentExpectation", key.Value)
		}
		seen[key.Value] = value
	}
	if shape := seen["shape"]; shape != nil {
		decoded, err := strictYAMLString(shape, "expectation.shape")
		if err != nil {
			return err
		}
		e.Shape = ExpectationShape(decoded)
	}
	if value := seen["value"]; value != nil {
		switch e.Shape {
		case ExpectationShapeSet:
			decoded, err := strictYAMLStringSequence(value, "expectation.value")
			if err != nil {
				return err
			}
			e.Value = decoded
		case ExpectationShapeScalar:
			decoded, err := strictYAMLString(value, "expectation.value")
			if err != nil {
				return err
			}
			e.Value = decoded
		default:
			e.Value = nil // validation reports the unsupported shape
		}
	}
	return nil
}

func strictYAMLString(node *yaml.Node, field string) (string, error) {
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return "", fmt.Errorf("%s must be a string", field)
	}
	return node.Value, nil
}

func strictYAMLStringSequence(node *yaml.Node, field string) ([]string, error) {
	if node.Kind != yaml.SequenceNode || node.Tag != "!!seq" {
		return nil, fmt.Errorf("%s must be a sequence of strings", field)
	}
	values := make([]string, len(node.Content))
	for i, member := range node.Content {
		value, err := strictYAMLString(member, fmt.Sprintf("%s[%d]", field, i))
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	return values, nil
}

// ValidateEmbodiment checks and canonicalizes c's optional declaration.
func ValidateEmbodiment(c *Claim) error {
	if c == nil || c.Embodiment == nil {
		return nil
	}
	e := c.Embodiment
	switch e.Mode {
	case EmbodimentModeCompare:
		if e.reasonPresent || e.Reason != "" {
			return fmt.Errorf("embodiment.reason must be absent for mode %q", e.Mode)
		}
		if len(e.Checks) == 0 {
			return fmt.Errorf("embodiment.checks must contain at least one check for mode %q", e.Mode)
		}
		ids := make(map[string]bool, len(e.Checks))
		addresses := make(map[[2]string]bool, len(e.Checks))
		for i := range e.Checks {
			check := &e.Checks[i]
			prefix := fmt.Sprintf("embodiment.checks[%d]", i)
			if strings.TrimSpace(check.ID) == "" {
				return fmt.Errorf("%s.id must be non-empty", prefix)
			}
			if ids[check.ID] {
				return fmt.Errorf("embodiment.checks contains duplicate id %q", check.ID)
			}
			ids[check.ID] = true
			if strings.TrimSpace(check.Adapter) == "" {
				return fmt.Errorf("%s.adapter must be non-empty", prefix)
			}
			if strings.TrimSpace(check.Target) == "" {
				return fmt.Errorf("%s.target must be non-empty", prefix)
			}
			address := [2]string{check.Adapter, check.Target}
			if addresses[address] {
				return fmt.Errorf("embodiment.checks contains duplicate adapter/target pair %q / %q", check.Adapter, check.Target)
			}
			addresses[address] = true
			if check.Expectation == nil {
				return fmt.Errorf("%s.expectation is required", prefix)
			}
			if err := validateExpectation(check.Expectation, prefix+".expectation"); err != nil {
				return err
			}
		}
		sort.Slice(e.Checks, func(i, j int) bool { return e.Checks[i].ID < e.Checks[j].ID })
	case EmbodimentModeNone:
		if strings.TrimSpace(e.Reason) == "" {
			return fmt.Errorf("embodiment.reason must be non-empty for mode %q", e.Mode)
		}
		if e.checksPresent || len(e.Checks) != 0 {
			return fmt.Errorf("embodiment mode %q cannot carry checks", e.Mode)
		}
	default:
		return fmt.Errorf("embodiment.mode %q is unsupported (v1 supports %q or %q)", e.Mode, EmbodimentModeCompare, EmbodimentModeNone)
	}
	return nil
}

func validateExpectation(expectation *EmbodimentExpectation, field string) error {
	switch expectation.Shape {
	case ExpectationShapeSet:
		values, ok := expectation.Value.([]string)
		if !ok {
			return fmt.Errorf("%s.value must be a sequence of strings for shape %q", field, expectation.Shape)
		}
		if err := validateSetMembers(values, field+".value"); err != nil {
			return err
		}
		sort.Strings(values)
		expectation.Value = values
	case ExpectationShapeScalar:
		value, ok := expectation.Value.(string)
		if !ok {
			return fmt.Errorf("%s.value must be a string for shape %q", field, expectation.Shape)
		}
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s.value must be non-empty", field)
		}
	default:
		return fmt.Errorf("%s.shape %q is unsupported (v1 supports %q or %q)", field, expectation.Shape, ExpectationShapeSet, ExpectationShapeScalar)
	}
	return nil
}

func validateSetMembers(values []string, field string) error {
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
