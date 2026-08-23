package graph

// The track axis, on the wire.
//
// Tracks are the second axis a claim may join — many-to-many where module is
// one-to-one — and the graph pane carries them so a reviewer can look at one
// feature's subgraph instead of the whole registry. Three properties are worth
// an executable check rather than a comment, and this file is all three:
//
//  1. MEMBERSHIP IS NOT AN EDGE. It rides on the node. An edge would join the
//     client's scc() walk and ring every claim in a track red.
//  2. THE ROLE IS RESOLVED HERE. "" means cites, and the payload never makes a
//     client re-derive that.
//  3. ZERO COST WHEN UNUSED. A project that declares no tracks emits neither
//     new key — byte-identical output, which is what three tracked fixture
//     viewers rest on.

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// trackClaim is a minimal claim carrying memberships, so a table case reads as
// the memberships it is about and nothing else.
func trackClaim(id string, tracks ...model.TrackRef) model.Claim {
	return model.Claim{
		ID: id, Module: "widget", Facet: "contract",
		Status: model.StatusDraft, Tracks: tracks,
	}
}

func TestBuildNodeTracks(t *testing.T) {
	cfg := &config.Config{
		Modules: []string{"widget"},
		Facets:  []string{"contract"},
		Tracks: []config.Track{
			{ID: "checkout", Title: "Checkout"},
			{ID: "search", Title: "Search"},
		},
	}

	cases := []struct {
		name  string
		claim model.Claim
		want  []NodeTrack
	}{
		{
			name:  "no memberships emits nothing at all",
			claim: trackClaim("widget.contract.plain"),
			want:  nil,
		},
		{
			// The unset role MEANS cites (model.TrackRoleCites). A client
			// re-deriving that default would be a second place to keep it
			// correct, so it is resolved before it reaches the wire.
			name:  "an unset role resolves to cites",
			claim: trackClaim("widget.contract.unset", model.TrackRef{ID: "checkout"}),
			want:  []NodeTrack{{ID: "checkout", Role: "cites"}},
		},
		{
			name: "owns is carried verbatim",
			claim: trackClaim("widget.contract.owner",
				model.TrackRef{ID: "checkout", Role: model.TrackRoleOwns}),
			want: []NodeTrack{{ID: "checkout", Role: "owns"}},
		},
		{
			// Declaration order, not sorted: a claim that wrongly owns two
			// tracks is resolved by model.Claim.OwnedTrackID taking the
			// FIRST, and the payload has to name the same one rather than
			// whichever sorts lower.
			name: "declaration order survives",
			claim: trackClaim("widget.contract.both",
				model.TrackRef{ID: "search", Role: model.TrackRoleOwns},
				model.TrackRef{ID: "checkout"}),
			want: []NodeTrack{{ID: "search", Role: "owns"}, {ID: "checkout", Role: "cites"}},
		},
		{
			// A membership naming no track selects nothing and cannot be
			// filtered on — the same reason the empty string is never a
			// module group. The claim keeps its node.
			name: "a membership with no id is dropped",
			claim: trackClaim("widget.contract.blank",
				model.TrackRef{ID: ""}, model.TrackRef{ID: "checkout"}),
			want: []NodeTrack{{ID: "checkout", Role: "cites"}},
		},
		{
			name: "a claim whose only membership has no id emits nothing",
			claim: trackClaim("widget.contract.onlyblank",
				model.TrackRef{ID: "", Role: model.TrackRoleOwns}),
			want: nil,
		},
		{
			// serve never lints, so a claim naming a track the config does
			// not declare reaches the payload routinely during authoring.
			// Dropping it would hide a membership the node genuinely has.
			name: "an undeclared track id is carried, not dropped",
			claim: trackClaim("widget.contract.unknown",
				model.TrackRef{ID: "not-in-config"}),
			want: []NodeTrack{{ID: "not-in-config", Role: "cites"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := nodeByID(t, buildFrom(t, cfg, tc.claim), tc.claim.ID)
			if len(n.Tracks) != len(tc.want) {
				t.Fatalf("tracks = %+v, want %+v", n.Tracks, tc.want)
			}
			for i := range tc.want {
				if n.Tracks[i] != tc.want[i] {
					t.Fatalf("tracks = %+v, want %+v", n.Tracks, tc.want)
				}
			}
		})
	}
}

// TestTrackMembershipIsNeverAnEdge is the executable half of the decision
// model.TrackRef's doc comment states: a track is a set, and a set has no
// direction to run in a circle. If membership ever became an Edge, the
// client's scc() would walk it and report a dependency cycle over claims that
// merely share a feature.
func TestTrackMembershipIsNeverAnEdge(t *testing.T) {
	cfg := &config.Config{
		Modules: []string{"widget"}, Facets: []string{"contract"},
		Tracks: []config.Track{{ID: "checkout", Title: "Checkout"}},
	}
	p := buildFrom(t, cfg,
		trackClaim("widget.contract.a", model.TrackRef{ID: "checkout", Role: model.TrackRoleOwns}),
		trackClaim("widget.contract.b", model.TrackRef{ID: "checkout"}),
		trackClaim("widget.contract.c", model.TrackRef{ID: "checkout"}),
	)
	if len(p.Edges) != 0 {
		t.Fatalf("edges = %+v, want none — three claims in one track declare no relation between each other", p.Edges)
	}
	for _, n := range p.Nodes {
		if n.InDegree != 0 || n.OutDegree != 0 {
			t.Fatalf("%s has degree %d/%d; membership must not count as an edge", n.ID, n.InDegree, n.OutDegree)
		}
	}
}

func TestTrackGroupsOrder(t *testing.T) {
	cases := []struct {
		name   string
		cfg    *config.Config
		claims []model.Claim
		want   []TrackGroup
	}{
		{
			name: "no tracks anywhere yields no group list",
			cfg:  &config.Config{Modules: []string{"widget"}, Facets: []string{"contract"}},
			claims: []model.Claim{
				trackClaim("widget.contract.plain"),
			},
			want: nil,
		},
		{
			// Declared order, like modules and facets: it is the order the
			// project wrote and the order its sidebar reads.
			name: "declared order is the config's own",
			cfg: &config.Config{
				Modules: []string{"widget"}, Facets: []string{"contract"},
				Tracks: []config.Track{
					{ID: "search", Title: "Search"},
					{ID: "checkout", Title: "Checkout"},
				},
			},
			claims: []model.Claim{trackClaim("widget.contract.plain")},
			want: []TrackGroup{
				{ID: "search", Title: "Search"},
				{ID: "checkout", Title: "Checkout"},
			},
		},
		{
			// A track a reader can see on a node and cannot select would be
			// worse than a raw id in the control.
			name: "an undeclared id lands after the declared ones, under its own id",
			cfg: &config.Config{
				Modules: []string{"widget"}, Facets: []string{"contract"},
				Tracks: []config.Track{{ID: "search", Title: "Search"}},
			},
			claims: []model.Claim{
				trackClaim("widget.contract.z", model.TrackRef{ID: "zulu"}),
				trackClaim("widget.contract.a", model.TrackRef{ID: "alpha"}),
			},
			want: []TrackGroup{
				{ID: "search", Title: "Search"},
				{ID: "alpha", Title: "alpha"},
				{ID: "zulu", Title: "zulu"},
			},
		},
		{
			// A declared track nobody has joined yet is still a track. The
			// filter offers it and the pane says the selection is empty,
			// which is the same answer a declared facet with no claims gets.
			name: "a declared track with no members is still listed",
			cfg: &config.Config{
				Modules: []string{"widget"}, Facets: []string{"contract"},
				Tracks: []config.Track{{ID: "unstarted", Title: "Unstarted"}},
			},
			claims: []model.Claim{trackClaim("widget.contract.plain")},
			want:   []TrackGroup{{ID: "unstarted", Title: "Unstarted"}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildFrom(t, tc.cfg, tc.claims...).Groups.Tracks
			if len(got) != len(tc.want) {
				t.Fatalf("groups.tracks = %+v, want %+v", got, tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Fatalf("groups.tracks = %+v, want %+v", got, tc.want)
				}
			}
		})
	}
}

// TestTrackLessPayloadIsByteIdenticalToPreTracks is the zero-cost contract,
// held as bytes rather than as an intention.
//
// Tracks are optional. A project that never declared one must get the payload
// it got before the axis existed — not a similar one, the same one. Three
// tracked fixture viewers carry this JSON inline, so a single extra
// "tracks":null per node would leave every one of them permanently dirty in
// the diff, and a reviewer would be reading noise to find the change that
// mattered.
//
// The check is the whole document, not the fields: any future key that
// defaults to a non-empty encoding fails here too, which is the point.
func TestTrackLessPayloadIsByteIdenticalToPreTracks(t *testing.T) {
	cfg := &config.Config{Modules: []string{"widget"}, Facets: []string{"contract"}}
	p := buildFrom(t, cfg,
		model.Claim{
			ID: "widget.contract.one", Module: "widget", Facet: "contract",
			Status: model.StatusLocked, BuildRole: model.BuildRoleAPI,
		},
	)
	b, err := Encode(p)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	const want = `{"schema":1,"generated_at":"","nodes":[{"id":"widget.contract.one",` +
		`"title":"One","module":"widget","facet":"contract","status":"locked","kind":"fact",` +
		`"build_role":"api","emphasis":false,"review_pending":false,"open_comments":0,` +
		`"in_degree":0,"out_degree":0}],"edges":[],` +
		`"groups":{"modules":["widget"],"facets":["contract"]},"dropped":{"unresolved_edges":0}}`
	if string(b) != want {
		t.Fatalf("track-less payload changed shape.\n got: %s\nwant: %s", b, want)
	}
}

// TestTrackedPayloadEncodesBothKeysOnce pins what the wire looks like once a
// project DOES opt in — the shape graph-ui.js's normalizePayload reads and the
// only description of it outside that file.
func TestTrackedPayloadEncodesBothKeys(t *testing.T) {
	cfg := &config.Config{
		Modules: []string{"widget"}, Facets: []string{"contract"},
		Tracks: []config.Track{{ID: "checkout", Title: "Checkout"}},
	}
	p := buildFrom(t, cfg,
		trackClaim("widget.contract.owner", model.TrackRef{ID: "checkout", Role: model.TrackRoleOwns}),
	)
	b, err := Encode(p)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	for _, want := range []string{
		`"tracks":[{"id":"checkout","role":"owns"}]`,
		`"tracks":[{"id":"checkout","title":"Checkout"}]`,
	} {
		if !strings.Contains(string(b), want) {
			t.Fatalf("payload does not contain %s\ngot: %s", want, b)
		}
	}

	// Two builds over the same corpus, byte-identical — the property three
	// tracked fixture viewers rest on, re-asserted over the new fields
	// because a map iteration reaching the output would only show up here.
	again, err := Encode(buildFrom(t, cfg,
		trackClaim("widget.contract.owner", model.TrackRef{ID: "checkout", Role: model.TrackRoleOwns}),
	))
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if string(again) != string(b) {
		t.Fatalf("two builds disagree:\n%s\n%s", b, again)
	}

	// And the shape survives a round trip, so the client is not reading a
	// document that only happens to look right as a string.
	var back Payload
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(back.Nodes) != 1 || len(back.Nodes[0].Tracks) != 1 || back.Nodes[0].Tracks[0].Role != "owns" {
		t.Fatalf("round trip lost the membership: %+v", back.Nodes)
	}
}
