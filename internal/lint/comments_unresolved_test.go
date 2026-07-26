package lint

import (
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

func TestCommentsUnresolvedLint(t *testing.T) {
	openThread := []model.Comment{{ID: "c-000001", Status: model.CommentStatusOpen, Author: model.CommentRoleHuman, Body: "clarify?"}}
	resolvedThread := []model.Comment{{ID: "c-000002", Status: model.CommentStatusResolved, Author: model.CommentRoleHuman, Body: "done"}}

	cases := []struct {
		name         string
		claim        model.Claim
		wantFindings int
	}{
		{
			name:         "draft claim with an open thread warns",
			claim:        model.Claim{ID: "widget.contract.a", Status: model.StatusDraft, Layout: model.LayoutCard, Comments: openThread},
			wantFindings: 1,
		},
		{
			// The rule deliberately has NO Status branch (unlike rest-on-locked):
			// it fires on locked claims too, since an open thread on a locked
			// claim is simultaneously its lock-gate refusal and its
			// review_pending trigger.
			name:         "locked claim with an open thread warns (no Status branch)",
			claim:        model.Claim{ID: "widget.contract.b", Status: model.StatusLocked, Layout: model.LayoutCard, Comments: openThread},
			wantFindings: 1,
		},
		{
			name:         "resolved-only threads do not warn",
			claim:        model.Claim{ID: "widget.contract.c", Status: model.StatusLocked, Layout: model.LayoutCard, Comments: resolvedThread},
			wantFindings: 0,
		},
		{
			name:         "no comments does not warn",
			claim:        model.Claim{ID: "widget.contract.d", Status: model.StatusDraft, Layout: model.LayoutCard},
			wantFindings: 0,
		},
		{
			name:         "banner claim with an open thread is skipped",
			claim:        model.Claim{ID: "widget.contract.banner", Status: model.StatusDraft, Layout: model.LayoutBanner, Comments: openThread},
			wantFindings: 0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := commentsUnresolvedLint{}.Check([]model.Claim{tc.claim}, nil)
			if len(findings) != tc.wantFindings {
				t.Fatalf("got %d findings, want %d: %+v", len(findings), tc.wantFindings, findings)
			}
			if tc.wantFindings > 0 {
				if findings[0].Severity != SeverityWarning {
					t.Fatalf("got severity %q, want %q (this lint is ALWAYS a warning)", findings[0].Severity, SeverityWarning)
				}
				if findings[0].LintName != "comments-unresolved" {
					t.Fatalf("got lint name %q, want comments-unresolved", findings[0].LintName)
				}
			}
		})
	}
}

// TestCommentsUnresolvedLintCountsOpenThreads proves the message reports the
// number of OPEN threads only (resolved threads are not counted).
func TestCommentsUnresolvedLintCountsOpenThreads(t *testing.T) {
	claim := model.Claim{
		ID: "widget.contract.multi", Status: model.StatusLocked, Layout: model.LayoutCard,
		Comments: []model.Comment{
			{ID: "c-000001", Status: model.CommentStatusOpen},
			{ID: "c-000002", Status: model.CommentStatusResolved},
			{ID: "c-000003", Status: model.CommentStatusOpen},
		},
	}
	findings := commentsUnresolvedLint{}.Check([]model.Claim{claim}, nil)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "2") {
		t.Fatalf("expected message to report 2 open threads, got %q", findings[0].Message)
	}
}
