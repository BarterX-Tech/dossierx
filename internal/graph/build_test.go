package graph

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// buildFrom is the shape every test in this file uses: raw claims in,
// payload out, through the same catalog.Build the render path uses. It takes
// claims rather than a *catalog.Catalog so a table case reads as the corpus
// it describes.
func buildFrom(t *testing.T, cfg *config.Config, claims ...model.Claim) Payload {
	t.Helper()
	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	return Build(cat, cfg)
}

// nodeByID finds a node in a payload, failing loudly rather than returning a
// zero value a later assertion would compare against and silently pass.
func nodeByID(t *testing.T, p Payload, id string) Node {
	t.Helper()
	for _, n := range p.Nodes {
		if n.ID == id {
			return n
		}
	}
	t.Fatalf("no node %q in payload (%d nodes)", id, len(p.Nodes))
	return Node{}
}

func TestBuildNodes(t *testing.T) {
	cfg := &config.Config{
		Modules: []string{"widget"},
		Facets:  []string{"contract", "internals"},
	}

	cases := []struct {
		name  string
		claim model.Claim
		want  Node
	}{
		{
			name: "every field populated",
			claim: model.Claim{
				ID:            "widget.contract.retry-policy",
				Module:        "widget",
				Facet:         "contract",
				Status:        model.StatusLocked,
				Kind:          model.KindFact,
				BuildRole:     model.BuildRoleAPI,
				Emphasis:      true,
				ReviewPending: true,
				Comments: []model.Comment{
					{ID: "c-000001", Status: model.CommentStatusOpen},
					{ID: "c-000002", Status: model.CommentStatusResolved},
				},
			},
			want: Node{
				ID: "widget.contract.retry-policy", Title: "Retry Policy",
				Module: "widget", Facet: "contract",
				Status: "locked", Kind: "fact", BuildRole: "api",
				Emphasis: true, ReviewPending: true, OpenComments: 1,
			},
		},
		{
			name: "empty module",
			claim: model.Claim{
				ID: "widget.contract.no-module", Facet: "contract",
				Status: model.StatusDraft,
			},
			want: Node{
				ID: "widget.contract.no-module", Title: "No Module",
				Module: "", Facet: "contract", Status: "draft", Kind: "fact",
			},
		},
		{
			name: "empty facet",
			claim: model.Claim{
				ID: "widget.contract.no-facet", Module: "widget",
				Status: model.StatusDraft,
			},
			want: Node{
				ID: "widget.contract.no-facet", Title: "No Facet",
				Module: "widget", Facet: "", Status: "draft", Kind: "fact",
			},
		},
		{
			name: "empty build_role is emitted as empty, not omitted",
			claim: model.Claim{
				ID: "widget.contract.unphased", Module: "widget",
				Facet: "contract", Status: model.StatusDraft,
			},
			want: Node{
				ID: "widget.contract.unphased", Title: "Unphased",
				Module: "widget", Facet: "contract",
				Status: "draft", Kind: "fact", BuildRole: "",
			},
		},
		{
			// The reserved overview facet implies orientation-note whether
			// or not the author set kind — EffectiveKind's rule, which the
			// payload must carry resolved so the client never re-derives it.
			name: "overview facet infers orientation-note without an explicit kind",
			claim: model.Claim{
				ID: "widget.overview.read-me-first", Module: "widget",
				Facet: config.ReservedOverviewFacet, Status: model.StatusDraft,
			},
			want: Node{
				ID: "widget.overview.read-me-first", Title: "Read Me First",
				Module: "widget", Facet: "overview",
				Status: "draft", Kind: "orientation-note",
			},
		},
		{
			name: "an id that is not three segments keeps its raw id as the title",
			claim: model.Claim{
				ID: "not-a-real-id", Module: "widget", Facet: "contract",
				Status: model.StatusDraft,
			},
			want: Node{
				ID: "not-a-real-id", Title: "not-a-real-id",
				Module: "widget", Facet: "contract", Status: "draft", Kind: "fact",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := buildFrom(t, cfg, tc.claim)
			if len(p.Nodes) != 1 {
				t.Fatalf("node count = %d, want 1", len(p.Nodes))
			}
			if !reflect.DeepEqual(p.Nodes[0], tc.want) {
				t.Errorf("node =\n  %#v\nwant\n  %#v", p.Nodes[0], tc.want)
			}
		})
	}

	t.Run("nodes are sorted by id", func(t *testing.T) {
		p := buildFrom(t, cfg,
			model.Claim{ID: "widget.contract.zulu", Module: "widget", Facet: "contract"},
			model.Claim{ID: "widget.contract.alpha", Module: "widget", Facet: "contract"},
			model.Claim{ID: "widget.contract.mike", Module: "widget", Facet: "contract"},
		)
		got := make([]string, 0, len(p.Nodes))
		for _, n := range p.Nodes {
			got = append(got, n.ID)
		}
		want := []string{"widget.contract.alpha", "widget.contract.mike", "widget.contract.zulu"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("node order = %v, want %v", got, want)
		}
	})

	// The graph audits claims, not code. There is no code-grounding signal
	// in this payload, and this assertion is the standing check that one did
	// not come back — asserted on the MARSHALLED object, because that is
	// what the client actually reads.
	t.Run("the marshalled node object carries no has_code_link key", func(t *testing.T) {
		p := buildFrom(t, cfg, model.Claim{
			ID: "widget.contract.retry-policy", Module: "widget",
			Facet: "contract", Status: model.StatusLocked,
		})
		b, err := json.Marshal(p.Nodes[0])
		if err != nil {
			t.Fatalf("json.Marshal(node): %v", err)
		}
		if strings.Contains(string(b), "has_code_link") {
			t.Errorf("node object carries has_code_link: %s", b)
		}

		var keys map[string]json.RawMessage
		if err := json.Unmarshal(b, &keys); err != nil {
			t.Fatalf("json.Unmarshal(node): %v", err)
		}
		want := []string{
			"id", "title", "module", "facet", "status", "kind", "build_role",
			"emphasis", "review_pending", "open_comments", "in_degree", "out_degree",
		}
		if len(keys) != len(want) {
			t.Errorf("node has %d keys, want exactly %d: %s", len(keys), len(want), b)
		}
		for _, k := range want {
			if _, ok := keys[k]; !ok {
				t.Errorf("node object is missing key %q: %s", k, b)
			}
		}
	})
}

