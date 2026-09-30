package comments

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

const briefFlow = "---\nsummary: The widget flow.\nstatus: locked\nrests_on:\n  - widget.contract.a\n---\n# Flow\n\nText.\n"

// briefProject is newProject with one brief at briefs/widget/flow.md.
func briefProject(t *testing.T) (p *project, file string) {
	t.Helper()
	p = newProject(t, map[string]string{"a.yaml": draftAYAML})
	file = filepath.Join(p.root, "briefs", "widget", "flow.md")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(briefFlow), 0o644); err != nil {
		t.Fatal(err)
	}
	return p, file
}

// TestBriefThreads_TheSameOpsAndRightsAsAClaim pins the brief methods on
// Deps: a thread is opened, replied to, and — because the advisory rights rule
// is the claims' own — an agent cannot resolve the human's thread while the
// human can; every op writes only the brief's comments block (its lock hash
// and body do not move) and records the brief's digest in the store's briefs
// map, never in the claim map.
func TestBriefThreads_TheSameOpsAndRightsAsAClaim(t *testing.T) {
	p, file := briefProject(t)
	d := &Deps{Cfg: p.cfg}
	before := briefs.Load(p.cfg).Briefs[0]

	b, tid, err := d.BriefAdd("briefs/widget/flow.md", model.CommentRoleHuman, "is this still the flow?")
	if err != nil || !threadIDRe.MatchString(tid) || b.OpenThreads() != 1 {
		t.Fatalf("BriefAdd: %v %q %+v", err, tid, b.Comments)
	}
	if _, rid, err := d.BriefReply("widget.flow", tid, model.CommentRoleAgent, "yes, unchanged"); err != nil || !replyIDRe.MatchString(rid) {
		t.Fatalf("BriefReply by id: %v %q", err, rid)
	}
	if _, err := d.BriefResolve("briefs/widget/flow.md", tid, model.CommentRoleAgent); !errors.Is(err, ErrRightsDenied) {
		t.Fatalf("an agent must not resolve the human's thread on a brief, got %v", err)
	}
	if _, err := d.BriefResolve("briefs/widget/flow.md", tid, model.CommentRoleHuman); err != nil {
		t.Fatalf("the human resolves: %v", err)
	}

	after := briefs.Load(p.cfg).Briefs[0]
	if after.LockHash != before.LockHash || after.Body != before.Body || after.Status != briefs.StatusLocked {
		t.Fatalf("a comment op moved the brief's signed content or status")
	}
	if len(after.Comments) != 1 || len(after.Comments[0].Replies) != 1 || after.Comments[0].Status != model.CommentStatusResolved {
		t.Fatalf("threads on disk = %+v", after.Comments)
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(raw), "---\n# Flow\n\nText.\n") {
		t.Fatalf("the body moved:\n%s", raw)
	}
	store, err := digest.LoadStore(digest.StorePath(p.cfg))
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := store.BriefDigest("widget.flow"); !ok || got != digest.BriefCommentsDigest("widget.flow", after.Comments) {
		t.Fatalf("the brief's digest must be recorded in the briefs map, got %q %v", got, ok)
	}
	if _, inClaims := store.Digest("widget.flow"); inClaims {
		t.Fatal("a brief's digest must never be a key of the claim map")
	}
	_, threads, err := d.BriefList("briefs/widget/flow.md", true)
	if err != nil || len(threads) != 0 {
		t.Fatalf("BriefList --open after resolve: %v %+v", err, threads)
	}
}

// TestBriefThreads_RefuseAHandEditedBlock is the digest gate for a brief: once
// the engine has recorded a brief's threads, a hand edit of the block refuses
// the next op instead of re-recording the edit as truth.
func TestBriefThreads_RefuseAHandEditedBlock(t *testing.T) {
	p, file := briefProject(t)
	d := &Deps{Cfg: p.cfg}
	if _, _, err := d.BriefAdd("briefs/widget/flow.md", model.CommentRoleHuman, "why?"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	forged := strings.Replace(string(raw), "status: open", "status: resolved", 1)
	if forged == string(raw) {
		t.Fatalf("fixture: no open thread to forge:\n%s", raw)
	}
	if err := os.WriteFile(file, []byte(forged), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.BriefAdd("briefs/widget/flow.md", model.CommentRoleAgent, "another"); !errors.Is(err, ErrCommentDigestDrift) {
		t.Fatalf("a hand-edited brief block must refuse the next op, got %v", err)
	}
	if got, err := os.ReadFile(file); err != nil || string(got) != forged {
		t.Fatal("a refused op wrote the brief")
	}
}

// TestBriefThreads_NeverWriteThroughALinkOrOverAnEdit pins the two write-path
// guards: a brief that changed between the read and the write is refused
// (ErrBriefFileChanged) and left as the editor saved it; and a brief file that
// is a symlink is not a brief the ops can reach, so the file it points at is
// never written.
func TestBriefThreads_NeverWriteThroughALinkOrOverAnEdit(t *testing.T) {
	p, file := briefProject(t)
	d := &Deps{Cfg: p.cfg}
	edited := briefFlow + "An editor's save.\n"
	mutateInterlude = func() {
		if err := os.WriteFile(file, []byte(edited), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { mutateInterlude = func() {} })
	if _, _, err := d.BriefAdd("briefs/widget/flow.md", model.CommentRoleHuman, "why?"); !errors.Is(err, ErrBriefFileChanged) {
		t.Fatalf("an edit between read and write must refuse the op, got %v", err)
	}
	if got, err := os.ReadFile(file); err != nil || string(got) != edited {
		t.Fatalf("the editor's save was overwritten:\n%s", got)
	}
	mutateInterlude = func() {}

	if runtime.GOOS == "windows" {
		return
	}
	target := filepath.Join(p.root, "outside.md")
	if err := os.WriteFile(target, []byte(briefFlow), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(p.root, "briefs", "widget", "linked.md")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := d.BriefAdd("briefs/widget/linked.md", model.CommentRoleHuman, "why?"); !errors.Is(err, ErrBriefNotFound) {
		t.Fatalf("a symlinked brief is refused by discovery, so the op must not find it, got %v", err)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != briefFlow {
		t.Fatal("the link's target was written")
	}
}
