package textdiff

import (
	"math/rand"
	"strings"
	"testing"
)

func partsText(parts []Part) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p.Text)
	}
	return b.String()
}

func marked(parts []Part, op Op) []string {
	var out []string
	for _, p := range parts {
		if p.Op == op {
			out = append(out, strings.TrimSpace(p.Text))
		}
	}
	return out
}

// THE DEFECT THIS EXISTS FOR, in the shape it was measured in. The same
// one-word edit must produce the same answer whether or not the passage was
// re-wrapped afterwards.
func TestMarkWordsIsBlindToRewrapping(t *testing.T) {
	const approved = "**The field is outward, not internal.** A claim in the support authority already depends on it.\n" +
		"An observer revision changing is enumerated there among the material changes that revoke a\n" +
		"support verdict."

	noRewrap := strings.Replace(approved, "internal", "private", 1)
	rewrapped := "**The field is outward, not private.** A claim in the support authority already depends on\n" +
		"it. An observer revision changing is enumerated there among the material changes that revoke\n" +
		"a support verdict."

	for _, tc := range []struct{ name, after string }{
		{"wrapping untouched", noRewrap},
		{"passage re-wrapped", rewrapped},
	} {
		hunks := MarkWords(Blocks(approved, tc.after))
		if len(hunks) != 2 || hunks[0].Op != OpRemove || hunks[1].Op != OpAdd {
			t.Fatalf("%s: one passage changed, so one removal and one addition; got %d hunks", tc.name, len(hunks))
		}
		removed, added := marked(hunks[0].Parts, OpRemove), marked(hunks[1].Parts, OpAdd)
		if len(removed) != 1 || len(added) != 1 {
			t.Fatalf("%s: one word moved, so one marked run each way; got removed=%v added=%v", tc.name, removed, added)
		}
		if !strings.Contains(removed[0], "internal") || !strings.Contains(added[0], "private") {
			t.Fatalf("%s: the marked runs must name the word that moved; got %q -> %q", tc.name, removed[0], added[0])
		}
	}
}

// Each side keeps its OWN bytes. Nothing downstream has to know which side it
// is rendering, and a hunk can never show the other version's text.
func TestMarkWordsPartsRebuildTheirOwnHunk(t *testing.T) {
	cases := []struct{ before, after string }{
		{"one two three four five", "one two THREE four five"},
		{"a b c\nd e f", "a b c\nd e F"},
		{"alpha beta gamma delta epsilon", "alpha beta gamma inserted delta epsilon"},
		{"alpha beta gamma delta", "alpha gamma delta"},
		{"keep\nold one here", "keep\nold two here"},
	}
	for _, tc := range cases {
		for _, h := range MarkWords(Blocks(tc.before, tc.after)) {
			if len(h.Parts) == 0 {
				continue
			}
			if got := partsText(h.Parts); got != h.Text {
				t.Fatalf("%q -> %q: a %s hunk's parts rebuilt %q, want %q", tc.before, tc.after, h.Op, got, h.Text)
			}
			for _, p := range h.Parts {
				if p.Op != OpEqual && p.Op != h.Op {
					t.Fatalf("a %s hunk carried a %s part; a side never holds the other side's words", h.Op, p.Op)
				}
			}
		}
	}
}

func TestMarkWordsPartsRebuildRandomEdits(t *testing.T) {
	rng := rand.New(rand.NewSource(20260922))
	words := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "eta"}
	for n := 0; n < 300; n++ {
		var before []string
		for i := 0; i < 6+rng.Intn(40); i++ {
			before = append(before, words[rng.Intn(len(words))])
		}
		after := append([]string(nil), before...)
		for e := 0; e < 1+rng.Intn(6); e++ {
			at := rng.Intn(len(after))
			if rng.Intn(2) == 0 {
				after = append(after[:at], append([]string{words[rng.Intn(len(words))]}, after[at:]...)...)
			} else {
				after = append(after[:at], after[at+1:]...)
			}
			if len(after) == 0 {
				after = append(after, words[0])
			}
		}
		// Wrap both sides at independent widths, so the fixture exercises the
		// re-wrapping the marking exists to see through.
		b, a := wrap(before, 3+rng.Intn(5)), wrap(after, 3+rng.Intn(5))
		for _, h := range MarkWords(Blocks(b, a)) {
			if len(h.Parts) == 0 {
				continue
			}
			if got := partsText(h.Parts); got != h.Text {
				t.Fatalf("case %d: parts rebuilt %q, want %q", n, got, h.Text)
			}
		}
	}
}

func wrap(words []string, per int) string {
	var lines []string
	for i := 0; i < len(words); i += per {
		end := i + per
		if end > len(words) {
			end = len(words)
		}
		lines = append(lines, strings.Join(words[i:end], " "))
	}
	return strings.Join(lines, "\n")
}

// A rewrite is not an edit. Marking the words of two unrelated passages is a
// confetti with no through-line, and the two clean passages are easier to
// read undecorated.
func TestMarkWordsLeavesAnOutrightRewriteUnmarked(t *testing.T) {
	hunks := MarkWords(Blocks(
		"the authorization authority separates refused from not determined",
		"every published observation names the revision of its observer",
	))
	for _, h := range hunks {
		if len(h.Parts) != 0 {
			t.Fatalf("an outright rewrite must stay unmarked, got %+v", h.Parts)
		}
	}
}

func TestMarkWordsAboveTheWordBoundLeavesThePairAlone(t *testing.T) {
	long := strings.TrimSpace(strings.Repeat("word ", MaxMarkWords+1))
	for _, h := range MarkWords(Blocks(long, long+" tail")) {
		if len(h.Parts) != 0 {
			t.Fatal("a pair past MaxMarkWords must not be marked")
		}
	}
}

// A passage with nothing facing it has nothing to be marked against.
func TestMarkWordsLeavesUnpairedHunksAlone(t *testing.T) {
	hunks := MarkWords(Blocks("keep\n\ngone", "keep"))
	for _, h := range hunks {
		if len(h.Parts) != 0 {
			t.Fatalf("a pure deletion has no addition to mark against; got %+v", h.Parts)
		}
	}
}

func TestTokenizeReproducesItsInput(t *testing.T) {
	for _, s := range []string{
		"", "one", "one two", "  leading", "trailing  ", "a\nb\nc",
		"\n\nblank lines\n\n", "tabs\tand spaces  mixed\n",
	} {
		var b strings.Builder
		for _, tok := range tokenize(s) {
			b.WriteString(tok.text)
		}
		if b.String() != s {
			t.Fatalf("tokenize lost bytes for %q: rebuilt %q", s, b.String())
		}
	}
}
