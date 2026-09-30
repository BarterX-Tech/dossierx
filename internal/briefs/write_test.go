package briefs

import (
	"errors"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// TestSetStatus_RewritesOnlyTheStatusToken pins the one byte range `brief
// lock` / `brief unlock` may change in a brief: the status value. Every other
// byte — a YAML comment on the line, quoting, CRLF line endings, the body — is
// kept, and a brief with no status line gains one before the closing ---.
func TestSetStatus_RewritesOnlyTheStatusToken(t *testing.T) {
	for _, tc := range []struct {
		name, in, want string
	}{
		{"plain", "---\nsummary: S.\nstatus: draft # the author's note\n---\nBody.\n", "---\nsummary: S.\nstatus: locked # the author's note\n---\nBody.\n"},
		{"quoted", "---\nsummary: S.\nstatus: \"draft\"\n---\nBody.\n", "---\nsummary: S.\nstatus: \"locked\"\n---\nBody.\n"},
		{"absent", "---\nsummary: S.\n---\nBody.\n", "---\nsummary: S.\nstatus: locked\n---\nBody.\n"},
		{"crlf", "---\r\nsummary: S.\r\n---\r\nBody.\r\n", "---\r\nsummary: S.\r\nstatus: locked\r\n---\r\nBody.\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := SetStatus([]byte(tc.in), StatusLocked)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Fatalf("SetStatus =\n%q\nwant\n%q", got, tc.want)
			}
			back, err := SetStatus(got, StatusDraft)
			if err != nil {
				t.Fatal(err)
			}
			if fm, _, _ := parseFrontmatter(string(normalizeLineEndings(back))); fm.status != "draft" {
				t.Fatalf("unlocking did not read back as draft: %q", back)
			}
		})
	}
	if _, err := SetStatus([]byte("---\nsummary: S.\nstatus: >-\n  draft\n---\nB.\n"), StatusLocked); !errors.Is(err, ErrFrontmatterNotRewritable) {
		t.Fatalf("a status the engine cannot rewrite in place must be refused, got %v", err)
	}
}

// TestSetComments_RoundTripsAndTouchesNothingElse pins the comment ops'
// rewrite: the threads read back exactly, the summary, rests_on, status and
// body are byte-identical, a second write replaces rather than appends, and an
// empty list removes the block.
func TestSetComments_RoundTripsAndTouchesNothingElse(t *testing.T) {
	in := "---\nsummary: S.\n# a comment the author wrote\nrests_on:\n  - a.b.c\nstatus: locked\n---\n# Title\n\nBody.\n"
	one := []model.Comment{{ID: "c-1", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Created: "2026-09-30T00:00:00Z", Body: "why?"}}
	got, err := SetComments([]byte(in), one)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), "---\nsummary: S.\n# a comment the author wrote\nrests_on:\n  - a.b.c\nstatus: locked\n") || !strings.HasSuffix(string(got), "---\n# Title\n\nBody.\n") {
		t.Fatalf("bytes outside the comments block moved:\n%s", got)
	}
	fm, _, problems := parseFrontmatter(string(got))
	if len(problems) > 0 || len(fm.comments) != 1 || fm.comments[0].Body != "why?" {
		t.Fatalf("threads did not read back: %+v %v", fm.comments, problems)
	}
	two := append(append([]model.Comment{}, one...), model.Comment{ID: "c-2", Status: model.CommentStatusResolved, Author: model.CommentRoleAgent, Created: "2026-09-30T00:00:01Z", Body: "done", Replies: []model.Reply{{ID: "r-1", Author: model.CommentRoleHuman, Created: "2026-09-30T00:00:02Z", Body: "ok"}}})
	again, err := SetComments(got, two)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(again), "comments:") != 1 {
		t.Fatalf("a second write must replace the block, not add one:\n%s", again)
	}
	cleared, err := SetComments(again, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(cleared) != in {
		t.Fatalf("an empty thread list must remove the block and leave the file as it was:\n%q\nwant\n%q", cleared, in)
	}
}
