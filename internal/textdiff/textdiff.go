// Package textdiff computes the difference between two versions of a claim's
// prose, for the one question the lock ledger could answer with a yes or no
// and not with a sentence: what actually changed.
//
// THE UNIT IS A PASSAGE, NOT A LINE, and that is the decision everything else
// here follows from.
//
// Claim bodies are markdown hard-wrapped by whatever the author's editor
// does. Diffed by physical line, changing one word inside a paragraph renders
// as one line struck and one added if the wrapping happened to survive, and
// as three struck and three added if the paragraph re-wrapped — the same
// shape a rewritten paragraph makes. Measured on the Curtainly claim
// permission-readiness.contract.the-observer-revision-is-published, changing
// `internal` to `private`: one line each way, or three each way, decided by
// nothing but line breaks. A line is also not a thing markdown can render:
// half a list item or the opening of a blockquote is not a document, so a
// line-shaped hunk could never be shown as prose.
//
// A passage is both. It survives re-wrapping, and it is a block a markdown
// renderer can take whole. docs/design's own board says the same in its own
// words — "1 passage differs from the approval".
//
// Inside a changed passage, MarkWords then says WHICH WORDS moved, so a
// reader comparing the old passage against the new one is not left to find
// the difference by eye.
//
// Cost is stated rather than assumed. Both diffs are the textbook LCS dynamic
// program: O(n*m) time and O(n*m) int16 cells over passages, and again over
// the words of one changed pair. Prose is small and the bounds below make
// that a promise rather than a hope. Nothing here is reachable from graph
// traversal: it runs once per claim a reader opens, never per edge.
package textdiff

import "strings"

// MaxBlocks is the per-side passage count above which Blocks stops computing
// an LCS and reports a whole-body replacement instead.
//
// 400 passages is far beyond any claim body this engine has seen — claims are
// atomic by construction, one assertion each — and keeps the table at 400*400
// int16, about 320KB, transient and per-call. The point of the bound is that
// a generated or pasted file cannot turn one reader opening one claim into a
// multi-second allocation storm.
const MaxBlocks = 400

// MaxMarkWords is the per-side word count above which MarkWords leaves a
// passage pair unmarked. 1500 words is past the longest claim in the
// Curtainly corpus by more than a factor of two and keeps its own table at
// 1500*1500 int16, about 4.5MB, transient and per-pair.
const MaxMarkWords = 1500

// MinMarkSimilarity is how much of the larger side must survive unchanged for
// a pair to be worth marking word by word.
//
// Below it the pair is a REWRITE, not an edit: marking the words of two
// unrelated passages produces a confetti with no through-line, and the two
// clean passages it decorates are easier to read undecorated. A third is
// where an inline mark stops being a sentence a reader can follow.
const MinMarkSimilarity = 0.34

// Op is what happened to a passage, or to a run of words inside one.
type Op string

const (
	// OpEqual: present, unchanged, in both versions.
	OpEqual Op = "equal"
	// OpRemove: in the approved version and gone.
	OpRemove Op = "remove"
	// OpAdd: in the current version and not approved.
	OpAdd Op = "add"
)

// Part is one run of words inside a hunk. On a removal hunk its Op is OpEqual
// or OpRemove; on an addition hunk, OpEqual or OpAdd. Concatenating every
// Part.Text reproduces the hunk's own Text exactly.
type Part struct {
	Op   Op     `json:"op"`
	Text string `json:"text"`
}

// Hunk is one passage, with the op that happened to it.
//
// Text carries the passage INCLUDING the blank lines that follow it, so
// concatenating a side's hunks reproduces that side byte for byte with no
// separator logic anywhere. A renderer trims it; a reconstruction does not.
type Hunk struct {
	Op   Op     `json:"op"`
	Text string `json:"text"`

	// Parts is the word-level detail of this passage against the passage it
	// replaced or was replaced by, filled by MarkWords. Empty on an unchanged
	// passage, on a passage with nothing facing it, and on a pair too
	// dissimilar to be worth marking.
	Parts []Part `json:"parts,omitempty"`
}

