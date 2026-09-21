package textdiff

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// reconstruct rebuilds one side from the hunks. Passages carry their own
// trailing blank lines, so there is no separator to re-insert — that is the
// property that makes this exact rather than approximate.
func reconstruct(hunks []Hunk, keep Op) string {
	var b strings.Builder
	for _, h := range hunks {
		if h.Op == OpEqual || h.Op == keep {
			b.WriteString(h.Text)
		}
	}
	return b.String()
}

func TestBlocksRoundTripsBothSides(t *testing.T) {
	cases := []struct{ before, after string }{
		{"", ""},
		{"one", "one"},
		{"", "added"},
		{"removed", ""},
		{"a\n\nb\n\nc", "a\n\nB\n\nc"},
		{"a\n\nb\n\nc", "c\n\nb\n\na"},
		{"intro\n\nold passage\n\noutro", "intro\n\nnew passage\n\noutro"},
		{"one\ntwo\nthree", "one\ntwo\nthree changed"},
		{"para\n\n\n\nspaced", "para\n\n\n\nspaced too"},
		{"trailing\n\n", "trailing\n\n"},
		{strings.Repeat("p\n\n", 20) + "tail", strings.Repeat("p\n\n", 20) + "different tail"},
	}
	for _, tc := range cases {
		hunks := Blocks(tc.before, tc.after)
		if got := reconstruct(hunks, OpRemove); got != tc.before {
			t.Fatalf("before round-trip failed for %q -> %q: rebuilt %q", tc.before, tc.after, got)
		}
		if got := reconstruct(hunks, OpAdd); got != tc.after {
			t.Fatalf("after round-trip failed for %q -> %q: rebuilt %q", tc.before, tc.after, got)
		}
	}
}

func TestBlocksRoundTripsRandomEdits(t *testing.T) {
	rng := rand.New(rand.NewSource(20260921))
	for n := 0; n < 300; n++ {
		var before []string
		for i := 0; i < rng.Intn(12); i++ {
			before = append(before, fmt.Sprintf("passage-%d line one\npassage-%d line two", rng.Intn(6), rng.Intn(6)))
		}
		after := append([]string(nil), before...)
		for e := 0; e < rng.Intn(6); e++ {
			if len(after) == 0 || rng.Intn(2) == 0 {
				at := rng.Intn(len(after) + 1)
				after = append(after[:at], append([]string{fmt.Sprintf("new-%d", rng.Intn(6))}, after[at:]...)...)
				continue
			}
			at := rng.Intn(len(after))
			after = append(after[:at], after[at+1:]...)
		}
		b, a := strings.Join(before, "\n\n"), strings.Join(after, "\n\n")
		hunks := Blocks(b, a)
		if got := reconstruct(hunks, OpRemove); got != b {
			t.Fatalf("seeded case %d: before round-trip rebuilt %q, want %q", n, got, b)
		}
		if got := reconstruct(hunks, OpAdd); got != a {
			t.Fatalf("seeded case %d: after round-trip rebuilt %q, want %q", n, got, a)
		}
	}
}

func TestSplitBlocksReproducesItsInput(t *testing.T) {
	for _, s := range []string{
		"", "one", "one\ntwo", "a\n\nb", "a\n\n\n\nb", "\n\nleading blanks\n\n",
		"trailing\n", "   \n\nspacey\n   \n\nend",
	} {
		var b strings.Builder
		for _, block := range splitBlocks(s) {
			b.WriteString(block)
		}
		if b.String() != s {
			t.Fatalf("splitBlocks lost bytes for %q: rebuilt %q", s, b.String())
		}
	}
}

// A passage is the unit, so one changed passage is one removal and one
// addition — never one hunk per line inside it.
func TestBlocksKeepsAChangedPassageWhole(t *testing.T) {
	before := "intro stays\n\nthe passage\nruns over\nthree lines\n\noutro stays"
	after := "intro stays\n\nthe passage\nnow runs over\nthree lines\n\noutro stays"
	var ops []Op
	for _, h := range Blocks(before, after) {
		ops = append(ops, h.Op)
	}
	want := []Op{OpEqual, OpRemove, OpAdd, OpEqual}
	if len(ops) != len(want) {
		t.Fatalf("got %v, want %v", ops, want)
	}
	for i := range want {
		if ops[i] != want[i] {
			t.Fatalf("hunk %d is %q, want %q (full: %v)", i, ops[i], want[i], ops)
		}
	}
}

func TestBlocksAboveTheBoundFallsBackToWholeBody(t *testing.T) {
	before := strings.Repeat("p\n\n", MaxBlocks+1)
	after := before + "tail"
	hunks := Blocks(before, after)
	if len(hunks) != 2 || hunks[0].Op != OpRemove || hunks[1].Op != OpAdd {
		t.Fatalf("expected a coarse two-hunk answer above the bound, got %d hunks", len(hunks))
	}
	if reconstruct(hunks, OpRemove) != before || reconstruct(hunks, OpAdd) != after {
		t.Fatal("the coarse answer must still round-trip both sides")
	}
}

func TestBlocksIdenticalInputIsOneEqualHunk(t *testing.T) {
	hunks := Blocks("same\n\ntext", "same\n\ntext")
	if len(hunks) != 1 || hunks[0].Op != OpEqual || hunks[0].Text != "same\n\ntext" {
		t.Fatalf("identical bodies must report one equal hunk, got %+v", hunks)
	}
	if Blocks("", "") != nil {
		t.Fatal("two empty bodies must report no hunks")
	}
}
