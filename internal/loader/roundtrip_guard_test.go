package loader

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// The save path is the systemic backstop against a store-bricking write: a
// comment body whose leading whitespace makes yaml.v3 v3.0.1 emit a block
// scalar it cannot re-parse (a leading newline, tab line, or blank line) would,
// if persisted verbatim, make the NEXT LoadClaims of the whole dir fail. These
// tests pin that SaveClaim / SaveClaimIfUnchanged REFUSE such a write (returning
// ErrClaimNotRoundTrippable) rather than persisting it, so no writer can brick
// the store — and prove the dir stays loadable after a refused write.

// brickingBodies are the exact leading-whitespace categories that make
// yaml.v3 v3.0.1 emit an unparseable block scalar (reproduced against v3.0.1).
var brickingBodies = []string{
	"\ncontract says X", // leading newline + real content -> body: |4-
	"\n0",               // leading newline + minimal content
	"\t\nreal content",  // leading tab line -> tab-in-indent
	"\n\nleading blank lines\ncontent",
}

func brickClaim(path, body string) model.Claim {
	return model.Claim{
		ID:       "widget.contract.a",
		Facet:    "contract",
		Module:   "widget",
		Status:   model.StatusDraft,
		Body:     "base claim body",
		Governed: model.Governed{Type: "none", Reason: "fixture"},
		Comments: []model.Comment{{
			ID:      "c-000001",
			Status:  model.CommentStatusOpen,
			Author:  model.CommentRoleHuman,
			Created: "2026-07-24T10:12:00Z",
			Body:    body,
			Replies: []model.Reply{{
				ID:      "r-000001",
				Author:  model.CommentRoleAgent,
				Created: "2026-07-24T10:40:00Z",
				Body:    body,
			}},
		}},
		SourcePath: path,
	}
}

func TestSaveClaim_RefusesStoreBrickingBody(t *testing.T) {
	for _, body := range brickingBodies {
		t.Run(body, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "a.yaml")

			err := SaveClaim(brickClaim(path, body))
			if !errors.Is(err, ErrClaimNotRoundTrippable) {
				t.Fatalf("SaveClaim(body=%q): want ErrClaimNotRoundTrippable, got %v", body, err)
			}
			// A refused write must not have left a file at all here (first save),
			// and crucially the dir must still LOAD — the whole point of the guard.
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Fatalf("SaveClaim(body=%q) refused but a file was written: stat err=%v", body, statErr)
			}
			if _, loadErr := LoadClaims(dir); loadErr != nil {
				t.Fatalf("claims dir failed to load after a refused SaveClaim(body=%q): %v", body, loadErr)
			}
		})
	}
}

func TestSaveClaimIfUnchanged_RefusesStoreBrickingBody(t *testing.T) {
	for _, body := range brickingBodies {
		t.Run(body, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "a.yaml")
			// Seed a valid, loadable claim file first so we can prove the refused
			// overwrite leaves the ORIGINAL good bytes intact (not a brick).
			good := []byte("id: widget.contract.a\nfacet: contract\nstatus: draft\nbody: good\n")
			if err := os.WriteFile(path, good, 0o644); err != nil {
				t.Fatalf("seed: %v", err)
			}
			token, err := CaptureClaimFileToken(path)
			if err != nil {
				t.Fatalf("capture token: %v", err)
			}

			err = SaveClaimIfUnchanged(brickClaim(path, body), token)
			if !errors.Is(err, ErrClaimNotRoundTrippable) {
				t.Fatalf("SaveClaimIfUnchanged(body=%q): want ErrClaimNotRoundTrippable, got %v", body, err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(good) {
				t.Fatalf("refused SaveClaimIfUnchanged(body=%q) clobbered the good file:\n%s", body, got)
			}
			if _, loadErr := LoadClaims(dir); loadErr != nil {
				t.Fatalf("claims dir failed to load after a refused SaveClaimIfUnchanged(body=%q): %v", body, loadErr)
			}
		})
	}
}

