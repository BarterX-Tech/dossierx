package graph

import (
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/catalog"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/loader"
)

// demoFixtureDir is the claims-graph demo corpus: the third tracked fixture,
// authored so the viewer's graph pane has a corpus with shape rather than a
// corpus that merely renders.
const demoFixtureDir = "../../testdata/fixture-graph-demo"

// loadDemoPayload takes the fixture through exactly the path "dossierx check"
// takes it through — config, loader, catalog, Build — so what this test
// asserts on is the payload a reader actually gets, not a reconstruction of
// it.
func loadDemoPayload(t *testing.T) (Payload, int) {
	t.Helper()
	cfg, err := config.LoadConfig(filepath.Join(demoFixtureDir, config.FileName))
	if err != nil {
		t.Fatalf("config.LoadConfig: %v", err)
	}
	claims, err := loader.LoadClaims(cfg.ClaimsDir)
	if err != nil {
		t.Fatalf("loader.LoadClaims: %v", err)
	}
	cat, err := catalog.Build(claims, cfg)
	if err != nil {
		t.Fatalf("catalog.Build: %v", err)
	}
	return Build(cat, cfg), len(claims)
}

// TestDemoFixtureSeedsEveryGapClass is what keeps the demo corpus a demo.
//
// The fixture exists so every gap class the pane can express has at least one
// instance to draw. Nothing about a YAML file says which class it was written
// to seed, so an edit that connects the module that was meant to be isolated,
// or that gives the under-phased module its missing phase, would silently
// empty a panel and nobody would notice until a reader opened the pane and
// found nothing there.
//
// Every assertion below is made on PAYLOAD FACTS rather than on the claim
// files, for two reasons: it is the payload the pane consumes, and a test that
// re-read the YAML would be asserting the fixture against itself.
func TestDemoFixtureSeedsEveryGapClass(t *testing.T) {
	p, claimCount := loadDemoPayload(t)

	if len(p.Nodes) == 0 || len(p.Edges) == 0 {
		t.Fatalf("payload is empty (%d nodes, %d edges) — every assertion below would pass vacuously", len(p.Nodes), len(p.Edges))
	}
	if len(p.Nodes) != claimCount {
		t.Errorf("payload has %d nodes for %d claims", len(p.Nodes), claimCount)
	}
	if p.Dropped.UnresolvedEdges != 0 {
		t.Errorf("dropped.unresolved_edges = %d, want 0 — this corpus passes check, so it cannot carry a dangling edge", p.Dropped.UnresolvedEdges)
	}

	// ---- derived views, computed once ----

	moduleOf := make(map[string]string, len(p.Nodes))
	degree := make(map[string]int, len(p.Nodes))
	for _, n := range p.Nodes {
		moduleOf[n.ID] = n.Module
		degree[n.ID] = n.InDegree + n.OutDegree
	}

	// Cross-module edge traffic, per module, in each direction.
	crossOut := map[string]int{}
	crossIn := map[string]int{}
	for _, e := range p.Edges {
		from, to := moduleOf[e.From], moduleOf[e.To]
		if from == to {
			continue
		}
		crossOut[from]++
		crossIn[to]++
	}

	// How many claims each governor governs.
	governs := map[string]int{}
	for _, e := range p.Edges {
		if e.Type == EdgeGovernedBy {
			governs[e.To]++
		}
	}

	// Which build phases each module has any claim in, and whether it has an
	// approved claim at all — the two inputs the missing-phase heuristic
	// takes.
	phasesSeen := map[string]map[string]bool{}
	hasLocked := map[string]bool{}
	for _, n := range p.Nodes {
		if phasesSeen[n.Module] == nil {
			phasesSeen[n.Module] = map[string]bool{}
		}
		if n.BuildRole != "" {
			phasesSeen[n.Module][n.BuildRole] = true
		}
		if n.Status == "locked" {
			hasLocked[n.Module] = true
		}
	}
	// The five real phases. "out-of-scope" is excluded by definition: it
	// means "deliberately not in any phase", so its absence is not a gap.
	phases := []string{"orientation", "schema", "behavior", "api", "verification"}

	// ---- one named case per gap class ----

	cases := []struct {
		class string
		why   string
		found func() (string, bool)
	}{
		{
			class: "isolated (a node with zero edges of any type)",
			why:   "makes the smallest thing on screen mean something",
			found: func() (string, bool) {
				for _, n := range p.Nodes {
					if degree[n.ID] == 0 {
						return n.ID, true
					}
				}
				return "", false
			},
		},
		{
			class: "weakly_linked (a node with exactly one edge)",
			why:   "the class immediately above isolated, and the one most often a real oversight",
			found: func() (string, bool) {
				for _, n := range p.Nodes {
					if degree[n.ID] == 1 {
						return n.ID, true
					}
				}
				return "", false
			},
		},
		{
			class: "sink_group (a module with cross-module outbound edges and zero inbound)",
			why:   "a module nothing in the project links back into",
			found: func() (string, bool) {
				for _, m := range p.Groups.Modules {
					if crossOut[m] > 0 && crossIn[m] == 0 {
						return m, true
					}
				}
				return "", false
			},
		},
		{
			class: "orphan_group (a module with no cross-module edge in either direction)",
			why:   "a module wired to nothing at all — invisible on a card-by-card read",
			found: func() (string, bool) {
				for _, m := range p.Groups.Modules {
					if crossOut[m] == 0 && crossIn[m] == 0 {
						return m, true
					}
				}
				return "", false
			},
		},
		{
			class: "review_pending (an approved claim the engine has flagged)",
			why:   "an engine-managed state that demands human action, drawn as a halo",
			found: func() (string, bool) {
				for _, n := range p.Nodes {
					if n.ReviewPending {
						return n.ID, true
					}
				}
				return "", false
			},
		},
		{
			class: "open_threads (a node with an unresolved comment thread)",
			why:   "the other halo state",
			found: func() (string, bool) {
				for _, n := range p.Nodes {
					if n.OpenComments > 0 {
						return n.ID, true
					}
				}
				return "", false
			},
		},
		{
			class: "a draft node",
			why:   "the dashed ring; without one, the ring channel encodes nothing",
			found: func() (string, bool) {
				for _, n := range p.Nodes {
					if n.Status == "draft" {
						return n.ID, true
					}
				}
				return "", false
			},
		},
		{
			class: "a locked node",
			why:   "the solid ring, and the precondition for the missing-phase heuristic",
			found: func() (string, bool) {
				for _, n := range p.Nodes {
					if n.Status == "locked" {
						return n.ID, true
					}
				}
				return "", false
			},
		},
		{
			class: "a governed node whose governor also governs something else",
			why:   "a governor with one governed claim proves nothing about the wedge marker or the governance overlay",
			found: func() (string, bool) {
				for _, e := range p.Edges {
					if e.Type == EdgeGovernedBy && governs[e.To] >= 2 {
						return e.From + " -> " + e.To, true
					}
				}
				return "", false
			},
		},
		{
			class: "a module with an approved claim and no claim in some build phase",
			why:   "the missing_build_phase heuristic; verification is the usual absentee",
			found: func() (string, bool) {
				for _, m := range p.Groups.Modules {
					if !hasLocked[m] {
						continue
					}
					for _, ph := range phases {
						if !phasesSeen[m][ph] {
							return m + " has no " + ph + " claim", true
						}
					}
				}
				return "", false
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.class, func(t *testing.T) {
			got, ok := tc.found()
			if !ok {
				t.Fatalf("the demo fixture no longer seeds this class.\n  class: %s\n  why it is seeded: %s\nAn edit to testdata/fixture-graph-demo removed the only instance; restore one rather than deleting this case.", tc.class, tc.why)
			}
			t.Logf("seeded by: %s", got)
		})
	}

	// ---- and the one thing the corpus must NOT contain ----

	t.Run("no cycle of any shape", func(t *testing.T) {
		// "dossierx check" returns above the catalog and render stages on the
		// first error-severity finding, and a rests_on loop, a governed_by
		// loop and a loop alternating the two are all error severity. So a
		// fixture that renders at all cannot carry one — and if this fixture
		// ever did, it would stop rendering rather than fail here. Asserting
		// it anyway states the property at the level the PANE cares about,
		// which is the payload's edge set, and it is what makes the empty
		// cycle block a reader sees the correct reading rather than a bug.
		shapes := []struct {
			name  string
			types map[string]bool
		}{
			{"rests_on only", map[string]bool{EdgeRestsOn: true}},
			{"governed_by only", map[string]bool{EdgeGovernedBy: true}},
			{"the union of both — the shape neither single-type rule can see", map[string]bool{EdgeRestsOn: true, EdgeGovernedBy: true}},
		}
		for _, sh := range shapes {
			adj := map[string][]string{}
			edgeCount := 0
			for _, e := range p.Edges {
				if !sh.types[e.Type] {
					continue
				}
				adj[e.From] = append(adj[e.From], e.To)
				edgeCount++
			}
			if edgeCount == 0 {
				t.Errorf("%s: no edges of these types in the payload — this shape's check would pass vacuously", sh.name)
				continue
			}
			for _, ids := range adj {
				sort.Strings(ids)
			}
			if cyc := findCycle(p.Nodes, adj); cyc != nil {
				t.Errorf("%s: cycle found: %s", sh.name, strings.Join(cyc, " -> "))
			}
		}
	})

	t.Run("no self-edge of any type", func(t *testing.T) {
		for _, e := range p.Edges {
			if e.From == e.To {
				t.Errorf("self-edge: %s -(%s)-> %s", e.From, e.Type, e.To)
			}
		}
	})
}

// TestFindCycleControl is the reason the "no cycle of any shape" assertion
// above means anything. That assertion's whole content is that findCycle
// returned nil, and a findCycle that ALWAYS returned nil would satisfy it
// forever while checking nothing. So the detector is exercised here against
// the shapes it exists to catch, including the mixed-type loop that is
// invisible to a walk over either edge type alone.
func TestFindCycleControl(t *testing.T) {
	nodesOf := func(ids ...string) []Node {
		out := make([]Node, 0, len(ids))
		for _, id := range ids {
			out = append(out, Node{ID: id})
		}
		return out
	}

	t.Run("a two-node loop is found", func(t *testing.T) {
		adj := map[string][]string{"a": {"b"}, "b": {"a"}}
		if cyc := findCycle(nodesOf("a", "b"), adj); cyc == nil {
			t.Error("findCycle returned nil on a 2-cycle")
		}
	})

	t.Run("a self-edge is found", func(t *testing.T) {
		adj := map[string][]string{"a": {"a"}}
		if cyc := findCycle(nodesOf("a"), adj); cyc == nil {
			t.Error("findCycle returned nil on a self-edge")
		}
	})

	t.Run("a loop reachable only through a longer path is found", func(t *testing.T) {
		adj := map[string][]string{"a": {"b"}, "b": {"c"}, "c": {"d"}, "d": {"b"}}
		if cyc := findCycle(nodesOf("a", "b", "c", "d"), adj); cyc == nil {
			t.Error("findCycle returned nil on a 3-cycle hanging off a chain")
		}
	})

	t.Run("a diamond is not a cycle", func(t *testing.T) {
		// Two paths reaching the same node is the shape a naive
		// already-visited check misreports as a loop.
		adj := map[string][]string{"a": {"b", "c"}, "b": {"d"}, "c": {"d"}}
		if cyc := findCycle(nodesOf("a", "b", "c", "d"), adj); cyc != nil {
			t.Errorf("findCycle reported a cycle in a diamond: %v", cyc)
		}
	})

	t.Run("a long chain terminates without recursing", func(t *testing.T) {
		// The engine's own cycle lints are iterative because nothing bounds
		// an authored edge chain's length. This one is too, and this is the
		// case that proves it.
		const n = 100000
		nodes := make([]Node, 0, n)
		adj := make(map[string][]string, n)
		for i := range n {
			id := "n" + itoa(i)
			nodes = append(nodes, Node{ID: id})
			if i+1 < n {
				adj[id] = []string{"n" + itoa(i+1)}
			}
		}
		if cyc := findCycle(nodes, adj); cyc != nil {
			t.Errorf("findCycle reported a cycle in a %d-node chain: %v", n, cyc)
		}
	})
}

// itoa avoids pulling strconv in for one call site in a test helper.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

// findCycle returns one directed cycle in adj as a node path, or nil when the
// graph is acyclic.
//
// It is ITERATIVE over an explicit frame stack rather than recursive, for the
// same reason the engine's own cycle lints are: depth here is the length of
// the longest authored edge chain, and nothing bounds that. Nodes are visited
// in payload order (already sorted by id) and each node's successors in
// sorted order, so the cycle it names is the same one on every run.
func findCycle(nodes []Node, adj map[string][]string) []string {
	const (
		white = iota // unvisited
		grey         // on the current path
		black        // fully explored
	)
	colour := make(map[string]int, len(nodes))

	type frame struct {
		id   string
		next int
	}

	for _, start := range nodes {
		if colour[start.ID] != white {
			continue
		}
		stack := []frame{{id: start.ID}}
		colour[start.ID] = grey
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			succs := adj[top.id]
			if top.next >= len(succs) {
				colour[top.id] = black
				stack = stack[:len(stack)-1]
				continue
			}
			to := succs[top.next]
			top.next++
			switch colour[to] {
			case grey:
				// A back edge to something on the current path: report the
				// path from that node onward, closed by the repeated node.
				path := []string{}
				seen := false
				for _, f := range stack {
					if f.id == to {
						seen = true
					}
					if seen {
						path = append(path, f.id)
					}
				}
				return append(path, to)
			case white:
				colour[to] = grey
				stack = append(stack, frame{id: to})
			}
		}
	}
	return nil
}
