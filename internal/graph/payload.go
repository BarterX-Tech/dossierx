// Package graph turns a built catalog into the JSON payload the viewer's
// claims-graph pane reads. It exports exactly two functions — Build and
// Encode — plus the wire types they move.
//
// WHAT THIS PACKAGE IS FOR, AND WHAT IT DELIBERATELY IS NOT.
//
// Go emits facts; the browser computes verdicts. Build states what the
// corpus IS — every claim, every declared edge, project-wide degrees, the
// group ordering the sidebar already uses — and nothing about what any of
// that MEANS. Every gap rule ("this claim is isolated", "this module is a
// sink") is computed client-side against whatever scope the reader has
// selected, because a gap list precomputed here would be wrong the moment a
// reader scopes the view to one module.
//
// THE GRAPH AUDITS CLAIMS, NOT CODE. Source files are not nodes and whether
// a claim has an implementation link is not a fact this payload carries.
// That question belongs to "dossierx check"'s impl-link stage, which already
// answers it with its own vocabulary and its own exit code; a second surface
// for it would be a second place to keep correct. Concretely: Build takes
// two arguments and no more, no node field carries a code-link flag, and this
// package does not import internal/implink (nor internal/render, nor
// internal/lint — the import direction is one-way and .golangci.yml's
// depguard block says so).
//
// PURITY IS A CONTRACT, NOT A STYLE CHOICE. Build performs no I/O, reads no
// clock, and returns no error, so it cannot fail a render. Two calls over the
// same catalog produce byte-identical Encode output — which matters because
// this payload lands inside three tracked, committed fixture viewers, where a
// moving byte is a permanently dirty diff. GeneratedAt is therefore left
// empty by Build and stamped by each caller (internal/render/graph_view.go
// from Render's own generatedAt; internal/serve's handleGraph from
// time.Now().UTC() at request time), so the one field that must move is the
// one field Build never touches.
//
// # Escaping: the one place this package can be unsafe
//
// The payload is injected into the rendered document inside a
// <script type="application/json"> block as an html/template template.JS
// value, and html/template performs NO escaping at all in that context. A
// payload string containing a literal script-closing tag would write straight
// out of the block and everything after it would parse as HTML. This is
// reachable rather than theoretical: a claim's module, its facet, and the id
// slug its title derives from are author-authored YAML scalars, and under
// "dossierx serve" no lint has run to constrain them.
//
// The guard is encoding/json's DEFAULT HTML escaping, which turns '<', '>'
// and '&' into their \u00XX forms before the bytes ever reach the template.
// Therefore, in this package and in this repository:
//
//   - Encode marshals via json.Marshal. It never uses json.Encoder, whose
//     escape-HTML behaviour is a settable toggle, and it never hand-assembles
//     JSON from string concatenation.
//   - That toggle must never be turned off anywhere in this repository. This
//     comment does not spell the method's name out, and that is deliberate:
//     the property is enforced by a repo-wide grep for the identifier which is
//     part of this lane's own proving command, so writing it here — even to
//     forbid it — would be the one thing that makes the gate fail. See
//     encode_test.go's TestEncodeEscapesScriptClose for the executable half.
package graph

// SchemaVersion is bumped when the payload shape changes in a way a browser
// built against an older shape cannot read. It rides on the wire as the
// payload's "schema" key so the client can refuse a payload it does not
// understand rather than silently mis-rendering one.
const SchemaVersion = 1

// Edge type values. These are the wire strings, matching the three claim
// edge kinds model.Claim declares (Mirrors, RestsOn, Governed). They are
// constants rather than literals because graph-core.js keys its edge-type
// toggles off them and viewer-tests asserts on them, so they are contract.
const (
	// EdgeRestsOn is one entry of model.Claim.RestsOn.
	EdgeRestsOn = "rests_on"
	// EdgeMirrors is one entry of model.Claim.Mirrors. Directional in
	// storage even though reciprocity is a lint rather than a model
	// invariant — see internal/lint/mirror_reciprocal.go.
	EdgeMirrors = "mirrors"
	// EdgeGovernedBy is a claim's single governed_by edge, emitted only
	// when the type names a real doctrine claim (see Build).
	EdgeGovernedBy = "governed_by"
)

