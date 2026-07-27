package comments

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// realLeafCommands is the v0.3.0 CLI surface, as a set: seven nouns, twenty
// leaves, and nothing else. It is duplicated here rather than imported because
// internal/comments must not depend on cmd/dossierx — the engine is the thing
// the CLI wraps, not the other way round — and because the point of the test
// below is precisely that this package's PROSE can drift from that surface
// without any compiler catching it.
//
// The authority for this list is `dossierx --help` plus each noun's own help,
// and it is restated in skills/dossierx/SKILL.md's "seven nouns, twenty leaves"
// block. If a release adds or removes a verb, this set and that block move
// together.
var realLeafCommands = map[string]bool{
	"check": true,

	"claim show": true, "claim list": true, "claim new": true,
	"claim lock": true, "claim unlock": true, "claim flag": true,
	"claim reaudit": true, "claim link": true,

	"comment inbox": true, "comment list": true,
	"comment add": true, "comment reply": true,

	"build-order propose": true, "build-order status": true, "build-order lock": true,

	"migrate": true,

	"serve": true, "skills export": true, "version": true,
}

// invokedCommand finds every `dossierx <word> [<word>]` an error message tells
// its reader to run. It deliberately stops at the first character that cannot
// begin a verb, so flags (`--reason`), punctuation and placeholders (`<id>`) end
// the match and only the command itself is captured.
//
// Two words are captured because three of the seven nouns are groups, and the
// second word is then either the leaf or the next word of the sentence
// ("dossierx check reports it as …"). namesARealCommand resolves that ambiguity
// the only way that cannot produce a false alarm: the pair counts, and failing
// that the first word alone counts. A bare group noun (`dossierx comment`) is
// deliberately NOT accepted — the binary refuses it with `usage`, so telling a
// reader to run one is the same defect in a smaller size.
var invokedCommand = regexp.MustCompile(`dossierx ([a-z][a-z-]*(?: [a-z][a-z-]*)?)`)

func namesARealCommand(captured string) bool {
	if realLeafCommands[captured] {
		return true
	}
	first, _, ok := strings.Cut(captured, " ")
	return ok && realLeafCommands[first]
}

// TestCommentDigestDriftRefusalNamesOnlyCommandsThatExist is the regression for
// a refusal that instructed its reader to run a command the binary does not
// have.
//
// The message used to end with "ask the human and run: dossierx comment reaudit
// --claim <id> --reason ...". Deps.ReauditDigest is real and is exactly that
// operation, but NO CLI verb reaches it — `dossierx comment` is inbox, list, add
// and reply, and nothing else — so the one moment an agent is wedged and reading
// the message for a way out, it was handed `usage: unknown command`. A refusal
// that names a non-existent recovery is worse than one that names none: it costs
// the reader a round trip and then leaves them improvising, and the improvisation
// closest to hand is deleting the digest store, which is the laundering the store
// exists to catch.
//
// The assertion is deliberately structural rather than a string match on the
// fixed text: every `dossierx …` this package prints has to name a leaf that
// exists, so the next verb someone invents in prose fails here rather than in a
// user's terminal.
func TestCommentDigestDriftRefusalNamesOnlyCommandsThatExist(t *testing.T) {
	msg := driftRefusal(t)

	found := invokedCommand.FindAllStringSubmatch(msg, -1)
	if len(found) == 0 {
		t.Fatalf("the refusal names no dossierx command at all, so it offers no recovery: %q", msg)
	}
	for _, m := range found {
		if !namesARealCommand(m[1]) {
			t.Errorf("the refusal tells its reader to run %q, which is not a command this binary has: %q", "dossierx "+m[1], msg)
		}
	}
}

