package loader

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// FuzzCommentBodyRoundTrip proves the YAML-fidelity risk the design flags: a
// comment body is user-authored free text, and loader.SaveClaim marshals the
// WHOLE claim struct and atomically replaces the file, so a body that survives
// Marshal->decode only lossily would silently corrupt review discussion.
//
// The seed corpus is every hostile category the spec enumerates — quotes,
// colons, a leading "-", the "---" document marker, tabs, unicode, emoji,
// CRLF, and very long text — carried on BOTH a thread root and a reply. The
// seeds execute on every normal `go test` run (Go fuzz runs its corpus as
// subtests); the `-fuzz` mode is a separate one-off exploration.
//
// Note: a body that BEGINS with a newline (or is only whitespace) is a known
// yaml.v3 block-scalar limitation (leading line breaks are not preserved) and
// is deliberately out of this contract — such bodies are unreachable through
// the internal/comments ops, which reject whitespace-only bodies outright.
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

		if err := SaveClaim(original); err != nil {
			t.Fatalf("SaveClaim: %v (body=%q)", err, body)
		}
		loaded, err := LoadClaims(dir)
		if err != nil {
			t.Fatalf("LoadClaims: %v (body=%q)", err, body)
		}
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