// Blocks returns the difference between before and after as an ordered run of
// passage hunks.
//
// Concatenating every hunk whose Op is not OpAdd reproduces before exactly;
// concatenating every hunk whose Op is not OpRemove reproduces after exactly.
// That round trip is the only statement about a diff a reader can rely on
// without re-reading it, so it is what the tests assert.
//
// Identical inputs return a single OpEqual hunk rather than nil, so a caller
// never has to distinguish "no difference" from "nothing computed".
func Blocks(before, after string) []Hunk {
	if before == after {
		if before == "" {
			return nil
		}
		return []Hunk{{Op: OpEqual, Text: before}}
	}
	a, b := splitBlocks(before), splitBlocks(after)
	if len(a) > MaxBlocks || len(b) > MaxBlocks {
		return coarse(before, after)
	}
	return merge(script(a, b), a, b)
}

// coarse is the answer above MaxBlocks: one removal of everything and one
// addition of everything. A worse diff, not a wrong one, and it says so by
// being exactly what a reader would write by hand if they gave up.
func coarse(before, after string) []Hunk {
	var out []Hunk
	if before != "" {
		out = append(out, Hunk{Op: OpRemove, Text: before})
	}
	if after != "" {
		out = append(out, Hunk{Op: OpAdd, Text: after})
	}
	return out
}

// splitBlocks cuts a body into passages at blank lines, KEEPING each
// passage's trailing blank lines on it. Concatenating the result reproduces
// the input byte for byte — which is what lets Hunk.Text need no separator
// rule and lets the round trip above be exact rather than approximate.
func splitBlocks(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	var out []string
	var cur []string
	inBlank := false
	for i, line := range lines {
		blank := strings.TrimSpace(line) == ""
		if blank {
			inBlank = true
			cur = append(cur, line)
			continue
		}
		if inBlank && len(cur) > 0 {
			out = append(out, strings.Join(cur, "\n")+"\n")
			cur = nil
			inBlank = false
		}
		cur = append(cur, line)
		_ = i
	}
	if len(cur) > 0 {
		out = append(out, strings.Join(cur, "\n"))
	}
	return out
}

// step is one element of the edit script, before consecutive runs are merged.
type step struct {
	op Op
	// index into a for equal/remove, into b for add.
	i int
}

// script walks the LCS table back to front to produce the edit script. The
// table is int16: a body with more than 32767 passages is already past
// MaxBlocks by two orders of magnitude, so the narrower cell is free.
func script(a, b []string) []step {
	n, m := len(a), len(b)
	lcs := make([]int16, (n+1)*(m+1))
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i*(m+1)+j] = lcs[(i+1)*(m+1)+j+1] + 1
				continue
			}
			down, right := lcs[(i+1)*(m+1)+j], lcs[i*(m+1)+j+1]
			if down >= right {
				lcs[i*(m+1)+j] = down
			} else {
				lcs[i*(m+1)+j] = right
			}
		}
	}

	var out []step
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			out = append(out, step{OpEqual, i})
			i++
			j++
		case lcs[(i+1)*(m+1)+j] >= lcs[i*(m+1)+j+1]:
			// Removal first on a tie, so a replaced passage renders as the
			// old text above the new text — the reading order the boards draw
			// and the order a reviewer narrates it in.
			out = append(out, step{OpRemove, i})
			i++
		default:
			out = append(out, step{OpAdd, j})
			j++
		}
	}
	for ; i < n; i++ {
		out = append(out, step{OpRemove, i})
	}
	for ; j < m; j++ {
		out = append(out, step{OpAdd, j})
	}
	return out
}

// merge turns the edit script into one Hunk per passage. Consecutive
// same-op passages are NOT merged: each is a block a markdown renderer takes
// whole, and gluing two together would hand it a document it never saw.
func merge(steps []step, a, b []string) []Hunk {
	out := make([]Hunk, 0, len(steps))
	for _, s := range steps {
		text := ""
		if s.op == OpAdd {
			text = b[s.i]
		} else {
			text = a[s.i]
		}
		out = append(out, Hunk{Op: s.op, Text: text})
	}
	return out
}
