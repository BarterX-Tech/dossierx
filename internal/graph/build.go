// build.go holds Build — the pure, deterministic, clock-free projection of a
// catalog onto the wire types in payload.go — and the small label derivation
// it needs. Nothing in this file performs I/O, reads the clock, or returns an
// error: Render must not be able to fail because of this package.
package graph

import (
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// Build projects cat and cfg onto the wire payload the viewer's graph pane
// reads. It takes TWO arguments and no more: the graph audits claims, not
// code, so there is no impl-link lookup to thread in and no code-link field
// to fill (see the package doc comment).
//
// It is TOTAL. Build(nil, nil) returns a valid, empty-ish payload; so does a
// catalog of claims with empty ids. It never panics and never returns an
// error, which is what lets internal/render call it unconditionally without
// gaining a new failure mode.
//
// It is PURE. No I/O, no clock, no map iteration reaching the output.
// GeneratedAt is left empty for the caller to stamp. Two calls over the same
// claims — in any input order — produce byte-identical Encode output, which
// is the property three tracked fixture viewers rest on.
//
// Ordering, stated because the client and the fixtures both depend on it:
// nodes by id; edges by (from, type, to); groups.modules in cfg order then
// any other module a claim names, sorted; groups.facets likewise.
func Build(cat *catalog.Catalog, cfg *config.Config) Payload {
	p := Payload{
		Schema: SchemaVersion,
		Nodes:  []Node{},
		Edges:  []Edge{},
		Groups: Groups{Modules: []string{}, Facets: []string{}},
	}

	var claims []model.Claim
	if cat != nil {
		claims = cat.Claims
	}

	// Known ids first: an edge is only emitted when its target resolves to
	// a claim in this same set, so the set has to be complete before any
	// edge is considered.
	known := make(map[string]bool, len(claims))
	for _, c := range claims {
		known[c.ID] = true
	}

	// Nodes. Degrees are filled in a second pass, once the edge set that
	// actually reached the payload is known.
	for _, c := range claims {
		p.Nodes = append(p.Nodes, Node{
			ID:            c.ID,
			Title:         claimLabel(c.ID),
			Module:        c.Module,
			Facet:         c.Facet,
			Status:        string(c.Status),
			Kind:          string(c.EffectiveKind()),
			BuildRole:     string(c.BuildRole),
			Emphasis:      c.Emphasis,
			ReviewPending: c.ReviewPending,
			OpenComments:  len(c.OpenThreadIDs()),
		})
	}

	// Edges, in the direction the claim declares them. A declared entry
	// whose target is not a known id is dropped and counted rather than
	// emitted: the pane says so in one line instead of silently drawing a
	// smaller graph than the data describes.
	//
	// Duplicate declarations (the same target listed twice under rests_on)
	// produce duplicate edges, deliberately: "one edge per declared
	// relation" is the contract, and the client aggregates by (from, to,
	// type) with a weight, so a duplicate reads as weight rather than
	// disappearing here. Self-edges are emitted for the same reason — the
	// client reports them under their own rule id, and swallowing them
	// here would make that rule unable to fire.
	for _, c := range claims {
		for _, target := range c.RestsOn {
			p.appendEdge(known, c.ID, target, EdgeRestsOn)
		}
		for _, target := range c.Mirrors {
			p.appendEdge(known, c.ID, target, EdgeMirrors)
		}
		// The same guard internal/lint/dangling.go uses: "none" and the
		// empty type are both "deliberately not governed", not an edge.
		if t := c.Governed.Type; t != "" && t != "none" {
			p.appendEdge(known, c.ID, t, EdgeGovernedBy)
		}
	}

	sort.Slice(p.Nodes, func(i, j int) bool { return p.Nodes[i].ID < p.Nodes[j].ID })
	sort.Slice(p.Edges, func(i, j int) bool {
		a, b := p.Edges[i], p.Edges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		return a.To < b.To
	})

	// Project-wide degrees, over all three edge types, counted over the
	// edges that actually reached the payload. Counting dropped edges here
	// too would hand the client a degree it cannot reconcile against the
	// edge list sitting beside it.
	in := make(map[string]int, len(p.Nodes))
	out := make(map[string]int, len(p.Nodes))
	for _, e := range p.Edges {
		out[e.From]++
		in[e.To]++
	}
	for i := range p.Nodes {
		p.Nodes[i].InDegree = in[p.Nodes[i].ID]
		p.Nodes[i].OutDegree = out[p.Nodes[i].ID]
	}

	var cfgModules, cfgFacets []string
	if cfg != nil {
		cfgModules, cfgFacets = cfg.Modules, cfg.Facets
	}
	seenModules := make([]string, 0, len(claims))
	seenFacets := make([]string, 0, len(claims))
	for _, c := range claims {
		seenModules = append(seenModules, c.Module)
		seenFacets = append(seenFacets, c.Facet)
	}
	p.Groups.Modules = groupOrder(cfgModules, seenModules)
	p.Groups.Facets = groupOrder(cfgFacets, seenFacets)

	return p
}

// appendEdge emits one edge, or counts one drop. It is a method on Payload
// so the drop counter and the edge slice can never fall out of step.
func (p *Payload) appendEdge(known map[string]bool, from, to, typ string) {
	if !known[to] {
		p.Dropped.UnresolvedEdges++
		return
	}
	p.Edges = append(p.Edges, Edge{From: from, To: to, Type: typ})
}

// groupOrder returns declared first — in the project config's own order,
// which is the order the viewer's sidebar already reads modules and facets
// in — then every other distinct value the claims mention, sorted.
//
// The empty string is never a group. A claim with no module still gets a
// node; the browser buckets it under a catch-all label rather than under a
// group named "", which would render as an unnamed swatch in the legend and
// tell a reader nothing.
//
// Extras exist for two ordinary reasons, not only for typos: the reserved
// "overview" facet is real and is deliberately absent from every project's
// declared facet list, and a claim can name a module the config has not
// caught up with yet. Sorting them puts them after the declared list in a
// stable place rather than dropping them.
func groupOrder(declared, seen []string) []string {
	out := make([]string, 0, len(declared)+len(seen))
	taken := make(map[string]bool, len(declared)+len(seen))
	for _, v := range declared {
		if v == "" || taken[v] {
			continue
		}
		taken[v] = true
		out = append(out, v)
	}
	extras := make([]string, 0, len(seen))
	for _, v := range seen {
		if v == "" || taken[v] {
			continue
		}
		taken[v] = true
		extras = append(extras, v)
	}
	sort.Strings(extras)
	return append(out, extras...)
}

// claimLabel turns a claim id into the readable label the viewer shows in its
// place: "widget.contract.retry-policy" -> "Retry Policy". Only the slug
// segment becomes the label; module and facet are context the pane already
// carries on the node's own fields. An id that is not exactly three non-empty
// dot-separated segments renders as the RAW ID, verbatim — never a partial
// label, which would silently mislabel an unlinted claim, and never a panic.
//
// WHY THIS IS DUPLICATED RATHER THAN CALLED.
//
// internal/render/components.ClaimLabel is the same twelve lines and is the
// canonical implementation; the viewer's cards use it, and the two must agree
// or the graph's node labels and the reading view's card headings disagree
// about the same claim. Calling it directly is nonetheless impossible here:
// that package transitively imports internal/lint and internal/implink, and
// this package's whole architectural point is that it imports neither (see
// the package doc comment, and .golangci.yml's depguard block, which denies
// the internal/render prefix outright). Moving ClaimLabel down into a leaf
// package would be the real fix and is the right follow-up; it is a change to
// files this lane does not own.
//
// So the duplication is deliberate, small, and stated. It is also the reason
// this function is a faithful transcription rather than a tidier rewrite:
// divergence in the derivation is exactly the failure this comment exists to
// make findable.
func claimLabel(id string) string {
	segs := strings.Split(id, ".")
	if len(segs) != 3 || segs[0] == "" || segs[1] == "" || segs[2] == "" {
		return id
	}
	return displayCase(segs[2])
}

// displayCase is the slug-to-words transformation claimLabel applies, kept
// separate for the same reason components.DisplayCase is: it is the half that
// is about presentation rather than about id grammar.
func displayCase(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for i, w := range words {
		if w == "" {
			continue
		}
		r := []rune(w)
		r[0] = []rune(strings.ToUpper(string(r[0])))[0]
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}
