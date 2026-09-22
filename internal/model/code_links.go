package model

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// LinksMode is the claim-level code-link declaration. v1 only names the
// deliberate absence: a behavior (or other code-producing) claim that has
// no implementing declaration, shown on the card and excluded from the
// code-link gate. Omitting `links` is different — that claim is still
// expected to be tagged once `source_dirs` is set.
type LinksMode string

const (
	LinksModeNone LinksMode = "none"
)

// StepOwnerProcess is the only owner `steps_owned_by` accepts in v1: a
// verification step discharged by a person or a review, not by a
// `dossierx-step` tag on dummy code.
const StepOwnerProcess = "process"

// ClaimLinks is the optional claim-level counterpart of Embodiment for
// code links: `mode: none` plus a reason, parallel to `embodiment.mode: none`.
type ClaimLinks struct {
	Mode   LinksMode `yaml:"mode"`
	Reason string    `yaml:"reason,omitempty"`

	reasonPresent bool
}

// StepsOwnedBy maps 1-based `steps:` indexes to an owner. v1 values are
// only "process". A process-owned step is counted as linked and shown on
// the card; it does not need a source tag.
type StepsOwnedBy map[int]string

func (l *ClaimLinks) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("links must be a mapping")
	}
	*l = ClaimLinks{}
	seen := map[string]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if seen[key.Value] {
			return fmt.Errorf("links contains duplicate field %q", key.Value)
		}
		seen[key.Value] = true
		switch key.Value {
		case "mode":
			decoded, err := strictYAMLString(value, "links.mode")
			if err != nil {
				return err
			}
			l.Mode = LinksMode(decoded)
		case "reason":
			l.reasonPresent = true
			decoded, err := strictYAMLString(value, "links.reason")
			if err != nil {
				return err
			}
			l.Reason = decoded
		default:
			return fmt.Errorf("field %s not found in type model.ClaimLinks", key.Value)
		}
	}
	return nil
}

func (s *StepsOwnedBy) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("steps_owned_by must be a mapping")
	}
	out := make(StepsOwnedBy)
	seen := map[int]bool{}
	for i := 0; i < len(node.Content); i += 2 {
		key, value := node.Content[i], node.Content[i+1]
		if key.Kind != yaml.ScalarNode {
			return fmt.Errorf("steps_owned_by keys must be 1-based step indexes")
		}
		n, err := strconv.Atoi(key.Value)
		if err != nil {
			return fmt.Errorf("steps_owned_by keys must be 1-based step indexes, not %q", key.Value)
		}
		if seen[n] {
			return fmt.Errorf("steps_owned_by contains duplicate step %d", n)
		}
		seen[n] = true
		owner, err := strictYAMLString(value, fmt.Sprintf("steps_owned_by.%d", n))
		if err != nil {
			return err
		}
		out[n] = owner
	}
	*s = out
	return nil
}

// LinksNone reports whether c deliberately declares it has no implementing
// declaration. The code-link gate and the viewer's "not linked to code"
// row both key off this.
func (c Claim) LinksNone() bool {
	return c.Links != nil && c.Links.Mode == LinksModeNone
}

// ProcessOwnedSteps returns the 1-based indexes c attests as process-owned,
// sorted. Unknown owners are ignored here; ValidateCodeLinkDeclarations
// already refused them at load.
func (c Claim) ProcessOwnedSteps() []int {
	if len(c.StepsOwnedBy) == 0 {
		return nil
	}
	out := make([]int, 0, len(c.StepsOwnedBy))
	for n, owner := range c.StepsOwnedBy {
		if owner == StepOwnerProcess {
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out
}

// ValidateCodeLinkDeclarations checks and leaves c's optional `links` and
// `steps_owned_by` in a canonical form. Called from the same parse path as
// ValidateEmbodiment.
func ValidateCodeLinkDeclarations(c *Claim) error {
	if c == nil {
		return nil
	}
	if err := validateClaimLinks(c); err != nil {
		return err
	}
	if err := validateStepsOwnedBy(c); err != nil {
		return err
	}
	if c.LinksNone() && len(c.StepsOwnedBy) > 0 {
		return fmt.Errorf("links.mode %q cannot be combined with steps_owned_by", LinksModeNone)
	}
	return nil
}

func validateClaimLinks(c *Claim) error {
	if c.Links == nil {
		return nil
	}
	l := c.Links
	switch l.Mode {
	case LinksModeNone:
		if strings.TrimSpace(l.Reason) == "" {
			return fmt.Errorf("links.reason must be non-empty for mode %q", l.Mode)
		}
	default:
		return fmt.Errorf("links.mode %q is unsupported (v1 supports %q)", l.Mode, LinksModeNone)
	}
	return nil
}

func validateStepsOwnedBy(c *Claim) error {
	if len(c.StepsOwnedBy) == 0 {
		return nil
	}
	if len(c.Steps) == 0 {
		return fmt.Errorf("steps_owned_by requires steps")
	}
	for n, owner := range c.StepsOwnedBy {
		if n < 1 || n > len(c.Steps) {
			return fmt.Errorf("steps_owned_by.%d is out of range (claim has %d steps)", n, len(c.Steps))
		}
		if owner != StepOwnerProcess {
			return fmt.Errorf("steps_owned_by.%d owner %q is unsupported (v1 supports %q)", n, owner, StepOwnerProcess)
		}
	}
	return nil
}
