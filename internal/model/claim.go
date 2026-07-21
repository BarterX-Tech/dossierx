// Package model defines the in-memory representation of a "claim" — the
// atomic unit of documentation this engine renders — and the small set of
// enums/edge types that make up its schema. This file is the canonical
// mapping of the YAML claim schema described in FORMAT.md onto Go
// types; every other package (catalog, lint, render, lock, reaudit) builds
// on these types, so changes here are load-bearing for the whole engine.
package model

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Status is the lifecycle state of a claim.
type Status string

const (
	StatusDraft  Status = "draft"
	StatusLocked Status = "locked"
)

// Layout selects which render component a claim is rendered with. When a
// claim omits layout, internal/catalog is responsible for inferring one of
// these from the claim's shape (rows present -> table; a non-empty Steps ->
// steps; otherwise -> card).
type Layout string

const (
	LayoutCard   Layout = "card"
	LayoutTable  Layout = "table"
	LayoutList   Layout = "list"
	LayoutSteps  Layout = "steps"
	LayoutTree   Layout = "tree"
	LayoutBanner Layout = "banner"

	// LayoutMockup is for claims whose content is a project-authored,
	// review-gated blob of raw HTML (Claim.RawHTML) rather than markdown
	// prose/rows/steps — e.g. an embedded viewer mockup. See RawHTML and
	// RawHTMLReviewed's doc comments for the lock-lifecycle gate this
	// layout implies.
	LayoutMockup Layout = "mockup"
)

// BuildRole classifies a claim by where it sits in a module's build (i.e.
// implementation) order, as distinct from Order/Section, which govern the
// unrelated reading-order the VIEWER presents claims in (see render.go's
// orderClaims). internal/buildorder consumes BuildRole to compute a
// module's implementation sequence once every one of that module's claims
// is locked; see that package's doc comment for the fixed phase sequence
// this drives.
//
// BuildRole is optional (the zero value, "") while a claim is still draft —
// it only becomes required once a claim locks, enforced by the
// "build-role-required-for-locked" lint (internal/lint/build_role_required.go),
// not by this type itself.
type BuildRole string

const (
	// BuildRoleOrientation is context/process claims that are read for
	// background but never themselves acted on during implementation
	// (e.g. "why this module exists", house-style notes). First phase.
	BuildRoleOrientation BuildRole = "orientation"

	// BuildRoleSchema is data-shape claims (types, fields, storage
	// layout) — built first among the "real work" phases, since behavior
	// and api claims describe logic over these shapes.
	BuildRoleSchema BuildRole = "schema"

	// BuildRoleBehavior is workflow/logic claims — the bulk of the real
	// implementation work. Within this phase, claims are ordered by their
	// rests_on edges (a behavior claim resting on another behavior claim
	// is built after it), not left in an arbitrary order.
	BuildRoleBehavior BuildRole = "behavior"

	// BuildRoleAPI is public-function/entry-point claims, built after
	// behavior: an API is a thin, addressable surface over behavior that
	// must already exist for the API to have something to call into.
	BuildRoleAPI BuildRole = "api"

	// BuildRoleVerification is test-checklist/acceptance-criteria claims,
	// read last so a human (or agent) writing tests has every other phase
	// of the module's build already in front of them to write tests
	// against.
	BuildRoleVerification BuildRole = "verification"

	// BuildRoleOutOfScope marks a claim as deferred/future-scope: it is
	// excluded from every module's build order sequence (never placed in
	// a phase), but internal/buildorder still reports it (as Excluded) so
	// it is never silently dropped from view the way a claim the pipeline
	// simply forgot about would be.
	BuildRoleOutOfScope BuildRole = "out-of-scope"
)

// Kind distinguishes a claim that states a fact about the system (the
// default, and everything the engine has ever rendered until this field
// existed) from one that is itself guidance about how to read the docs —
// an "orientation note". This is a different axis from BuildRole: a
// BuildRoleOrientation claim is still a *fact* the module rests on (e.g.
// "why this module exists"), while a KindOrientationNote claim is a
// pointer *at* other claims (e.g. "if you only call the public API, read
// Contract, never Internals"). See internal/lint/orientation_note_shape.go and
// internal/lint/orientation_note_order.go for the rules this field feeds,
// and FORMAT.md for the full authoring contract.
type Kind string