func TestBuildEdges(t *testing.T) {
	cfg := &config.Config{Modules: []string{"widget"}, Facets: []string{"contract"}}

	base := func(id string) model.Claim {
		return model.Claim{ID: id, Module: "widget", Facet: "contract", Status: model.StatusDraft}
	}

	t.Run("all three types, in the declared direction", func(t *testing.T) {
		a, b, d := base("widget.contract.a"), base("widget.contract.b"), base("widget.contract.d")
		a.RestsOn = []string{"widget.contract.b"}
		a.Mirrors = []string{"widget.contract.d"}
		a.Governed = model.Governed{Type: "widget.contract.d"}
		p := buildFrom(t, cfg, a, b, d)

		want := []Edge{
			{From: "widget.contract.a", To: "widget.contract.d", Type: EdgeGovernedBy},
			{From: "widget.contract.a", To: "widget.contract.d", Type: EdgeMirrors},
			{From: "widget.contract.a", To: "widget.contract.b", Type: EdgeRestsOn},
		}
		if !reflect.DeepEqual(p.Edges, want) {
			t.Errorf("edges =\n  %#v\nwant\n  %#v", p.Edges, want)
		}
		if p.Dropped.UnresolvedEdges != 0 {
			t.Errorf("dropped = %d, want 0", p.Dropped.UnresolvedEdges)
		}
	})

	// The same guard internal/lint/dangling.go applies: "" and "none" are
	// both "deliberately not governed", and neither is an edge.
	for _, typ := range []string{"", "none"} {
		t.Run(fmt.Sprintf("governed_by type %q produces no edge", typ), func(t *testing.T) {
			a := base("widget.contract.a")
			a.Governed = model.Governed{Type: typ, Reason: "not backed by doctrine"}
			p := buildFrom(t, cfg, a)
			if len(p.Edges) != 0 {
				t.Errorf("edges = %#v, want none", p.Edges)
			}
			if p.Dropped.UnresolvedEdges != 0 {
				t.Errorf("dropped = %d, want 0 (a non-edge is not a drop)", p.Dropped.UnresolvedEdges)
			}
		})
	}

	t.Run("unknown targets are dropped and counted, of every type", func(t *testing.T) {
		a := base("widget.contract.a")
		a.RestsOn = []string{"widget.contract.ghost"}
		a.Mirrors = []string{"widget.contract.phantom"}
		a.Governed = model.Governed{Type: "widget.contract.spectre"}
		p := buildFrom(t, cfg, a)
		if len(p.Edges) != 0 {
			t.Errorf("edges = %#v, want none", p.Edges)
		}
		if p.Dropped.UnresolvedEdges != 3 {
			t.Errorf("dropped.unresolved_edges = %d, want 3", p.Dropped.UnresolvedEdges)
		}
	})

	t.Run("edges are sorted by (from, type, to)", func(t *testing.T) {
		a, b, c := base("widget.contract.a"), base("widget.contract.b"), base("widget.contract.c")
		a.RestsOn = []string{"widget.contract.c", "widget.contract.b"}
		a.Mirrors = []string{"widget.contract.c"}
		b.RestsOn = []string{"widget.contract.a"}
		p := buildFrom(t, cfg, b, a, c)

		want := []Edge{
			{From: "widget.contract.a", To: "widget.contract.c", Type: EdgeMirrors},
			{From: "widget.contract.a", To: "widget.contract.b", Type: EdgeRestsOn},
			{From: "widget.contract.a", To: "widget.contract.c", Type: EdgeRestsOn},
			{From: "widget.contract.b", To: "widget.contract.a", Type: EdgeRestsOn},
		}
		if !reflect.DeepEqual(p.Edges, want) {
			t.Errorf("edges =\n  %#v\nwant\n  %#v", p.Edges, want)
		}
	})

	// Design §3.1: a self-edge is a real, distinct defect the client reports
	// under its own rule id. Swallowing it here would make that rule unable
	// to fire at all.
	t.Run("a self-edge survives to the payload", func(t *testing.T) {
		a := base("widget.contract.a")
		a.RestsOn = []string{"widget.contract.a"}
		p := buildFrom(t, cfg, a)
		want := []Edge{{From: "widget.contract.a", To: "widget.contract.a", Type: EdgeRestsOn}}
		if !reflect.DeepEqual(p.Edges, want) {
			t.Errorf("edges = %#v, want %#v", p.Edges, want)
		}
	})
}

