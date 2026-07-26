package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/loader"
)

// friendlyCommentBodyErr keeps the two DISTINCT store-safety failures apart at
// the CLI boundary, and NEVER surfaces the loader's cryptic internal "did not
// round-trip byte-exact" text:
//
//   - loader.ErrClaimNotRoundTrippable is the WHOLE claim's stored bytes (often a
//     pre-existing prose/comment body a user hand-edited to a store-bricking
//     shape) failing to re-serialize. That is NOT the caller's supplied body, so
//     it must translate to a DISTINCT, claim-SCOPED message that names the claim
//     and points at its stored body — never the de-indent-your-input guidance.
//   - comments.ErrUnsafeBody (the supplied-body pre-check) keeps its own
//     actionable de-indent guidance and passes through unchanged.
//
// Every other error (and nil) passes through untouched.
func TestFriendlyCommentBodyErr(t *testing.T) {
	const claimID = "widget.contract.poison"

	// The wrapped loader store-bricking sentinel maps to a DISTINCT claim-scoped
	// message that NAMES the claim and points at its STORED body — never the raw
	// internal round-trip text, and never the supplied-body de-indent guidance.
	raw := fmt.Errorf(`loader: claim "widget.contract.poison": marshaled YAML does not re-parse (yaml: line 6: found a tab character where an indentation space is expected): %w`, loader.ErrClaimNotRoundTrippable)
	got := friendlyCommentBodyErr(claimID, raw)
	if got == nil {
		t.Fatal("friendlyCommentBodyErr(store-bricking) must not be nil")
	}
	// It is NO LONGER collapsed into ErrUnsafeBody: the two failures are distinct.
	if errors.Is(got, comments.ErrUnsafeBody) {
		t.Fatalf("store-bricking stored-body error must NOT be mapped to ErrUnsafeBody (its de-indent guidance is wrong here): %q", got.Error())
	}
	msg := got.Error()
	// Claim-scoped: it names the offending claim and points at its stored body.
	if !strings.Contains(msg, claimID) {
		t.Fatalf("claim-scoped message must name the claim %q, got: %q", claimID, msg)
	}
	if !strings.Contains(msg, "stored body") {
		t.Fatalf("claim-scoped message must point at the STORED body, got: %q", msg)
	}
	// It must NOT carry the supplied-body de-indent guidance (that guidance is
	// about the CALLER's input, which is fine here).
	if strings.Contains(msg, "de-indent") {
		t.Fatalf("claim-scoped message must not carry the supplied-body de-indent guidance: %q", msg)
	}
	// It must NOT leak any internal loader/yaml/round-trip detail.
	for _, cryptic := range []string{"round-trip", "byte-exact", "store-bricking", "block scalar", "did not re-parse", "loader:", "yaml:"} {
		if strings.Contains(msg, cryptic) {
			t.Fatalf("translated error still leaks internal detail %q: %q", cryptic, msg)
		}
	}

	// nil passes through as nil.
	if friendlyCommentBodyErr(claimID, nil) != nil {
		t.Fatal("friendlyCommentBodyErr(nil) must be nil")
	}
	// A genuinely-unsafe SUPPLIED body keeps its own de-indent guidance: the
	// pre-check's ErrUnsafeBody passes through unchanged (NOT the claim-scoped
	// message — the caller's input IS at fault here).
	if got := friendlyCommentBodyErr(claimID, comments.ErrUnsafeBody); !errors.Is(got, comments.ErrUnsafeBody) {
		t.Fatalf("friendlyCommentBodyErr(ErrUnsafeBody) = %v, want ErrUnsafeBody (supplied-body guidance preserved)", got)
	}
	// An unrelated error is returned unchanged.
	other := errors.New("comments: some other failure")
	if got := friendlyCommentBodyErr(claimID, other); got != other {
		t.Fatalf("friendlyCommentBodyErr(other) = %v, want the same error unchanged", got)
	}
}
