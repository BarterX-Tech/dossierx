package loader

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// FuzzCommentBodyRoundTrip proves the YAML-fidelity contract the design flags: a
// comment body is user-authored free text, and loader.SaveClaim marshals the
// WHOLE claim struct and atomically replaces the file, so a body that Marshal ->
// decode mangles would silently corrupt review discussion — and, worse, a body
// yaml.v3 v3.0.1 emits as an unparseable block scalar (a leading newline / tab
// line / blank line) would BRICK the whole claims dir on the next load.
//
// Post-fix contract, pinned here for EVERY body: SaveClaim either
//
//   - REFUSES the write with loader.ErrClaimNotRoundTrippable (the systemic
//     round-trip guard caught a body it cannot faithfully store), leaving the
//     dir loadable and unbricked; or
//   - SUCCEEDS, and then every thread and reply body round-trips byte-exact.
//
// It is NEVER allowed to persist a file the next LoadClaims cannot parse. The
// seeds carry each hostile category on BOTH a thread root and a reply; the
// seeds execute on every normal `go test` run (Go fuzz runs its corpus as
// subtests), and `-fuzz` is a separate one-off exploration.
func FuzzCommentBodyRoundTrip(f *testing.F) {
	seeds := []string{
		"a plain body",
		"with: an inline colon",
		`"double quoted"`,
		"'single quoted'",
		"- leading dash looks like a list item",
		"--- yaml document marker",
		"tab\tin\tthe\tmiddle",
		"unicode café — ☕ 你好 Ω",
		"emoji 🎉🔥🧪✅",
		"windows\r\nline\r\nendings",
		"trailing spaces at eol   ",
		"multi\nline\nbody\nhere",
		"#hash at start",
		"key: value\nnested: like\na: map",
		"back\\slash and a literal \\n sequence",
		strings.Repeat("very long body ", 4000),
		// Store-bricking leading-whitespace bodies (real content, NOT
		// whitespace-only) that yaml.v3 v3.0.1 emits as an unparseable block
		// scalar — the exact regression that motivated the round-trip guard.
		// Carried on both a thread root and a reply by the harness below. The
		// post-fix contract for each is: a clean ErrClaimNotRoundTrippable
		// refusal, never a bricked dir.
		"\ncontract says X", // leading newline + content -> body: |4-
		"\n0",               // leading newline + minimal content
		"\t\nreal content",  // leading tab line -> tab-in-indent
		"\n\nblank\nlines then content",
		" \nleading whitespace-only first line then content",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, body string) {
		dir := t.TempDir()
		path := filepath.Join(dir, "a.yaml")
		original := model.Claim{
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
				Edited:  false,
				Replies: []model.Reply{{
					ID:      "r-000001",
					Author:  model.CommentRoleAgent,
					Created: "2026-07-24T10:40:00Z",
					Body:    body,
					Edited:  false,
				}},
			}},
			SourcePath: path,
		}

		saveErr := SaveClaim(original)

		// Whatever SaveClaim decided, the claims dir MUST still load — an
		// unparseable, bricked file is the failure this whole contract guards
		// against.
		loaded, loadErr := LoadClaims(dir)
		if loadErr != nil {
			t.Fatalf("claims dir failed to reload after SaveClaim (body=%q): %v", body, loadErr)
		}

		if saveErr != nil {
			// The only acceptable failure is the systemic round-trip guard
			// refusing a body it cannot faithfully store. A refused first save
			// writes nothing, so the dir is empty (and, above, still loadable).
			if !errors.Is(saveErr, ErrClaimNotRoundTrippable) {
				t.Fatalf("SaveClaim failed with an unexpected error (body=%q): %v", body, saveErr)
			}
			if len(loaded) != 0 {
				t.Fatalf("SaveClaim refused (body=%q) but a claim was written: %d loaded", body, len(loaded))
			}
			return
		}

		// SaveClaim succeeded -> both bodies MUST round-trip byte-exact.
		if len(loaded) != 1 || len(loaded[0].Comments) != 1 {
			t.Fatalf("expected 1 claim with 1 comment, got %d claims (body=%q)", len(loaded), body)
		}
		gotThread := loaded[0].Comments[0].Body
		if gotThread != body {
			t.Fatalf("thread body did not round-trip byte-exact:\n want %q\n  got %q", body, gotThread)
		}
		if len(loaded[0].Comments[0].Replies) != 1 {
			t.Fatalf("expected 1 reply after round-trip, got %d (body=%q)", len(loaded[0].Comments[0].Replies), body)
		}
		gotReply := loaded[0].Comments[0].Replies[0].Body
		if gotReply != body {
			t.Fatalf("reply body did not round-trip byte-exact:\n want %q\n  got %q", body, gotReply)
		}
	})
}