const (
	// KindFact is the default (the zero value maps to it via
	// Claim.EffectiveKind — see that method): a claim stating a fact about
	// the system.
	KindFact Kind = "fact"

	// KindOrientationNote marks a claim as agent/reviewer-facing reading
	// guidance rather than a fact. Every claim under the reserved
	// config.ReservedOverviewFacet facet is a KindOrientationNote whether
	// or not this field is set explicitly — see EffectiveKind.
	KindOrientationNote Kind = "orientation-note"
)

// GovernedType is the kind of doctrine governance backing a claim.
type GovernedType string

const (
	// GovernedNone means the claim is deliberately not backed by any
	// doctrine claim; Reason is required in that case (see Governed.Reason).
	GovernedNone GovernedType = "none"
)

// Governed records why a claim is (or is deliberately not) governed by a
// doctrine claim. Type is either "none" or a doctrine claim id. Reason is
// required by the lint suite whenever Type == GovernedNone.
type Governed struct {
	Type   string `yaml:"type"`
	Reason string `yaml:"reason,omitempty"`
}

// Row is one structured data row under a claim's Rows. It is intentionally
// a generic string-keyed map so claims can carry arbitrary columns; the
// rows-shape lint is responsible for checking that all rows on a claim
// share a consistent set of columns.
//
// Row deliberately stays a plain map[string]any rather than some ordered
// map type: many existing model.Row{"key": val} composite literals across
// the codebase's tests, and text/template's index/range builtins in
// render/components/table.html, depend on Row's reflect.Kind being Map.
// Authored column order is instead recovered via UnmarshalYAML/MarshalYAML
// below, which stash it inside the map itself under rowOrderKey — see that
// constant's doc comment — and RowColumns, which reads it back out. A Row
// built directly in Go (as most existing tests do) simply carries no
// rowOrderKey entry, and callers (RowColumns, render/components.rowKeys)
// treat that as "no captured order", falling back to their prior
// alphabetical behavior — so this is purely additive for any Row that
// didn't come from a YAML decode.
type Row map[string]any

// rowOrderKey is the map entry Row's UnmarshalYAML stashes a row's
// authored column order under (as a []string), alongside that row's real
// column data. It is a key no claim author can produce from an ordinary
// YAML string scalar (a leading NUL byte), so it can never collide with a
// real column name; RowColumns and Row.MarshalYAML both know to treat it
// as engine-private bookkeeping rather than a real column, and
// MarshalYAML never writes it back out to disk.
const rowOrderKey = "\x00order"

// RowColumns returns row's authored column order, as captured by
// UnmarshalYAML at YAML-decode time, or nil if row carries no captured
// order (e.g. it was constructed directly in Go rather than decoded from
// YAML — see Row's doc comment). Callers that need a column order
// regardless (e.g. render/components.rowKeys) are expected to fall back to
// their own default (currently: that row's own keys, alphabetically) when
// RowColumns returns nil.
func RowColumns(row Row) []string {
	v, ok := row[rowOrderKey]
	if !ok {
		return nil
	}
	order, ok := v.([]string)
	if !ok {
		return nil
	}
	return order
}

// UnmarshalYAML decodes a YAML mapping node into r, preserving the
// authored key order (recoverable afterward via RowColumns) alongside the
// usual column data. See rowOrderKey's doc comment for why this is done
// via a reserved map entry rather than changing Row's underlying type.
func (r *Row) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("model: row must be a YAML mapping, got kind %v", value.Kind)
	}
	out := make(Row, len(value.Content)/2+1)
	order := make([]string, 0, len(value.Content)/2)
	for i := 0; i+1 < len(value.Content); i += 2 {
		var key string
		if err := value.Content[i].Decode(&key); err != nil {
			return fmt.Errorf("model: row key: %w", err)
		}
		var v any
		if err := value.Content[i+1].Decode(&v); err != nil {
			return fmt.Errorf("model: row[%q]: %w", key, err)
		}
		out[key] = v
		order = append(order, key)
	}
	out[rowOrderKey] = order
	*r = out
	return nil
}

