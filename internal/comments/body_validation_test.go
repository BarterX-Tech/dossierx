package comments

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// A body with REAL content but a leading blank/whitespace-only line is the
// store-bricking category (yaml.v3 v3.0.1 emits an unparseable block scalar).
// It is distinct from a whitespace-ONLY body (ErrEmptyBody). Every mutating op
// that takes a body (add/reply/edit) must reject it up front with ErrUnsafeBody
// — a clean, specific error at the shared CLI+serve boundary — and write
// nothing, so the store is never even offered the bricking bytes.
var unsafeLeadingBodies = []string{
	"\ncontract says X", // leading newline + content
	"\n0",               // leading newline + minimal content
	"\t\nreal content",  // leading tab line + content
	"\n\nblank lines then content",
}

func TestValidateBody_RejectsUnsafeLeadingWhitespace(t *testing.T) {
	// Whitespace-only stays ErrEmptyBody (unchanged); leading-blank-line-with-
	// content becomes ErrUnsafeBody; ordinary and interior-newline bodies pass.
	cases := []struct {
		body string
		want error // nil means "accepted"
	}{
		{"", ErrEmptyBody},
		{"   ", ErrEmptyBody},
		{"\t\n", ErrEmptyBody}, // whitespace-only (has a newline but no content)
		{"\ncontract says X", ErrUnsafeBody},
		{"\n0", ErrUnsafeBody},
		{"\t\nreal content", ErrUnsafeBody},
		{"\n\nblank then content", ErrUnsafeBody},
		{"plain body", nil},
		{"multi\nline\nbody", nil},
		{"\tleading tab single line", nil}, // no newline -> yaml.v3 quotes it safely
		{"  leading spaces single line", nil},
		{"trailing newline\n", nil},
	}
	for _, tc := range cases {
		err := validateBody(tc.body)
		if tc.want == nil {
			if err != nil {
				t.Errorf("validateBody(%q) = %v, want accepted", tc.body, err)
			}
			continue
		}
		if !errors.Is(err, tc.want) {
			t.Errorf("validateBody(%q) = %v, want %v", tc.body, err, tc.want)
		}
	}
}