func TestBuildDegrees(t *testing.T) {
	cfg := &config.Config{Modules: []string{"m"}, Facets: []string{"f"}}
	base := func(id string) model.Claim {
		return model.Claim{ID: id, Module: "m", Facet: "f", Status: model.StatusDraft}
	}

	// hub is rested on by two claims, mirrors one, and is governed by one —
	// so every one of the three edge types contributes to its degrees.
	hub, a, b, doc := base("m.f.hub"), base("m.f.a"), base("m.f.b"), base("m.f.doc")
	a.RestsOn = []string{"m.f.hub"}
	b.RestsOn = []string{"m.f.hub"}
	hub.Mirrors = []string{"m.f.a"}
	hub.Governed = model.Governed{Type: "m.f.doc"}
	p := buildFrom(t, cfg, hub, a, b, doc)

	want := map[string][2]int{ // id -> {in, out}
		"m.f.hub": {2, 2},
		"m.f.a":   {1, 1},
		"m.f.b":   {0, 1},
		"m.f.doc": {1, 0},
	}
	for id, wd := range want {
		n := nodeByID(t, p, id)
		if n.InDegree != wd[0] || n.OutDegree != wd[1] {
			t.Errorf("%s: in/out = %d/%d, want %d/%d", id, n.InDegree, n.OutDegree, wd[0], wd[1])
		}
	}

	t.Run("a dropped edge is not counted in either degree", func(t *testing.T) {
		c := base("m.f.c")
		c.RestsOn = []string{"m.f.nowhere"}
		p := buildFrom(t, cfg, c)
		n := nodeByID(t, p, "m.f.c")
		if n.OutDegree != 0 || n.InDegree != 0 {
			t.Errorf("in/out = %d/%d, want 0/0 — degrees must agree with the emitted edge list", n.InDegree, n.OutDegree)
		}
		if p.Dropped.UnresolvedEdges != 1 {
			t.Errorf("dropped = %d, want 1", p.Dropped.UnresolvedEdges)
		}
	})
}

