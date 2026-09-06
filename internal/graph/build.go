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
// any other module a claim names, sorted; groups.facets and groups.tracks
// likewise; a node's own tracks in the order the claim declares them.
//
// It is also ZERO-COST WHEN UNUSED, and that is a contract rather than a
// happy accident: over a corpus where no claim joins a track and the config
// declares none, Encode's bytes are IDENTICAL to what they were before tracks
// existed. Both new keys carry omitempty and both are left nil rather than
// empty here. TestTrackLessPayloadIsByteIdenticalToPreTracks holds it.
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
		node := Node{
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
			Tracks:        nodeTracks(c),
		}
		if assessment, ok := cat.Readiness[c.ID]; ok {
			assessmentCopy := assessment
			node.Readiness = &assessmentCopy
		}
		p.Nodes = append(p.Nodes, node)
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
		// c.Tracks IS NOT WALKED HERE, AND NOTHING BELONGS IN THIS LOOP FOR
		// IT. Track membership is a set, not a dependency: it has no
		// direction, so it cannot be a cycle, and the client's scc() walks
		// every edge in Edges. An edge kind that joined the walk here would
		// ring every claim in a track red under the `cycle` rule and hand a
		// reviewer a structural defect the corpus does not have. Membership
		// rides on the node instead (Node.Tracks), where a set belongs. See
		// model.TrackRef and internal/lint/mixed_cycle.go for the same
		// decision taken twice before this one.
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
	var cfgTracks []config.Track
	if cfg != nil {
		cfgModules, cfgFacets, cfgTracks = cfg.Modules, cfg.Facets, cfg.Tracks
	}
	seenModules := make([]string, 0, len(claims))
	seenFacets := make([]string, 0, len(claims))
	for _, c := range claims {
		seenModules = append(seenModules, c.Module)
		seenFacets = append(seenFacets, c.Facet)
	}
	p.Groups.Modules = groupOrder(cfgModules, seenModules)
	p.Groups.Facets = groupOrder(cfgFacets, seenFacets)
	// Left NIL when this project has no tracks, which is what the field's
	// omitempty turns into an absent key. Modules and Facets are initialised
	// to empty slices at the top of this function precisely because they are
	// always emitted; this one must not be.
	p.Groups.Tracks = trackOrder(cfgTracks, claims)

	return p
}

// nodeTracks projects one claim's memberships onto the wire, resolving every
// role. It returns NIL rather than an empty slice for a claim that joins no
// track, which is what makes the key disappear instead of arriving as
// "tracks":[] — see Node's doc comment for why that byte matters.
//
// A membership naming no track is dropped, for the reason groupOrder drops
// the empty group: it selects nothing, it cannot be filtered on, and it would
// render as a nameless row in a control. The claim keeps its node either way.
func nodeTracks(c model.Claim) []NodeTrack {
	out := make([]NodeTrack, 0, len(c.Tracks))
	for _, t := range c.Tracks {
		if t.ID == "" {
			continue
		}
		out = append(out, NodeTrack{ID: t.ID, Role: string(t.EffectiveRole())})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// trackOrder returns the project's declared tracks in the config's own order,
// then every other track id some claim names, sorted — the same shape
// groupOrder produces for modules and facets, and for the same reasons.
//
// The extras are not only typos. "dossierx serve" never lints, so a claim
// naming a track the config has not caught up with reaches this function
// routinely during authoring. Dropping such an id would leave its claims
// carrying a membership the filter control could not offer: a track a reader
// can see on a node and can never select. It is listed under its raw id
// instead, which is simultaneously the only label available and the visible
// evidence that track-unknown has something to say about it.
//
// Returns nil — never an empty slice — when there is nothing to list, so a
// project that never opted into tracks emits no groups.tracks key at all.
func trackOrder(declared []config.Track, claims []model.Claim) []TrackGroup {
	out := make([]TrackGroup, 0, len(declared))
	taken := make(map[string]bool, len(declared))
	for _, t := range declared {
		if t.ID == "" || taken[t.ID] {
			continue
		}
		taken[t.ID] = true
		out = append(out, TrackGroup{ID: t.ID, Title: t.Title})
	}
	extras := make([]string, 0, len(claims))
	for _, c := range claims {
		for _, m := range c.Tracks {
			if m.ID == "" || taken[m.ID] {
				continue
			}
			taken[m.ID] = true
			extras = append(extras, m.ID)
		}
	}
	sort.Strings(extras)
	for _, id := range extras {
		out = append(out, TrackGroup{ID: id, Title: id})
	}
	if len(out) == 0 {
		return nil
	}
	return out
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
