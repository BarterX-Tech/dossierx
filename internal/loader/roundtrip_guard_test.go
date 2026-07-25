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
			got, _ := os.ReadFile(path)
			if string(got) != string(good) {
				t.Fatalf("refused SaveClaimIfUnchanged(body=%q) clobbered the good file:\n%s", body, got)
			}
			if _, loadErr := LoadClaims(dir); loadErr != nil {
				t.Fatalf("claims dir failed to load after a refused SaveClaimIfUnchanged(body=%q): %v", body, loadErr)
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
