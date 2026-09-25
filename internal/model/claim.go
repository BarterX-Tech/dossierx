// Package model defines the in-memory representation of a "claim" — the
// atomic unit of documentation this engine renders — and the small set of
// enums/edge types that make up its schema. This file is the canonical
// mapping of the YAML claim schema described in FORMAT.md onto Go
// types; every other package (catalog, lint, render, lock, reaudit) builds
// on these types, so changes here are load-bearing for the whole engine.
package model

import (
	"fmt"
	"strings"
	"unicode/utf8"

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

// Kind is the claim's kind field. The only legal value is KindFact
// (also the default when the field is omitted). kind-shape refuses every
// other string, including the retired orientation-note value.
type Kind string

const (
	// KindFact is the default (the zero value maps to it via
	// Claim.EffectiveKind — see that method): a claim stating a fact about
	// the system.
	KindFact Kind = "fact"
)

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
	Facet  string `yaml:"facet,omitempty"`
	Module string `yaml:"module,omitempty"`
	// Scope is "project" for project-claims store nodes (id project.<slug>).
	// Module claims omit it. Facet and Module are forbidden on project claims.
	Scope  string `yaml:"scope,omitempty"`
	Status Status `yaml:"status"`
	Layout Layout `yaml:"layout,omitempty"`

	// Kind is optional; unset (or explicitly KindFact) means "an ordinary
	// fact claim". Read via EffectiveKind, not this field directly,
	// everywhere except lint's own explicit-value checks.
	Kind Kind `yaml:"kind,omitempty"`

	// Embodiment optionally declares one structured implementation expectation,
	// or deliberately records that this claim has no software embodiment. It is
	// project-neutral: adapter and target are opaque strings interpreted only by
	// the project-owned adapter that emits observations.
	Embodiment *Embodiment `yaml:"embodiment,omitempty"`

	// Summary is the required one-line plain-text description of the claim.
	// check ERRORs when it is missing, multiline, markdown-shaped, or over
	// max_claim_summary_chars. It is part of lock.ContentHash when non-empty.
	Summary string `yaml:"summary,omitempty"`

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

	// RawHTML is a project-authored blob of raw HTML, legal on any layout
	// (not only layout: mockup, see LayoutMockup) — e.g. an embedded viewer
	// mockup that cannot be expressed as markdown prose/rows/steps. Unlike
	// Body, Steps, and Row values, which flow through html/template as
	// plain, auto-escaped strings (see render/components.renderBody and the
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

	// Edges. rests_on is the one claim-to-claim edge: the required
	// dependency chain, and the drift baseline a locked claim is checked
	// against. It is a list of claim ids, or the stated absence
	// {none: true, reason} (NIT-24). The retired governed_by edge (NIT-29)
	// has no field and no shadow key: a claim file that still carries it
	// fails strict decode.
	//
	// Mirrors is not an edge. The field exists only so KnownFields still
	// names the historical YAML key and LockedClaimHash / ContentHash stay
	// byte-identical. Nothing walks it.
	Mirrors []string `yaml:"mirrors,omitempty"`
	RestsOn RestsOn  `yaml:"rests_on,omitempty"`

	// Sources is the evidence this claim rests on, cited from Body by "[n]"
	// markers matching each entry's Ref. See model.Source for the whole
	// rationale; the short version is that MigratedFrom below records WHICH
	// sources a claim came from and this records WHAT they were, which is the
	// difference between a comment and something the engine can check.
	//
	// Optional and additive: a claim without sources serializes byte-for-byte
	// as it did before this field existed (the `omitempty` tag is
	// load-bearing, exactly as it is for Comments), and every source-* lint
	// is a no-op on it.
	Sources []Source `yaml:"sources,omitempty"`

	// Tracks is this claim's membership in cross-cutting concerns — the
	// second axis, orthogonal to Module. See model.TrackRef and
	// model.TrackRole for why membership is not an edge and why the
	// owns/cites pair is what keeps it from being tagging.
	//
	// Optional and additive in the same sense as Sources: a corpus that
	// declares no tracks behaves exactly as it did before this field
	// existed. Module is untouched by it — a claim keeps exactly one module,
	// and track membership never gates locking.
	Tracks []TrackRef `yaml:"tracks,omitempty"`

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
	// is deliberately its own field rather than being inferred from the
	// claim's edges: what a claim rests on is orthogonal to "how loudly
	// should this render". render/components/card.html uses Emphasis to add
	// the claim-card--warn class.
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

	// Comments is engine-managed review discussion attached to this claim —
	// the threaded, Google-Docs-style "comments on claims" feature (see
	// model.Comment and internal/comments). It is bookkeeping about the
	// claim, not comparable content, so internal/lock.ContentHash
	// deliberately excludes it — commenting on a claim never flips its
	// dependents to review_pending — exactly like ReviewPending and
	// AuditNotes above. The `omitempty` tag is load-bearing: a claim that
	// has never been commented on serializes byte-for-byte as it did before
	// this field existed, so adding the feature does not rewrite every
	// existing claim file. An unresolved (open) thread is a lock gate (a
	// claim cannot lock while it carries one) and, on an already-locked
	// claim, a review_pending trigger — see OpenThreadIDs/HasOpenThreads.
	Comments []Comment `yaml:"comments,omitempty"`

	// SourcePath is the filesystem path the claim was loaded from. It is
	// populated by the claim loader (not part of the YAML schema itself)
	// and is used by internal/lock to write edits back to the right file.
	SourcePath string `yaml:"-"`
}

// EffectiveKind returns c's real Kind: unset maps to KindFact, otherwise
// the authored value. kind-shape refuses every value other than fact.
func (c Claim) EffectiveKind() Kind {
	if c.Kind == "" {
		return KindFact
	}
	return c.Kind
}

// IsProjectClaim reports a project-claims store node.
func (c Claim) IsProjectClaim() bool {
	return c.Scope == ScopeProject || IsProjectClaimID(c.ID)
}

// ClaimProseChars is the Unicode code-point count of body + steps + rows
// cells. raw_html is excluded: NIT-8 caps authored prose, not markup.
func ClaimProseChars(c Claim) int {
	n := utf8.RuneCountInString(c.Body)
	for _, step := range c.Steps {
		n += utf8.RuneCountInString(step)
	}
	for _, row := range c.Rows {
		for key, value := range row {
			if key == rowOrderKey {
				continue
			}
			n += utf8.RuneCountInString(rowCellText(value))
		}
	}
	return n
}

func rowCellText(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

// SummaryIsMissing reports a blank or whitespace-only summary.
func SummaryIsMissing(summary string) bool {
	return strings.TrimSpace(summary) == ""
}