func TestBuildGroups(t *testing.T) {
	cfg := &config.Config{
		// Deliberately NOT alphabetical: config order is the sidebar's
		// reading order and must survive verbatim.
		Modules: []string{"viewer", "engine", "cli"},
		Facets:  []string{"contract", "schema", "behavior"},
	}

	p := buildFrom(t, cfg,
		model.Claim{ID: "cli.contract.a", Module: "cli", Facet: "contract"},
		model.Claim{ID: "zeta.behavior.b", Module: "zeta", Facet: "behavior"},
		model.Claim{ID: "alpha.overview.c", Module: "alpha", Facet: "overview"},
		model.Claim{ID: "engine.verification.d", Module: "engine", Facet: "verification"},
		model.Claim{ID: "orphan.x.e", Module: "", Facet: ""},
	)

	wantModules := []string{"viewer", "engine", "cli", "alpha", "zeta"}
	if !reflect.DeepEqual(p.Groups.Modules, wantModules) {
		t.Errorf("groups.modules = %v, want %v (config order, then extras sorted)", p.Groups.Modules, wantModules)
	}
	wantFacets := []string{"contract", "schema", "behavior", "overview", "verification"}
	if !reflect.DeepEqual(p.Groups.Facets, wantFacets) {
		t.Errorf("groups.facets = %v, want %v (config order, then extras sorted)", p.Groups.Facets, wantFacets)
	}
	for _, g := range append(append([]string{}, p.Groups.Modules...), p.Groups.Facets...) {
		if g == "" {
			t.Errorf("groups contain an empty-string group: %#v", p.Groups)
		}
	}
}

// TestBuildDeterministic is the property three tracked, committed fixture
// viewers rest on: the same corpus must encode to the same bytes, every time,
// regardless of the order the loader happened to hand the claims over in or
// what a map iteration did on the way.
func TestBuildDeterministic(t *testing.T) {
	cfg := &config.Config{
		Modules: []string{"viewer", "engine", "cli"},
		Facets:  []string{"contract", "schema", "behavior"},
	}

	claims := make([]model.Claim, 0, 60)
	byID := make(map[string]model.Claim, 60)
	mods := []string{"viewer", "engine", "cli", "extra"}
	facets := []string{"contract", "schema", "behavior", "overview"}
	for i := range 60 {
		c := model.Claim{
			ID:     fmt.Sprintf("%s.%s.claim-%02d", mods[i%len(mods)], facets[i%len(facets)], i),
			Module: mods[i%len(mods)],
			Facet:  facets[i%len(facets)],
			Status: model.StatusDraft,
		}
		claims = append(claims, c)
		byID[c.ID] = c
	}
	// Wire a dense-enough edge set that ordering has something to get wrong.
	for i := range claims {
		if i >= 3 {
			claims[i].RestsOn = []string{claims[i-1].ID, claims[i-3].ID}
		}
		if i%5 == 0 && i+1 < len(claims) {
			claims[i].Mirrors = []string{claims[i+1].ID}
		}
		if i%7 == 0 {
			claims[i].Governed = model.Governed{Type: claims[0].ID}
		}
		byID[claims[i].ID] = claims[i]
	}

	baseline, err := Encode(buildFrom(t, cfg, claims...))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	rng := rand.New(rand.NewSource(1))
	for i := range 100 {
		shuffled := append([]model.Claim(nil), claims...)
		rng.Shuffle(len(shuffled), func(a, b int) { shuffled[a], shuffled[b] = shuffled[b], shuffled[a] })
		got, err := Encode(buildFrom(t, cfg, shuffled...))
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		if string(got) != string(baseline) {
			t.Fatalf("iteration %d: shuffled input produced different bytes", i)
		}
	}

	// And from a map-sourced claim slice, so Go's randomized map iteration
	// order is a real input rather than a hypothetical one.
	for i := range 100 {
		fromMap := make([]model.Claim, 0, len(byID))
		for _, c := range byID {
			fromMap = append(fromMap, c)
		}
		got, err := Encode(buildFrom(t, cfg, fromMap...))
		if err != nil {
			t.Fatalf("Encode: %v", err)
		}
		if string(got) != string(baseline) {
			t.Fatalf("map-sourced iteration %d: produced different bytes", i)
		}
	}
}