// MarshalYAML encodes r back to YAML in the authored order captured by
// UnmarshalYAML, when present — so loader.SaveClaim round-trips a table
// claim's column order exactly as a human authored it — and always omits
// the reserved rowOrderKey entry itself, since it is engine-only
// bookkeeping, never on-disk schema. A Row with no captured order (built
// directly in Go, not via YAML decode) marshals as a plain map, same as
// before this ordering mechanism existed (alphabetical, gopkg.in/yaml.v3's
// default map behavior).
func (r Row) MarshalYAML() (interface{}, error) {
	if order := RowColumns(r); order != nil {
		node := &yaml.Node{Kind: yaml.MappingNode}
		for _, k := range order {
			var keyNode, valNode yaml.Node
			if err := keyNode.Encode(k); err != nil {
				return nil, fmt.Errorf("model: encode row key %q: %w", k, err)
			}
			if err := valNode.Encode(r[k]); err != nil {
				return nil, fmt.Errorf("model: encode row[%q]: %w", k, err)
			}
			node.Content = append(node.Content, &keyNode, &valNode)
		}
		return node, nil
	}
	if _, has := r[rowOrderKey]; !has {
		return map[string]any(r), nil
	}
	out := make(map[string]any, len(r))
	for k, v := range r {
		if k != rowOrderKey {
			out[k] = v
		}
	}
	return out, nil
}

