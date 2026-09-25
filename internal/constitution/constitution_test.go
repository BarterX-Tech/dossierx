package constitution

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

const sample = `status: draft
invariants:
  - slug: single-roof
    title: One roof
    body: Every module builds toward this file.
glossary:
  - slug: claim
    body: One reviewable fact, in one file.
decisions:
  - slug: no-refs
    title: Claims never cite the constitution
    body: It is unsaid context for every claim.
`

func write(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func mustParse(t *testing.T, body, name string) *File {
	t.Helper()
	f, err := Parse([]byte(body), name)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestParseAcceptsTheThreeSectionsAndDefaultsToDraft(t *testing.T) {
	f, err := Parse([]byte(sample), "constitution.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if f.Status != model.StatusDraft {
		t.Fatalf("status = %q, want draft", f.Status)
	}
	if len(f.Invariants) != 1 || len(f.Glossary) != 1 || len(f.Decisions) != 1 {
		t.Fatalf("sections = %d/%d/%d", len(f.Invariants), len(f.Glossary), len(f.Decisions))
	}
	if got := Sections(f); len(got) != 3 || got[0].Name != SectionInvariants || got[2].Name != SectionDecisions {
		t.Fatalf("Sections = %+v", got)
	}
}

func TestParseRefusesUnknownKeysBadStatusMissingSlugAndDuplicates(t *testing.T) {
	for name, body := range map[string]string{
		"unknown key":    "status: draft\nmodules: [a]\n",
		"bad status":     "status: pending\n",
		"missing slug":   "status: draft\ninvariants:\n  - body: no slug\n",
		"duplicate slug": "status: draft\ninvariants:\n  - slug: a\n    body: one\n  - slug: a\n    body: two\n",
		"two documents":  "status: draft\n---\nstatus: draft\n",
	} {
		if _, err := Parse([]byte(body), name); err == nil {
			t.Errorf("%s: expected a parse error", name)
		}
	}
}

func TestLoadDistinguishesMissingFromUnreadable(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(filepath.Join(dir, "nope.yaml")); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing file must wrap ErrNotFound, got %v", err)
	}
	if f, err := LoadOptional(filepath.Join(dir, "nope.yaml")); err != nil || f != nil {
		t.Fatalf("LoadOptional on a missing file = %v, %v", f, err)
	}
	p := write(t, dir, "constitution.yaml", "status: draft\nmodules: [a]\n")
	if _, err := Load(p); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("an unreadable file is not a missing one: %v", err)
	}
}

func TestWordCountCountsTitlesAndBodiesNotSlugs(t *testing.T) {
	f := mustParse(t, sample, "c")
	// "One roof" (2) + "Every module builds toward this file." (6) +
	// "One reviewable fact, in one file." (6) +
	// "Claims never cite the constitution" (5) + "It is unsaid context for every claim." (7)
	if got := WordCount(f); got != 26 {
		t.Fatalf("WordCount = %d, want 26 (slugs are not words)", got)
	}
	// Letter/number runs only: hyphens and underscores split a run, so
	// "a-b" and "c_d" are two words each.
	if CountWords("a-b c_d 12 é") != 6 {
		t.Fatalf("CountWords splits on non-letter/number runs: %d", CountWords("a-b c_d 12 é"))
	}
	if WordCount(nil) != 0 {
		t.Fatal("nil file has no words")
	}
}

func TestCapsAndNearCapBand(t *testing.T) {
	entry := func(n int) *File {
		return &File{Invariants: []Entry{{Slug: "x", Body: strings.Repeat("word ", n)}}}
	}
	if OverCap(entry(WordCap)) || !OverCap(entry(WordCap+1)) {
		t.Fatal("OverCap is strictly over 800")
	}
	if IsNearCap(entry(NearCap-1)) || !IsNearCap(entry(NearCap)) || !IsNearCap(entry(WordCap)) || IsNearCap(entry(WordCap+1)) {
		t.Fatal("near-cap band is [720, 800]")
	}
	d := NewDigest("c.yaml", entry(WordCap+1))
	if !d.OverCap || d.Words != WordCap+1 || d.WordCap != WordCap || !d.Present {
		t.Fatalf("Digest = %+v", d)
	}
}

