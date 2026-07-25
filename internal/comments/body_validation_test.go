package comments

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

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
			before, _ := os.ReadFile(filepath.Join(p.claimsDir, "a.yaml"))

			_, _, err := p.deps().Add(claimA, model.CommentRoleHuman, body)
			if !errors.Is(err, ErrUnsafeBody) {
				t.Fatalf("Add(body=%q): want ErrUnsafeBody, got %v", body, err)
			}
			after, _ := os.ReadFile(filepath.Join(p.claimsDir, "a.yaml"))
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