// Claim is one atomic YAML fact as described in FORMAT.md. YAML struct
// tags below are the authoritative field names for claim files on disk.
type Claim struct {
	ID     string `yaml:"id"`
	Facet  string `yaml:"facet"`
	Module string `yaml:"module,omitempty"`
	Status Status `yaml:"status"`
	Layout Layout `yaml:"layout,omitempty"`

	// Kind is optional; unset (or explicitly KindFact) means "an ordinary
	// fact claim". See Kind's doc comment. Read via EffectiveKind, not this
	// field directly, everywhere except lint's own explicit-value checks —
	// EffectiveKind also accounts for the reserved overview facet implying
	// orientation-note without the author having to repeat it.
	Kind Kind `yaml:"kind,omitempty"`

	// BuildRole is optional (see BuildRole's doc comment for why, and for
	// each phase value's meaning); internal/buildorder is the only
	// consumer, and only once every claim in a module is locked.
	BuildRole BuildRole `yaml:"build_role,omitempty"`

	Body string `yaml:"body,omitempty"`
	Rows []Row  `yaml:"rows,omitempty"`

	// Section is an optional, human-readable in-content section label a
	// project MAY set (e.g. "5 - workflows / lifecycle") to get section
	// headings rendered in the content area. It is purely optional,
	// free-form data the claim author chooses — the engine does not parse
	// or derive it from anything (in particular, not from directory
	// layout: directory layout is deliberately not part of the claim
	// schema, so this field is the project-agnostic way to get section
	// structure into rendered output instead).
	Section string `yaml:"section,omitempty"`

	// RawHTML is a project-authored blob of raw HTML for layout: mockup
	// claims (see LayoutMockup) — e.g. an embedded viewer mockup that
	// cannot be expressed as markdown prose/rows/steps. Unlike Body, Steps,
	// and Row values, which flow through html/template as plain,
	// auto-escaped strings (see render/components.renderBody and the
	// raw-html-scope lint), RawHTML is meant to be rendered unescaped, so
	// it carries its own explicit review gate: RawHTMLReviewed.
	RawHTML string `yaml:"raw_html,omitempty"`

	// RawHTMLReviewed is the lock-lifecycle gate for RawHTML content: a
	// claim carrying RawHTML is only safe to render unescaped once a human
	// has explicitly reviewed that HTML and set this true. It is
	// deliberately a separate, explicit flag rather than inferred from
	// Status/Layout, mirroring the project's existing "gate is a distinct
	// field, not derived" precedent (see ReviewPending below).
	RawHTMLReviewed bool `yaml:"raw_html_reviewed,omitempty"`

	// Steps is populated for layout: steps claims. Each entry is free-form
	// markdown describing one step; render/components/steps.html renders
	// them in order.
	Steps []string `yaml:"steps,omitempty"`

	// Edges.
	Mirrors  []string `yaml:"mirrors,omitempty"`
	RestsOn  []string `yaml:"rests_on,omitempty"`
	Governed Governed `yaml:"governed_by"`

	MigratedFrom string `yaml:"migrated_from,omitempty"`

	// Order is an optional, author-set hint for the VIEWER's per-group
	// claim sequence (internal/render's orderClaims): claims with Order set
	// sort ascending by it, ahead of everything else. 0/unset means "no
	// explicit order" — such claims keep a stable fallback order instead.
	// This is deliberately unrelated to internal/catalog.Document's
	// alphabetical-by-id claim order, which exists only to make
	// .catalog.json/lint output byte-deterministic and must stay
	// unaffected by this field.
	Order int `yaml:"order,omitempty"`

	// Emphasis marks a claim as carrying outsized weight for its facet (the
	// docs/ source's "hard boundary" cards — border-color:var(--warn) with a
	// matching .k color — are the hand-authored precedent this mirrors). It
	// is deliberately its own field rather than being inferred from Governed:
	// GovernedNone/GovernedType answer "what backs this claim's truth", which
	// is orthogonal to "how loudly should this render" — a governed claim can
	// still be a hard boundary, and an ungoverned-with-reason claim usually
	// isn't one. render/components/card.html uses Emphasis to add the
	// claim-card--warn class.
	Emphasis bool `yaml:"emphasis,omitempty"`

	// ReviewPending is engine-managed: it is only meaningful when
	// Status == StatusLocked, and is flipped to true by internal/lock when
	// a dependency's content hash changes underneath a locked claim. It is
	// cleared only via a confirmed internal/reaudit apply. It is never set
	// on a draft claim.
	ReviewPending bool `yaml:"review_pending,omitempty"`

	// AuditNotes is engine-managed provenance for the reaudit lifecycle: a
	// confirmed internal/reaudit.Apply appends the human-facing Note that
	// accompanied the applied (or no-change) proposal, so a claim carries a
	// durable trail of every reaudit that touched it. It is bookkeeping,
	// not comparable content, so internal/lock.ContentHash deliberately
	// does not include it (same reasoning as ReviewPending).
	AuditNotes []string `yaml:"audit_notes,omitempty"`

	// SourcePath is the filesystem path the claim was loaded from. It is
	// populated by the claim loader (not part of the YAML schema itself)
	// and is used by internal/lock to write edits back to the right file.
	SourcePath string `yaml:"-"`
}

// EffectiveKind returns c's real Kind, accounting for the reserved
// "overview" facet (config.ReservedOverviewFacet) implicitly meaning
// KindOrientationNote even when Kind itself is unset — see Kind's doc
// comment. This package cannot import internal/config (config does not
// depend on model, and importing it here would invert that), so the facet
// name is duplicated as the unexported reservedOverviewFacet constant
// immediately below, which internal/model/claim_test.go and
// internal/config/config.go's own ReservedOverviewFacet constant both
// pin equal via a same-package/cross-package equality test in Task 3.
func (c Claim) EffectiveKind() Kind {
	if c.Facet == reservedOverviewFacet {
		return KindOrientationNote
	}
	if c.Kind == "" {
		return KindFact
	}
	return c.Kind
}

// reservedOverviewFacet mirrors config.ReservedOverviewFacet — see
// EffectiveKind's doc comment for why model can't import config directly.
const reservedOverviewFacet = "overview"