func TestHashIgnoresStatusAndMovesOnContent(t *testing.T) {
	a := mustParse(t, sample, "c")
	b := mustParse(t, strings.Replace(sample, "status: draft", "status: locked", 1), "c")
	if Hash(a) != Hash(b) {
		t.Fatal("status must not move the content hash: the lock flips it")
	}
	c := mustParse(t, strings.Replace(sample, "One roof", "Two roofs", 1), "c")
	if Hash(a) == Hash(c) {
		t.Fatal("editing a title must move the hash")
	}
	if Hash(nil) != "" {
		t.Fatal("nil file has no hash")
	}
}

func TestTextIsTheWordsInSectionOrder(t *testing.T) {
	f := mustParse(t, sample, "c")
	text := Text(f)
	for _, want := range []string{"# Invariants", "## One roof (single-roof)", "Every module builds toward this file.", "# Glossary", "## claim", "# Decisions", "## Claims never cite the constitution (no-refs)"} {
		if !strings.Contains(text, want) {
			t.Errorf("Text lacks %q:\n%s", want, text)
		}
	}
	if strings.Index(text, "# Invariants") > strings.Index(text, "# Glossary") || strings.Index(text, "# Glossary") > strings.Index(text, "# Decisions") {
		t.Fatal("sections must come out in file order")
	}
	if Text(&File{}) != "" {
		t.Fatal("an empty file has no text; empty sections are omitted")
	}
}

func TestEvaluateStates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "constitution.yaml")

	v := EvaluateAt(path, nil)
	if v.State != StateMissing || v.Locked() {
		t.Fatalf("missing file: %+v", v)
	}
	write(t, dir, "constitution.yaml", "status: draft\nmodules: [a]\n")
	if v := EvaluateAt(path, nil); v.State != StateUnreadable || v.Error == "" {
		t.Fatalf("unreadable file: %+v", v)
	}
	write(t, dir, "constitution.yaml", sample)
	if v := EvaluateAt(path, nil); v.State != StateDraft {
		t.Fatalf("draft file: %+v", v)
	}
	locked := strings.Replace(sample, "status: draft", "status: locked", 1)
	write(t, dir, "constitution.yaml", locked)
	if v := EvaluateAt(path, nil); v.State != StateUnrecorded {
		t.Fatalf("locked file, no record: %+v", v)
	}
	f := mustParse(t, locked, path)
	rec := &LockRecord{Hash: Hash(f), Reason: "yes", LockedAt: "2026-09-24T00:00:00Z"}
	v = EvaluateAt(path, rec)
	if v.State != StateLocked || !v.Locked() || v.StoredHash != rec.Hash || v.Reason != "yes" {
		t.Fatalf("locked and matching: %+v", v)
	}
	write(t, dir, "constitution.yaml", strings.Replace(locked, "One roof", "Two roofs", 1))
	v = EvaluateAt(path, rec)
	if v.State != StateEdited || v.Locked() || v.Hash == v.StoredHash {
		t.Fatalf("edited after lock: %+v", v)
	}
	for _, s := range []State{StateMissing, StateUnreadable, StateDraft, StateUnrecorded, StateEdited} {
		if (Verdict{State: s, Path: path}).Hint() == "" {
			t.Errorf("state %s has no recovery hint", s)
		}
	}
}

func TestOverCapIsReportedOnALockedRoofToo(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "constitution.yaml")
	big := "status: locked\ninvariants:\n  - slug: x\n    body: " + strings.Repeat("word ", WordCap+1) + "\n"
	write(t, dir, "constitution.yaml", big)
	f := mustParse(t, big, path)
	v := EvaluateAt(path, &LockRecord{Hash: Hash(f)})
	if v.State != StateLocked || !v.OverCap {
		t.Fatalf("a locked roof over the cap is locked AND over cap: %+v", v)
	}
}
