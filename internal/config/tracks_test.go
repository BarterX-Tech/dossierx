// tracks_test.go covers the `tracks:` registry — the declared vocabulary a
// claim's cross-cutting membership is checked against. The tests come in two
// halves: the registry must reject an entry that could never do its job (no
// id, no title, a duplicate id), and a project that declares no tracks must
// keep behaving exactly as it did before the field existed.
package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// trackConfig builds a minimal valid config with the supplied tracks block
// appended, so each test below varies exactly one thing.
func trackConfig(t *testing.T, tracksBlock string) (*Config, error) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := writeConfig(t, dir, "project.config.yaml", `
schema_version: 1
facets: [contract, internals]
modules: [payment, inventory]
claims_dir: claims
`+tracksBlock)
	return LoadConfig(p)
}

func TestLoadConfig_TracksValid(t *testing.T) {
	cfg, err := trackConfig(t, `
tracks:
  - id: checkout
    title: Checkout
    summary: What a buyer experiences from cart to confirmation.
  - id: refunds
    title: Refunds
`)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if len(cfg.Tracks) != 2 {
		t.Fatalf("len(Tracks) = %d, want 2", len(cfg.Tracks))
	}
	if cfg.Tracks[0].ID != "checkout" || cfg.Tracks[0].Title != "Checkout" {
		t.Errorf("Tracks[0] = %+v, want checkout/Checkout", cfg.Tracks[0])
	}
	// summary is genuinely optional — a well-named track with an owned claim
	// already says what it is.
	if cfg.Tracks[1].Summary != "" {
		t.Errorf("Tracks[1].Summary = %q, want empty when omitted", cfg.Tracks[1].Summary)
	}

	// TrackIDs must preserve DECLARATION order, not sort: config order is the
	// order the viewer's sidebar and `track list` present tracks in, and a
	// project that has arranged its features deliberately must not have that
	// arrangement silently alphabetized.
	got := strings.Join(cfg.TrackIDs(), ",")
	if got != "checkout,refunds" {
		t.Errorf("TrackIDs() = %q, want %q in declaration order", got, "checkout,refunds")
	}

	if !cfg.HasTrack("checkout") || !cfg.HasTrack("refunds") {
		t.Error("HasTrack returned false for a declared track")
	}
	if cfg.HasTrack("checkout ") || cfg.HasTrack("Checkout") || cfg.HasTrack("nope") {
		t.Error("HasTrack matched something other than an exact declared id; the registry exists to catch typos, so it must not be forgiving about them")
	}

	tr, ok := cfg.TrackByID("checkout")
	if !ok || tr.Title != "Checkout" {
		t.Errorf("TrackByID(checkout) = %+v, %v; want the declared track", tr, ok)
	}
	if _, ok := cfg.TrackByID("nope"); ok {
		t.Error("TrackByID reported an undeclared track as found")
	}
}

// TestLoadConfig_TracksUnsetIsFine is the zero-cost-when-unused contract: a
// project that predates tracks, or simply does not use them, must load and
// behave exactly as before. Every track-* lint is a no-op on it and the
// viewer renders no Tracks group.
func TestLoadConfig_TracksUnsetIsFine(t *testing.T) {
	cfg, err := trackConfig(t, "")
	if err != nil {
		t.Fatalf("LoadConfig with no tracks: %v", err)
	}
	if cfg.Tracks != nil {
		t.Errorf("Tracks = %+v, want nil when omitted", cfg.Tracks)
	}
	if len(cfg.TrackIDs()) != 0 {
		t.Errorf("TrackIDs() = %v, want empty", cfg.TrackIDs())
	}
	if cfg.HasTrack("anything") {
		t.Error("HasTrack returned true on a project that declares no tracks")
	}
}

func TestLoadConfig_TrackWithoutIDRejected(t *testing.T) {
	_, err := trackConfig(t, `
tracks:
  - title: Checkout
`)
	if err == nil {
		t.Fatal("expected an error for a track with no id")
	}
	if !strings.Contains(err.Error(), "id") {
		t.Errorf("error %q does not mention the missing id", err)
	}
}

// TestLoadConfig_TrackWithoutTitleRejected: a track exists to be read about
// by someone asking "what does the user get", and an id is not an answer to
// that question. Rejecting at load keeps the viewer and `track list` from
// having to invent a display name.
func TestLoadConfig_TrackWithoutTitleRejected(t *testing.T) {
	_, err := trackConfig(t, `
tracks:
  - id: checkout
`)
	if err == nil {
		t.Fatal("expected an error for a track with no title")
	}
	if !strings.Contains(err.Error(), "title") {
		t.Errorf("error %q does not mention the missing title", err)
	}
}

// TestLoadConfig_DuplicateTrackIDsRejected. Two entries competing for one id
// would make TrackByID's answer depend on declaration order and would let a
// claim's membership resolve to whichever the reader happened not to mean —
// the same reasoning that rejects duplicate modules and facets.
func TestLoadConfig_DuplicateTrackIDsRejected(t *testing.T) {
	_, err := trackConfig(t, `
tracks:
  - id: checkout
    title: Checkout
  - id: checkout
    title: Checkout Again
`)
	if err == nil {
		t.Fatal("expected an error for duplicate track ids")
	}
	if !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("error %q does not say the ids were duplicated", err)
	}
}

func TestLoadConfig_BlankTrackIDRejected(t *testing.T) {
	_, err := trackConfig(t, `
tracks:
  - id: "   "
    title: Checkout
`)
	if err == nil {
		t.Fatal("expected an error for a whitespace-only track id")
	}
}

// TestLoadConfig_TracksAreStrictlyDecoded confirms the tracks block is
// covered by the config's strict decode like everything else: an unknown key
// inside a track entry is a load error, not a silently ignored typo. Without
// this, a misspelled summary key would be dropped on the floor and the track
// would render with no summary and no complaint.
//
// The fixture below misspells `summary` ON PURPOSE, which is why the three
// lines that carry the typo are exempted from the spell checker: it is the
// input, not a mistake in the test.
func TestLoadConfig_TracksAreStrictlyDecoded(t *testing.T) {
	// Spelled as a constant rather than inline so the one occurrence of the
	// typo sits on a line the spell checker can be told about; inside the raw
	// string below there is nowhere to put the directive that would not become
	// part of the fixture.
	const typoKey = "sumary" //nolint:misspell // the misspelling IS the fixture

	_, err := trackConfig(t, `
tracks:
  - id: checkout
    title: Checkout
    `+typoKey+`: a typo
`)
	if err == nil {
		t.Fatal("expected an unknown field inside a track entry to be rejected")
	}
	if !strings.Contains(err.Error(), typoKey) {
		t.Errorf("error %q does not name the unknown field", err)
	}
}