// Node is one claim, projected down to the facts the pane draws with.
//
// Every field is emitted unconditionally — no omitempty anywhere in this
// file. The client reads a fixed shape, and an absent key and a zero value
// are different things to it: "build_role": "" is the fact the
// missing_build_phase rule keys off, and a node object that simply lacked
// the key would make that rule silently unable to fire.
type Node struct {
	// ID is model.Claim.ID, verbatim.
	ID string `json:"id"`

	// Title is the readable label derived from ID's slug segment — the
	// same derivation the viewer's cards use. It is DERIVED, never
	// authored: model.Claim has no title field, on purpose.
	Title string `json:"title"`

	// Module and Facet are model.Claim.Module / .Facet verbatim. Either
	// may be empty, or may name something absent from the project config;
	// the browser buckets those under a catch-all label and the reserved
	// --dxg-facet-other palette slot rather than dropping the node.
	Module string `json:"module"`
	Facet  string `json:"facet"`

	// Status is model.Claim.Status: the closed enum "draft" | "locked".
	Status string `json:"status"`

	// Kind is model.Claim.EffectiveKind(), never the raw Kind field — the
	// reserved overview facet implies orientation-note without the author
	// repeating it, and a client re-deriving that rule would be a second
	// place to keep it correct.
	Kind string `json:"kind"`

	// BuildRole is model.Claim.BuildRole. Empty is meaningful, not
	// missing: it is what the client's missing_build_phase hint counts.
	BuildRole string `json:"build_role"`

	// Emphasis is model.Claim.Emphasis.
	Emphasis bool `json:"emphasis"`

	// ReviewPending is model.Claim.ReviewPending — engine-managed, and
	// only meaningful on a locked claim.
	ReviewPending bool `json:"review_pending"`

	// OpenComments is len(model.Claim.OpenThreadIDs()).
	OpenComments int `json:"open_comments"`

	// InDegree and OutDegree count this node's PROJECT-WIDE edges across
	// all three types, over the edges that actually reached Edges (an
	// unresolved edge is dropped, so it is not counted here either — a
	// degree that disagreed with the edge list beside it would be a fact
	// the client could not reconcile). They are facts; the browser
	// recomputes scope-relative degree for node radius and for the
	// connectivity rules, because a claim isolated within one module may
	// be well-connected project-wide.
	InDegree  int `json:"in_degree"`
	OutDegree int `json:"out_degree"`
}

// Edge is one declared relation, in the direction the claim declares it.
// Edges whose target is not a known claim id never appear here; they are
// counted into Dropped instead.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
	// Type is one of EdgeRestsOn, EdgeMirrors, EdgeGovernedBy.
	Type string `json:"type"`
}

// Groups carries the two grouping axes the pane can collapse the graph
// along, each in the order the viewer's sidebar already reads them in:
// the project config's declared order first, then anything else a claim
// mentions, sorted. Empty values are not groups and are excluded.
type Groups struct {
	Modules []string `json:"modules"`
	Facets  []string `json:"facets"`
}

// Dropped reports what Build could not represent, so the pane can say so in
// one line rather than silently drawing a smaller graph than the data
// describes. "dossierx check" refuses to render a corpus with a dangling
// edge — it returns above the catalog and render stages on any
// error-severity lint finding — but "dossierx serve" never lints, so a live
// session genuinely can carry one.
type Dropped struct {
	// UnresolvedEdges counts declared edges whose target id matches no
	// claim in the catalog.
	UnresolvedEdges int `json:"unresolved_edges"`
}

// Payload is the whole wire document: the JSON that lands in the rendered
// viewer's <script type="application/json"> block and that GET /api/graph
// returns.
type Payload struct {
	Schema int `json:"schema"`

	// GeneratedAt is RFC3339 UTC and is stamped by the CALLER, never by
	// Build — see this package's doc comment. Build leaves it empty, which
	// is what keeps Build's output byte-stable and its unit tests free of a
	// moving byte.
	GeneratedAt string `json:"generated_at"`

	// Nodes is sorted by ID. Never nil: an empty corpus marshals as [],
	// because the client iterates it without a null guard.
	Nodes []Node `json:"nodes"`

	// Edges is sorted by (From, Type, To). Never nil, for the same reason
	// as Nodes.
	Edges []Edge `json:"edges"`

	Groups  Groups  `json:"groups"`
	Dropped Dropped `json:"dropped"`
}