func TestAdd_RejectsUnsafeLeadingWhitespaceBody_NoWrite(t *testing.T) {
	for _, body := range unsafeLeadingBodies {
		t.Run(body, func(t *testing.T) {
			p := newProject(t, map[string]string{"a.yaml": draftAYAML})
			before, readErr := os.ReadFile(filepath.Join(p.claimsDir, "a.yaml"))
			if readErr != nil {
				t.Fatal(readErr)
			}

			_, _, err := p.deps().Add(claimA, model.CommentRoleHuman, body)
			if !errors.Is(err, ErrUnsafeBody) {
				t.Fatalf("Add(body=%q): want ErrUnsafeBody, got %v", body, err)
			}
			after, readErr := os.ReadFile(filepath.Join(p.claimsDir, "a.yaml"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !bytes.Equal(before, after) {
				t.Fatalf("Add(body=%q) rejected but the claim file changed", body)
			}
			// And the dir must still load — never a brick.
			if _, err := p.deps().List(claimA, false); err != nil {
				t.Fatalf("claim dir unusable after a rejected Add(body=%q): %v", body, err)
			}
		})
	}
}

// TestValidateBody_RoundTripAccurate pins that validation is ROUND-TRIP-ACCURATE
// — a body is ErrUnsafeBody iff it cannot be stored and read back byte-exact —
// rather than driven by a leading-whitespace heuristic. It covers the two
// classes the old heuristic got wrong:
//
//   - a first CONTENT line that itself begins with a tab or space indent
//     ("\tcode\nmore", "    code\n    return") is store-bricking, but the old
//     heuristic (first line trims to non-empty) MISSED it: it passed validateBody
//     and was only refused later by the loader guard, leaking a raw round-trip
//     error to the user. These must now be rejected up front as ErrUnsafeBody.
//   - a body whose first line is whitespace-only or CR/NBSP/NEL/VT/FF-led yet
//     round-trips cleanly (" \ncontent", "\r\ncontent", NBSP/NEL + newline) was
//     FALSE-REJECTED by the old heuristic; these must now be accepted.
func TestValidateBody_RoundTripAccurate(t *testing.T) {
	nbsp := string(rune(0x00A0))
	nel := string(rune(0x0085))
	vt := string(rune(0x000B))
	ff := string(rune(0x000C))
	cases := []struct {
		name string
		body string
		want error // nil == accepted
	}{
		// content-first-line class — MUST be rejected (old heuristic missed these)
		{"tab-led-content-line", "\tcode\nmore", ErrUnsafeBody},
		{"space-indented-content-line", "    func main(){}\n    return", ErrUnsafeBody},
		{"two-space-indented-multiline", "  a\n  b", ErrUnsafeBody},
		// still rejected (bare newline / blank / tab-only first line)
		{"bare-leading-newline", "\ncontent", ErrUnsafeBody},
		{"leading-blank-lines", "\n\nblank then content", ErrUnsafeBody},
		{"tab-only-first-line", "\t\nreal content", ErrUnsafeBody},
		// round-trips fine — MUST be accepted (old heuristic false-rejected these)
		{"space-then-newline", " \ncontent", nil},
		{"crlf-lead", "\r\ncontent", nil},
		{"nbsp-then-newline", nbsp + "\ncontent", nil},
		{"nel-then-newline", nel + "\ncontent", nil},
		{"vt-then-newline", vt + "\ncontent", nil},
		{"ff-then-newline", ff + "\ncontent", nil},
		{"interior-tab", "a\tb", nil},
		{"normal-multiline", "a\nb\nc", nil},
		{"windows-endings", "windows\r\nline\r\nendings", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBody(tc.body)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("validateBody(%q) = %v, want accepted", tc.body, err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("validateBody(%q) = %v, want %v", tc.body, err, tc.want)
			}
		})
	}
}

// contentFirstLineUnsafeBodies are store-bricking bodies whose FIRST line is real
// CONTENT that begins with indentation — the class the old leading-whitespace
// heuristic MISSED, so they slipped past validateBody and surfaced only as a
// leaked raw round-trip error from the loader guard. They must now be rejected
// up front, at the shared CLI+serve input boundary, with the clean ErrUnsafeBody.
var contentFirstLineUnsafeBodies = []string{
	"\tcode line\nmore",                  // tab-led first content line
	"    func main() {}\n    return nil", // space-indented first content line
}

// TestAddReplyEdit_RejectContentFirstLineUnsafeBody_NoWrite_NoLeak proves the
// content-first-line class is caught at the input boundary (clean ErrUnsafeBody,
// NOT the leaked loader round-trip error), writes nothing, and never bricks the
// store, across Add/Reply/Edit.
func TestAddReplyEdit_RejectContentFirstLineUnsafeBody_NoWrite_NoLeak(t *testing.T) {
	for _, body := range contentFirstLineUnsafeBodies {
		t.Run(body, func(t *testing.T) {
			p := newProject(t, map[string]string{"a.yaml": draftAYAML})
			_, tid, err := p.deps().Add(claimA, model.CommentRoleHuman, "real thread")
			if err != nil {
				t.Fatalf("seed Add: %v", err)
			}
			before, readErr := os.ReadFile(filepath.Join(p.claimsDir, "a.yaml"))
			if readErr != nil {
				t.Fatal(readErr)
			}

			assertUnsafeNoLeak := func(op string, err error) {
				t.Helper()
				if !errors.Is(err, ErrUnsafeBody) {
					t.Fatalf("%s(body=%q): want ErrUnsafeBody, got %v", op, body, err)
				}
				// The clean error must NOT be (or wrap) the internal loader
				// round-trip sentinel, and must not leak its raw text.
				if errors.Is(err, loader.ErrClaimNotRoundTrippable) {
					t.Fatalf("%s(body=%q): leaked loader.ErrClaimNotRoundTrippable to the caller: %v", op, body, err)
				}
				if strings.Contains(err.Error(), "round-trip") || strings.Contains(err.Error(), "round trip") {
					t.Fatalf("%s(body=%q): error text leaked the internal round-trip detail: %q", op, body, err.Error())
				}
			}

			_, _, addErr := p.deps().Add(claimA, model.CommentRoleHuman, body)
			assertUnsafeNoLeak("Add", addErr)
			_, _, replyErr := p.deps().Reply(claimA, tid, model.CommentRoleHuman, body)
			assertUnsafeNoLeak("Reply", replyErr)
			_, editErr := p.deps().Edit(claimA, tid, "", model.CommentRoleHuman, body)
			assertUnsafeNoLeak("Edit", editErr)

			// Nothing was written past the seed thread, and the dir still loads.
			after, readErr := os.ReadFile(filepath.Join(p.claimsDir, "a.yaml"))
			if readErr != nil {
				t.Fatal(readErr)
			}
			if !bytes.Equal(before, after) {
				t.Fatalf("rejected unsafe ops changed the claim file (body=%q)", body)
			}
			if got := p.reload(claimA); len(got.Comments) != 1 || got.Comments[0].Body != "real thread" {
				t.Fatalf("store mutated/bricked by a rejected unsafe op (body=%q): %+v", body, got.Comments)
			}
		})
	}
}

func TestReplyAndEdit_RejectUnsafeLeadingWhitespaceBody(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	_, tid, err := p.deps().Add(claimA, model.CommentRoleHuman, "real thread")
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	for _, body := range unsafeLeadingBodies {
		if _, _, err := p.deps().Reply(claimA, tid, model.CommentRoleHuman, body); !errors.Is(err, ErrUnsafeBody) {
			t.Fatalf("Reply(body=%q): want ErrUnsafeBody, got %v", body, err)
		}
		if _, err := p.deps().Edit(claimA, tid, "", model.CommentRoleHuman, body); !errors.Is(err, ErrUnsafeBody) {
			t.Fatalf("Edit(body=%q): want ErrUnsafeBody, got %v", body, err)
		}
	}
	// The thread root is unchanged after the rejected edits.
	if got := p.reload(claimA); got.Comments[0].Body != "real thread" {
		t.Fatalf("thread root mutated by a rejected Edit: %q", got.Comments[0].Body)
	}
}
