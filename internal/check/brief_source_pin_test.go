package check_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// brief_source_pin_test.go pins how an internal source that cites a BRIEF is
// checked (NIT-198 F1): its sha256 is the brief's content hash — summary,
// rests_on and body, what `brief lock` signs and `brief show` prints as
// content_hash — so the thread writes the served viewer makes into the brief's
// frontmatter, and a status flip, never read as source-internal-drift under
// the citing claim, in --validate or --staged. A body edit still does, and an
// internal source that is not a brief keeps its whole-file pin.

const pinnedBrief = "---\nsummary: How the widget flow reads end to end.\nstatus: draft\nrests_on:\n  - widget.contract.overview\n---\n# Widget flow\n\nText.\n"
const pinnedNote = "---\nstatus: draft\n---\nA plain note beside the claims.\n"

func citingClaim(briefPin, notePin string) string {
	return "id: widget.contract.cites\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\nsummary: A claim that cites a brief and a note.\n" +
		"body: |\n  Rests on the flow [1] and the note [2].\n" +
		"rests_on:\n  none: true\n  reason: fixture\n" +
		"sources:\n" +
		"  - ref: 1\n    kind: internal\n    title: The widget flow brief\n    path: briefs/widget/flow.md\n    sha256: " + briefPin + "\n" +
		"  - ref: 2\n    kind: internal\n    title: A note\n    path: notes/evidence.md\n    sha256: " + notePin + "\n"
}

// sourceDrift keeps a verdict's source-internal-drift findings as
// "claim|message-prefix" so a failure names which source moved.
func sourceDrift(verdict []string) []string {
	var out []string
	for _, line := range verdict {
		parts := strings.SplitN(line, "|", 5)
		if len(parts) == 5 && parts[1] == "source-internal-drift" {
			out = append(out, parts[2]+"|"+strings.SplitN(parts[4], ":", 2)[0])
		}
	}
	return out
}

func TestACitedBriefsPinIgnoresItsThreadsAndStatus(t *testing.T) {
	flow := filepath.Join("briefs", "widget", "flow.md")
	repo := filepath.Join(t.TempDir(), "repo")
	cfg := writeProjectFiles(t, repo, baseConfig, map[string]string{
		"claims/overview.yaml":  draftClaim("widget.contract.overview"),
		"briefs/widget/flow.md": pinnedBrief,
		"notes/evidence.md":     pinnedNote,
	})
	b, ok := briefs.Load(cfg).Lookup("widget.flow")
	if !ok {
		t.Fatal("no brief")
	}
	noteSum := sha256.Sum256([]byte(pinnedNote))
	writeFixtureFile(t, filepath.Join(repo, "claims", "cites.yaml"), citingClaim(b.LockHash, hex.EncodeToString(noteSum[:])))
	gitRepo(t, repo)
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "fixture")

	bothModes := func(t *testing.T, what string, want []string) {
		t.Helper()
		if got := sourceDrift(worktreeVerdict(t, cfg)); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("%s, --validate: got %v, want %v", what, got, want)
		}
		git(t, repo, "add", "-A")
		if got, _ := stagedVerdict(t, cfg); strings.Join(sourceDrift(got), ",") != strings.Join(want, ",") {
			t.Fatalf("%s, --staged: got %v, want %v", what, sourceDrift(got), want)
		}
	}
	bothModes(t, "the pins as written", nil)

	// The thread writes the viewer makes: add, reply, resolve.
	deps := &comments.Deps{Cfg: cfg}
	_, tid, err := deps.BriefAdd("widget.flow", model.CommentRoleHuman, "does this still hold?")
	if err != nil {
		t.Fatal(err)
	}
	bothModes(t, "after a thread was opened on the brief", nil)
	if _, _, err := deps.BriefReply("widget.flow", tid, model.CommentRoleAgent, "yes"); err != nil {
		t.Fatal(err)
	}
	if _, err := deps.BriefResolve("widget.flow", tid, model.CommentRoleHuman); err != nil {
		t.Fatal(err)
	}
	bothModes(t, "after a reply and the human's resolve", nil)

	// A status flip is not content either.
	raw := readFixtureFile(t, filepath.Join(repo, flow))
	writeFixtureFile(t, filepath.Join(repo, flow), strings.Replace(raw, "status: draft", "status: locked", 1))
	bothModes(t, "after the status flipped", nil)

	// A body edit is: the claim may now rest on something the brief no
	// longer says.
	writeFixtureFile(t, filepath.Join(repo, flow), strings.Replace(raw, "Text.", "Text, rewritten.", 1))
	bothModes(t, "after the body changed", []string{"widget.contract.cites|sources[0]"})

	// A note that is not a brief keeps its whole-file pin: its status line
	// is bytes like any other.
	writeFixtureFile(t, filepath.Join(repo, flow), raw)
	writeFixtureFile(t, filepath.Join(repo, "notes", "evidence.md"), strings.Replace(pinnedNote, "draft", "final", 1))
	bothModes(t, "after the note's status line changed", []string{"widget.contract.cites|sources[1]"})
}

// TestACitedBriefsStagedPinReadsTheIndex is the combo-audit ENG-1 pin: a
// pre-commit hook that hashed the worktree would accept a staged brief
// rewrite whenever the unstaged copy still matched the claim's sha256.
func TestACitedBriefsStagedPinReadsTheIndex(t *testing.T) {
	flow := filepath.Join("briefs", "widget", "flow.md")
	repo := filepath.Join(t.TempDir(), "repo")
	cfg := writeProjectFiles(t, repo, baseConfig, map[string]string{
		"claims/overview.yaml":  draftClaim("widget.contract.overview"),
		"briefs/widget/flow.md": pinnedBrief,
		"notes/evidence.md":     pinnedNote,
	})
	b, ok := briefs.Load(cfg).Lookup("widget.flow")
	if !ok {
		t.Fatal("no brief")
	}
	noteSum := sha256.Sum256([]byte(pinnedNote))
	writeFixtureFile(t, filepath.Join(repo, "claims", "cites.yaml"), citingClaim(b.LockHash, hex.EncodeToString(noteSum[:])))
	gitRepo(t, repo)
	git(t, repo, "add", "-A")
	git(t, repo, "commit", "-qm", "fixture")

	rewritten := strings.Replace(pinnedBrief, "Text.", "Text, rewritten.", 1)
	writeFixtureFile(t, filepath.Join(repo, flow), rewritten)
	git(t, repo, "add", flow)
	writeFixtureFile(t, filepath.Join(repo, flow), pinnedBrief)

	if got := sourceDrift(worktreeVerdict(t, cfg)); len(got) != 0 {
		t.Fatalf("--validate must follow the worktree (still pinned): got %v", got)
	}
	got, _ := stagedVerdict(t, cfg)
	if want := []string{"widget.contract.cites|sources[0]"}; strings.Join(sourceDrift(got), ",") != strings.Join(want, ",") {
		t.Fatalf("--staged must hash the index blob: got %v, want %v", sourceDrift(got), want)
	}
}

func readFixtureFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