// TestCommentDigestDriftRefusalNeverAdvisesDeletingTheStore pins the half of the
// message that is a security property rather than a usability one.
//
// Deleting .dossierx-comment-digest.json makes this refusal go away, and it is
// the single worst thing a wedged reader can do: a claim the store has never
// seen is *unknown*, never *drifted*, so the delete clears both this refusal and
// the comment-ledger-drift finding that named the edit. internal/check reports
// the delete as comment-digest-absent for exactly that reason, and
// recordCommentDigest refuses to re-adopt into a ledger-covered project whose
// store is missing. The refusal must not undo any of that by suggesting it.
func TestCommentDigestDriftRefusalNeverAdvisesDeletingTheStore(t *testing.T) {
	msg := driftRefusal(t)
	lower := strings.ToLower(msg)

	// `rm` is never legitimate in this message in any framing, so it is a flat
	// ban rather than a negation check.
	if strings.Contains(lower, "rm ") {
		t.Errorf("the refusal reaches for rm, which is the laundering path: %q", msg)
	}

	// The English verbs ARE legitimate — but only inside a prohibition, which is
	// the whole reason the message mentions them. So each occurrence has to sit
	// downstream of a negation; an un-negated one is advice.
	for _, verb := range []string{"delete", "deleting", "remove", "removing"} {
		for at := 0; ; {
			i := strings.Index(lower[at:], verb)
			if i < 0 {
				break
			}
			i += at
			if !negatedBefore(lower, i) {
				t.Errorf("the refusal uses %q at offset %d without negating it, which reads as advice: %q", verb, i, msg)
			}
			at = i + len(verb)
		}
	}

	// And it must say so out loud, since "restore the store" and "delete the
	// store" are one keystroke apart for a reader in a hurry.
	if !strings.Contains(lower, "do not delete") && !strings.Contains(lower, "never delete") {
		t.Errorf("the refusal must warn against deleting %s explicitly: %q", digest.StoreFileName, msg)
	}
}

// negatedBefore reports whether one of the negations sits in the window of text
// immediately preceding index i. The window is a sentence-ish 60 characters:
// long enough to reach back over "DO NOT DELETE <the store's 31-character
// filename>", short enough that a negation two clauses away does not launder an
// un-negated verb further on.
func negatedBefore(lower string, i int) bool {
	from := i - 60
	if from < 0 {
		from = 0
	}
	window := lower[from:i]
	for _, neg := range []string{"do not", "don't", "never", "not the", "rather than"} {
		if strings.Contains(window, neg) {
			return true
		}
	}
	return false
}

// TestCommentDigestDriftRefusalNamesBothRestores asserts the message describes a
// recovery that actually clears the refusal.
//
// Only two things do, and both are version control: restoring the claim file
// (when its block was hand-edited) and restoring the digest store (when a commit
// carried the claim file without it). Nothing the engine runs clears it —
// `dossierx check`'s SweepCommentDigests adopts only ids the store has never
// seen, so a RECORDED digest that disagrees survives every command in the
// binary. The message therefore has to name both files.
func TestCommentDigestDriftRefusalNamesBothRestores(t *testing.T) {
	msg := driftRefusal(t)

	if !strings.Contains(msg, digest.StoreFileName) {
		t.Errorf("the refusal must name %s as one of the two files to restore: %q", digest.StoreFileName, msg)
	}
	if !strings.Contains(strings.ToLower(msg), "claim file") {
		t.Errorf("the refusal must name the claim file as the other side to restore: %q", msg)
	}
	if !strings.Contains(strings.ToLower(msg), "version control") {
		t.Errorf("the refusal must say the recovery is version control: %q", msg)
	}
}

// driftRefusal reproduces the wedge the same way TestCommentWriteRefusesA
// LaunderedDigest does — a hand-edited comment block on a covered claim — and
// returns the refusal's message, so the three assertions above read the real
// error rather than a copy of it.
func driftRefusal(t *testing.T) string {
	t.Helper()
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	p.addThread(model.CommentRoleHuman, "this contradicts the API facet")

	before := p.readAYAML()
	stripped := before[:bytes.Index(before, []byte("comments:"))]
	if err := os.WriteFile(filepath.Join(p.claimsDir, "a.yaml"), stripped, 0o644); err != nil {
		t.Fatalf("hand-edit the claim: %v", err)
	}

	_, _, err := p.deps().Add(claimA, model.CommentRoleAgent, "a write that would re-bless the edit")
	if !errors.Is(err, ErrCommentDigestDrift) {
		t.Fatalf("expected ErrCommentDigestDrift, got %v", err)
	}
	return err.Error()
}
