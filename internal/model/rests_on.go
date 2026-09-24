package model

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ScopeProject marks a claim that lives in the project-claims store, not
// under a module. Module claims leave Scope empty. See IsProjectClaimID for
// the id grammar that goes with it.
const ScopeProject = "project"

// RestsOn is a claim's required dependency declaration (NIT-24). On disk it
// is ONE of two shapes and nothing else:
//
//	rests_on:                      # a list of claim ids
//	  - widget.contract.overview
//	  - project.retention
//
//	rests_on:                      # the stated absence
//	  none: true
//	  reason: first fact in the module; nothing to rest on yet
//
// Targets are claim ids only — project.<slug>, any module's *.contract.*, or
// this module's own *.internals.* (rests-on-target refuses a foreign
// module's internals). The constitution is never a target: it is the brief
// every claim already builds toward, so there is no ref grammar for it and
// nothing here knows it exists. A claim with neither shape is refused by
// rests-on-required; a claim with both is refused by the same rule.
//
// The zero value is "not declared". It is what a claim file without the key
// decodes to, and it marshals back to an absent key (see IsZero), so the
// loader's round-trip guard holds.
type RestsOn struct {
	// None is the stated absence: this claim deliberately rests on nothing.
	None bool
	// Reason is the author's explanation, required when None is true.
	Reason string
	// IDs are the claim ids this claim rests on, in authored order.
	IDs []string
}

// RestsOnIDs builds a target list.
func RestsOnIDs(ids ...string) RestsOn {
	out := make([]string, len(ids))
	copy(out, ids)
	return RestsOn{IDs: out}
}

// RestsNone builds RESTS ON NONE with the author's reason.
func RestsNone(reason string) RestsOn {
	return RestsOn{None: true, Reason: reason}
}

// AppendIDs adds targets. A stated NONE is cleared: a claim that now rests
// on something no longer rests on nothing, and its reason goes with it.
func (r *RestsOn) AppendIDs(ids ...string) {
	if r == nil {
		return
	}
	if r.None {
		r.None = false
		r.Reason = ""
	}
	r.IDs = append(r.IDs, ids...)
}

// Empty reports an undeclared rests_on: neither NONE nor any target. It is
// the rests-on-required lint's question, and it is deliberately not IsZero —
// a `none: true` with a blank reason is not empty, it is malformed, and the
// same lint reports that separately.
func (r RestsOn) Empty() bool {
	return !r.None && len(r.IDs) == 0
}

// IsZero is yaml.v3's omitempty hook: an undeclared rests_on writes no key.
func (r RestsOn) IsZero() bool {
	return !r.None && r.Reason == "" && len(r.IDs) == 0
}

// restsNoneYAML is the on-disk mapping form, as a struct so the two keys
// come out in a fixed order rather than a map's sorted one.
type restsNoneYAML struct {
	None   bool   `yaml:"none"`
	Reason string `yaml:"reason"`
}

// UnmarshalYAML accepts a sequence of claim-id strings or the {none, reason}
// mapping. A sequence of mappings is refused: there is no `{id: ...}` item
// form, and accepting one would be a second, undocumented grammar.
func (r *RestsOn) UnmarshalYAML(value *yaml.Node) error {
	if value == nil || (value.Kind == yaml.ScalarNode && (value.Tag == "!!null" || value.Value == "")) {
		*r = RestsOn{}
		return nil
	}
	switch value.Kind {
	case yaml.MappingNode:
		var raw restsNoneYAML
		if err := value.Decode(&raw); err != nil {
			return fmt.Errorf("rests_on: %w", err)
		}
		if !raw.None {
			return fmt.Errorf("rests_on: a mapping must be {none: true, reason: ...}; targets are a list of claim ids")
		}
		*r = RestsOn{None: true, Reason: raw.Reason}
		return nil
	case yaml.SequenceNode:
		ids := make([]string, 0, len(value.Content))
		for i, item := range value.Content {
			if item.Kind != yaml.ScalarNode {
				return fmt.Errorf("rests_on[%d]: expected a claim id string", i)
			}
			if strings.TrimSpace(item.Value) == "" {
				return fmt.Errorf("rests_on[%d]: a claim id is required", i)
			}
			ids = append(ids, item.Value)
		}
		*r = RestsOn{IDs: ids}
		return nil
	default:
		return fmt.Errorf("rests_on: expected a list of claim ids or {none: true, reason: ...}")
	}
}

// MarshalYAML writes NONE as the mapping and targets as a plain list.
func (r RestsOn) MarshalYAML() (interface{}, error) {
	if r.None {
		return restsNoneYAML{None: true, Reason: r.Reason}, nil
	}
	if len(r.IDs) == 0 {
		return nil, nil
	}
	out := make([]string, len(r.IDs))
	copy(out, r.IDs)
	return out, nil
}

// IsProjectClaimID reports the project-claim id grammar: project.<slug>,
// exactly two segments, with "project" reserved as the first one.
func IsProjectClaimID(id string) bool {
	module, slug, ok := strings.Cut(id, ".")
	if !ok || module != ScopeProject || slug == "" || strings.Contains(slug, ".") {
		return false
	}
	return true
}