// bodyRoundTripCase is one probe body plus whether it is expected to survive
// the marshal + strict-decode round-trip. The reject set is the exact
// store-bricking class yaml.v3 v3.0.1 produces (reproduced against v3.0.1): a
// bare leading newline, a leading blank/whitespace-only line, a tab-only first
// line, AND — the class the old leading-whitespace heuristic MISSED — a first
// CONTENT line that itself begins with a TAB. The accept set includes bodies the
// old heuristic FALSE-REJECTED (" \n…", "\r\n…", a NBSP/NEL/VT/FF-led first
// line) even though they round-trip cleanly, and — since v0.4.0 (T6) — a first
// CONTENT line indented with SPACES, which emitting at claimYAMLIndent stores
// back byte-exact.
func bodyRoundTripCases() []struct {
	name string
	body string
	want bool
} {
	nbsp := string(rune(0x00A0))
	nel := string(rune(0x0085))
	vt := string(rune(0x000B))
	ff := string(rune(0x000C))
	return []struct {
		name string
		body string
		want bool
	}{
		// --- REJECT (does not round-trip; would brick the store) ---
		{"bare-leading-newline", "\ncontract says X", false},
		{"leading-newline-minimal", "\n0", false},
		{"tab-only-first-line", "\t\nreal content", false},
		{"leading-blank-lines", "\n\nleading blank lines\ncontent", false},
		{"tab-led-first-content-line", "\tcode\nmore", false},
		// --- ACCEPT (round-trips; must NOT be rejected) ---
		// Space-indented first content lines moved here in v0.4.0 (T6): emitting at
		// claimYAMLIndent (2) stores them back byte-exact. A TAB-led one still does not.
		{"space-indented-first-content-line", "    func main(){}\n    return", true},
		{"two-space-indented-multiline", "  a\n  b", true},
		{"space-then-newline", " \ncontent", true},
		{"crlf-lead", "\r\ncontent", true},
		{"nbsp-then-newline", nbsp + "\ncontent", true},
		{"nel-then-newline", nel + "\ncontent", true},
		{"vt-then-newline", vt + "\ncontent", true},
		{"ff-then-newline", ff + "\ncontent", true},
		{"plain", "plain body", true},
		{"normal-multiline", "multi\nline\nbody\nhere", true},
		{"interior-tab", "tab\tin\tthe\tmiddle", true},
		{"leading-tab-single-line", "\tleading tab but single line", true},
		{"trailing-newline", "trailing newline\n", true},
		{"windows-endings", "windows\r\nline\r\nendings", true},
		{"unicode", "unicode café — ☕ 你好 Ω \U0001f389", true},
	}
}

// TestCommentBodyRoundTrips_MatchesExpectedBrickClass pins the shared pre-check
// against the empirically-established yaml.v3 v3.0.1 brick class.
func TestCommentBodyRoundTrips_MatchesExpectedBrickClass(t *testing.T) {
	for _, c := range bodyRoundTripCases() {
		t.Run(c.name, func(t *testing.T) {
			if got := CommentBodyRoundTrips(c.body); got != c.want {
				t.Fatalf("CommentBodyRoundTrips(%q) = %v, want %v", c.body, got, c.want)
			}
		})
	}
}

// TestCommentBodyRoundTrips_AgreesWithSaveGuard proves the pre-check matches the
// actual save-time round-trip guard BY CONSTRUCTION: for every probe body,
// CommentBodyRoundTrips must agree with whether SaveClaim (which runs
// verifyRoundTrip over the full claim, body in BOTH a thread and a reply body)
// accepts a claim carrying it. If these ever diverge, a body could pass the
// input pre-check yet be refused at save (a leaked round-trip error) or vice
// versa — exactly the inconsistency this fix removes.
func TestCommentBodyRoundTrips_AgreesWithSaveGuard(t *testing.T) {
	for _, c := range bodyRoundTripCases() {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "a.yaml")
			saveErr := SaveClaim(brickClaim(path, c.body))
			saveOK := saveErr == nil
			if saveErr != nil && !errors.Is(saveErr, ErrClaimNotRoundTrippable) {
				t.Fatalf("SaveClaim(body=%q): unexpected error kind: %v", c.body, saveErr)
			}
			if pre := CommentBodyRoundTrips(c.body); pre != saveOK {
				t.Fatalf("pre-check/save-guard DISAGREE for body=%q: CommentBodyRoundTrips=%v, SaveClaim-accepts=%v", c.body, pre, saveOK)
			}
		})
	}
}

// The guard must NOT reject valid bodies (interior newlines, interior/leading
// tabs without a following newline, trailing newline, unicode) — those all
// round-trip and must still save exactly as before.
func TestSaveClaim_RoundTripGuard_AllowsValidBodies(t *testing.T) {
	valid := []string{
		"plain body",
		"multi\nline\nbody\nhere",
		"tab\tin\tthe\tmiddle",
		"\tleading tab but single line",
		"trailing newline\n",
		"unicode café — ☕ 你好 Ω 🎉",
		"windows\r\nline\r\nendings",
	}
	for _, body := range valid {
		t.Run(body, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "a.yaml")
			if err := SaveClaim(brickClaim(path, body)); err != nil {
				t.Fatalf("SaveClaim(valid body=%q) unexpectedly failed: %v", body, err)
			}
			loaded, err := LoadClaims(dir)
			if err != nil {
				t.Fatalf("LoadClaims(valid body=%q): %v", body, err)
			}
			if len(loaded) != 1 || len(loaded[0].Comments) != 1 || loaded[0].Comments[0].Body != body {
				t.Fatalf("valid body=%q did not round-trip: %+v", body, loaded)
			}
		})
	}
}
