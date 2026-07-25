package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/loader"
)

// friendlyCommentBodyErr is the CLI backstop that guarantees an unsafe comment
// body NEVER surfaces the loader's cryptic internal "did not round-trip
// byte-exact" text: should the store-bricking sentinel reach the CLI (any future
// divergence between the round-trip-accurate input pre-check and the save-time
// guard), it must print the friendly comments.ErrUnsafeBody guidance instead,
// while passing every other error through untouched.
func TestFriendlyCommentBodyErr(t *testing.T) {
	// The wrapped loader sentinel maps to the friendly ErrUnsafeBody, and the
	// resulting message carries none of the internal round-trip text.
	raw := fmt.Errorf(`loader: claim "x": comment "c-1" body did not round-trip byte-exact: %w`, loader.ErrClaimNotRoundTrippable)
	got := friendlyCommentBodyErr(raw)
	if !errors.Is(got, comments.ErrUnsafeBody) {
		t.Fatalf("friendlyCommentBodyErr(round-trip err) = %v, want comments.ErrUnsafeBody", got)
	}
	for _, cryptic := range []string{"round-trip", "byte-exact", "store-bricking"} {
		if strings.Contains(got.Error(), cryptic) {
			t.Fatalf("translated error still leaks %q: %q", cryptic, got.Error())
		}
	}

	// nil passes through as nil.
	if friendlyCommentBodyErr(nil) != nil {
		t.Fatal("friendlyCommentBodyErr(nil) must be nil")
	}
	// An already-friendly ErrUnsafeBody stays ErrUnsafeBody.
	if got := friendlyCommentBodyErr(comments.ErrUnsafeBody); !errors.Is(got, comments.ErrUnsafeBody) {
		t.Fatalf("friendlyCommentBodyErr(ErrUnsafeBody) = %v, want ErrUnsafeBody", got)
	}
	// An unrelated error is returned unchanged.
	other := errors.New("comments: some other failure")
	if got := friendlyCommentBodyErr(other); got != other {
		t.Fatalf("friendlyCommentBodyErr(other) = %v, want the same error unchanged", got)
	}
}