// TestBuildDoesNotStampTime is the executable half of "GeneratedAt is stamped
// by the caller". Its grep half — that "time" does not appear in build.go at
// all — is part of this lane's proving command, because a test can only show
// the clock was not read on THIS run.
func TestBuildDoesNotStampTime(t *testing.T) {
	cfg := &config.Config{Modules: []string{"m"}, Facets: []string{"f"}}
	claim := model.Claim{ID: "m.f.a", Module: "m", Facet: "f", Status: model.StatusDraft}

	p := buildFrom(t, cfg, claim)
	if p.GeneratedAt != "" {
		t.Errorf("Build stamped generated_at = %q; it must be left for the caller", p.GeneratedAt)
	}

	first, err := Encode(p)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// A measurable interval, so a second-resolution clock read would show.
	time.Sleep(1100 * time.Millisecond)
	second, err := Encode(buildFrom(t, cfg, claim))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("two builds over an interval differ:\n first: %s\nsecond: %s", first, second)
	}
}

// TestBuildNilSafe pins totality. Render calls Build unconditionally and has
// no branch for a payload that failed to build, because there is no such
// thing: every input below returns a valid payload.
func TestBuildNilSafe(t *testing.T) {
	cases := []struct {
		name string
		cat  *catalog.Catalog
		cfg  *config.Config
	}{
		{"both nil", nil, nil},
		{"nil catalog, real config", nil, &config.Config{Modules: []string{"m"}, Facets: []string{"f"}}},
		{"empty catalog, nil config", &catalog.Catalog{}, nil},
		{"catalog with nil claims slice", &catalog.Catalog{Claims: nil}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := Build(tc.cat, tc.cfg)
			if p.Schema != SchemaVersion {
				t.Errorf("schema = %d, want %d", p.Schema, SchemaVersion)
			}
			if p.Nodes == nil || p.Edges == nil || p.Groups.Modules == nil || p.Groups.Facets == nil {
				t.Errorf("a nil slice reached the payload: %#v", p)
			}
			out, err := Encode(p)
			if err != nil {
				t.Fatalf("Encode: %v", err)
			}
			want := []string{`"nodes":[]`, `"edges":[]`}
			if tc.cfg == nil {
				// With no config there is nothing to declare and no claim
				// to name anything, so both group axes are empty too.
				want = append(want, `"modules":[]`, `"facets":[]`)
			}
			for _, w := range want {
				if !strings.Contains(string(out), w) {
					t.Errorf("encoded payload missing %s (the client iterates these without a null guard):\n%s", w, out)
				}
			}
			if strings.Contains(string(out), "null") {
				t.Errorf("encoded payload contains null:\n%s", out)
			}
		})
	}

	t.Run("claims with empty ids", func(t *testing.T) {
		// An empty id is not a shape any linted corpus reaches, but "serve"
		// never lints, so Build has to survive it rather than assume it away.
		p := buildFrom(t, nil,
			model.Claim{ID: "", Module: "m", Facet: "f"},
			model.Claim{ID: "", Module: "m", Facet: "f", RestsOn: []string{""}},
			model.Claim{ID: "m.f.real", Module: "m", Facet: "f", RestsOn: []string{""}},
		)
		if len(p.Nodes) != 3 {
			t.Errorf("node count = %d, want 3 (an empty id is still a claim)", len(p.Nodes))
		}
		if _, err := Encode(p); err != nil {
			t.Fatalf("Encode: %v", err)
		}
	})
}
