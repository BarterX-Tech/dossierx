package briefs

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// testConfig decodes a minimal project config anchored at dir, with extra
// appended verbatim, through the real strict decoder.
func testConfig(t testing.TB, dir, extra string) *config.Config {
	t.Helper()
	raw := "schema_version: 1\nfacets: [contract, internals]\nmodules: [widget]\nclaims_dir: claims\n" + extra
	cfg, err := config.DecodeConfig([]byte(raw), dir, "project.config.yaml")
	if err != nil {
		t.Fatalf("decode config: %v", err)
	}
	return cfg
}

const okFront = "---\nsummary: A brief used by the test corpus.\n---\n"

func md(body string) File {
	return File{Regular: true, Data: []byte(body), Size: int64(len(body))}
}

func tree(files map[string]File) []File {
	out := make([]File, 0, len(files))
	for rel, f := range files {
		f.Rel = rel
		out = append(out, f)
	}
	return out
}

// rulesAndPaths flattens findings into "rule path" strings, sorted, which is
// the whole of what these tests assert about a finding besides its message.
func rulesAndPaths(fs []lint.Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.LintName+" "+f.ClaimID)
	}
	sort.Strings(out)
	return out
}

// TestLoad_AbsentBriefsDirIsAProjectWithNoBriefs pins the zero-cost contract:
// a project that never wrote briefs/ gets an empty set with no finding, which
// is what lets every consumer (check, the payload) change nothing for it.
func TestLoad_AbsentBriefsDirIsAProjectWithNoBriefs(t *testing.T) {
	dir := t.TempDir()
	s := Load(testConfig(t, dir, ""))
	if !s.Empty() {
		t.Fatalf("expected no briefs, got %+v", s.Briefs)
	}
	if got := s.Findings(nil); len(got) != 0 {
		t.Fatalf("an absent briefs/ must raise nothing, got %v", got)
	}
}

// TestLoad_ReadsTheWorkingTreeAndSkipsHiddenEntries drives Load over a real
// directory: the brief is found with its id and path, and the OS litter the
// package doc exempts (.DS_Store, a dot-directory) raises nothing.
func TestLoad_ReadsTheWorkingTreeAndSkipsHiddenEntries(t *testing.T) {
	dir := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(dir, "briefs", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("checkout/flow.md", okFront+"# Checkout flow\n\nWords here.\n")
	write(".DS_Store", "junk")
	write("checkout/.flow.md.swp", "junk")
	write(".git-ish/anything.txt", "junk")

	s := Load(testConfig(t, dir, ""))
	if got := s.Findings(nil); len(got) != 0 {
		t.Fatalf("hidden entries must be ignored, got %v", rulesAndPaths(got))
	}
	if len(s.Briefs) != 1 || s.Briefs[0].ID != "checkout.flow" || s.Briefs[0].Path != "briefs/checkout/flow.md" {
		t.Fatalf("unexpected briefs: %+v", s.Briefs)
	}
}

// TestLoad_SymlinkedTreeIsRefusedNotEmpty pins that a link where the tree or a
// folder should be is a brief-shape finding with a message that says what it
// is. A symlinked briefs_dir used to read as a project with no briefs (Stat
// followed the link, WalkDir would not descend it): the briefs behind it were
// never judged and `check` passed. A symlinked folder was reported as "a file
// directly under briefs/", which is not what the author made.
func TestLoad_SymlinkedTreeIsRefusedNotEmpty(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink fixture requires Unix symlink semantics; TestFromFiles_ShapeRefusals covers the rule on every platform")
	}
	realTree := func(t *testing.T, dir string) string {
		t.Helper()
		target := filepath.Join(dir, "elsewhere")
		if err := os.MkdirAll(filepath.Join(target, "checkout"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(target, "checkout", "flow.md"), []byte(okFront+"text\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		return target
	}

	t.Run("briefs_dir", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Symlink(realTree(t, dir), filepath.Join(dir, "briefs")); err != nil {
			t.Fatal(err)
		}
		got := Load(testConfig(t, dir, "")).Findings(nil)
		if len(got) != 1 || got[0].LintName != RuleShape || got[0].ClaimID != "briefs/" || !strings.Contains(got[0].Message, "symlink") {
			t.Fatalf("a symlinked briefs_dir must be one brief-shape finding on briefs/ naming the link, got %+v", got)
		}
	})

	t.Run("folder", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(realTree(t, dir), "checkout")
		if err := os.MkdirAll(filepath.Join(dir, "briefs"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(dir, "briefs", "checkout")); err != nil {
			t.Fatal(err)
		}
		got := Load(testConfig(t, dir, "")).Findings(nil)
		if len(got) != 1 || got[0].LintName != RuleShape || got[0].ClaimID != "briefs/checkout" {
			t.Fatalf("a symlinked folder must be one brief-shape finding on its path, got %+v", got)
		}
		if strings.Contains(got[0].Message, "a file directly under") || !strings.Contains(got[0].Message, "symlink") {
			t.Fatalf("the message must say the folder is a link, not a loose file: %q", got[0].Message)
		}
	})
}

// TestLoad_AnUnreadableEntryIsOneFindingNotAnEmptyTree pins that a folder or a
// brief the engine cannot read is a brief-shape finding on that entry, and that
// every other folder is still read. The walk used to abort on the first read
// error and replace the whole set with one tree finding, so one unreadable
// folder dropped every brief the project held.
func TestLoad_AnUnreadableEntryIsOneFindingNotAnEmptyTree(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs POSIX permissions enforced: an unreadable directory is still readable on Windows and to root")
	}
	dir := t.TempDir()
	write := func(rel, body string) {
		t.Helper()
		p := filepath.Join(dir, "briefs", filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("alpha/one.md", okFront+"text\n")
	write("locked/two.md", okFront+"text\n")
	write("zeta/three.md", okFront+"text\n")
	write("zeta/four.md", okFront+"text\n")
	locked := filepath.Join(dir, "briefs", "locked")
	unreadableFile := filepath.Join(dir, "briefs", "zeta", "four.md")
	for _, p := range []string{locked, unreadableFile} {
		if err := os.Chmod(p, 0o000); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		os.Chmod(locked, 0o755)         //nolint:errcheck // best-effort restore for TempDir cleanup
		os.Chmod(unreadableFile, 0o644) //nolint:errcheck // best-effort restore for TempDir cleanup
	})

	s := Load(testConfig(t, dir, ""))
	var ids []string
	for _, b := range s.Briefs {
		ids = append(ids, b.ID)
	}
	if strings.Join(ids, ",") != "alpha.one,zeta.three" {
		t.Fatalf("the readable briefs must still be read, got %v", ids)
	}
	got := s.TreeFindings()
	if want := []string{"brief-shape briefs/locked/", "brief-shape briefs/zeta/four.md"}; !reflect.DeepEqual(rulesAndPaths(got), want) {
		t.Fatalf("findings = %v, want %v", rulesAndPaths(got), want)
	}
	for _, f := range got {
		if !strings.Contains(f.Message, "could not be read") || strings.Contains(f.Message, dir) {
			t.Fatalf("an unreadable entry's message must say so without the absolute path: %q", f.Message)
		}
	}

	// briefs_dir itself unreadable — its listing refused (the walk's root
	// error), or the path to it not a directory (Lstat's error) — is the
	// whole-tree finding on briefs/, and its message carries no absolute path
	// either.
	root := filepath.Join(dir, "briefs")
	if err := os.Chmod(root, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(root, 0o755) }) //nolint:errcheck // best-effort restore for TempDir cleanup
	notADir := filepath.Join(dir, "file")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, cfg := range map[string]*config.Config{
		"listing refused": testConfig(t, dir, ""),
		"not a directory": testConfig(t, dir, "briefs_dir: file/briefs\n"),
	} {
		got := Load(cfg).TreeFindings()
		if len(got) != 1 || got[0].LintName != RuleShape || !strings.HasSuffix(got[0].ClaimID, "briefs/") ||
			!strings.Contains(got[0].Message, "briefs_dir could not be read") || strings.Contains(got[0].Message, dir) {
			t.Fatalf("%s: briefs_dir unreadable must be one brief-shape finding on the tree with no absolute path, got %+v", name, got)
		}
	}
}

// TestFromFiles_ShapeRefusals is the brief-shape rule's whole refusal list, one
// tree per case, each asserting the exact rule and path the finding names.
func TestFromFiles_ShapeRefusals(t *testing.T) {
	cfg := testConfig(t, t.TempDir(), "")
	for _, tc := range []struct {
		name  string
		files map[string]File
		want  []string
	}{
		{"loose file under briefs/", map[string]File{"readme.md": md(okFront)}, []string{"brief-shape briefs/readme.md"}},
		{"deeper than one folder", map[string]File{"a/b/c.md": md(okFront)}, []string{"brief-shape briefs/a/b/c.md"}},
		{"any other file", map[string]File{"a/notes.txt": {Regular: true, Size: 3}}, []string{"brief-shape briefs/a/notes.txt"}},
		{"uppercase extension is another file", map[string]File{"a/x.MD": md(okFront)}, []string{"brief-shape briefs/a/x.MD"}},
		{"folder name outside the set", map[string]File{"Check_Out/x.md": md(okFront)}, []string{"brief-shape briefs/Check_Out/x.md"}},
		{"file name outside the set", map[string]File{"a/My Brief.md": md(okFront)}, []string{"brief-shape briefs/a/My Brief.md"}},
		{"non-regular file", map[string]File{"a/x.md": {}}, []string{"brief-shape briefs/a/x.md"}},
		{"non-regular folder (a symlink or a gitlink)", map[string]File{"a": {}}, []string{"brief-shape briefs/a"}},
		{"briefs_dir itself a link", map[string]File{".": {}}, []string{"brief-shape briefs/"}},
		{"briefs_dir itself a file", map[string]File{".": {Regular: true}}, []string{"brief-shape briefs"}},
		{"image nothing references", map[string]File{"a/x.md": md(okFront + "text\n"), "a/orphan.png": {Regular: true, Size: 10}}, []string{"brief-shape briefs/a/orphan.png"}},
		{"image referenced from another folder is still unreferenced", map[string]File{"a/x.md": md(okFront + "![f](f.png)\n"), "b/f.png": {Regular: true, Size: 10}}, []string{"brief-shape briefs/a/x.md", "brief-shape briefs/b/f.png"}},
		{"referenced and present is clean", map[string]File{"a/x.md": md(okFront + "![f](f.png)\n"), "a/f.png": {Regular: true, Size: 10}}, nil},
		{"an uppercase image src the gate refuses", map[string]File{"a/x.md": md(okFront + "![p](PIC.svg)\n")}, []string{"brief-shape briefs/a/x.md"}},
		{"a ./ image src the gate refuses", map[string]File{"a/x.md": md(okFront + "![p](./pic.svg)\n")}, []string{"brief-shape briefs/a/x.md"}},
		{"a ../ image src the gate refuses", map[string]File{"a/x.md": md(okFront + "![p](../x.svg)\n")}, []string{"brief-shape briefs/a/x.md"}},
		{"an image inside a fenced example is not an image", map[string]File{"a/x.md": md(okFront + "```\n![p](PIC.svg)\n```\n")}, nil},
		{"hidden names are not read", map[string]File{".DS_Store": {Regular: true}, "a/.keep": {Regular: true}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := rulesAndPaths(FromFiles(cfg, tree(tc.files)).Findings(nil))
			if len(got) == 0 {
				got = nil
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("findings = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestParseFrontmatter pins the strict decode: the block's one spelling, the
// three keys and their shapes, and the summary ceiling.
func TestParseFrontmatter(t *testing.T) {
	long := strings.Repeat("x", MaxSummaryChars+1)
	for _, tc := range []struct {
		name     string
		in       string
		problems int
		status   string
		restsOn  []string
	}{
		{"minimal", okFront, 0, "", nil},
		{"every field", "---\nsummary: s\nstatus: locked\nrests_on:\n  - widget.contract.a\n---\nbody\n", 0, "locked", []string{"widget.contract.a"}},
		{"no block", "# Title\n", 1, "", nil},
		{"unclosed block", "---\nsummary: s\n", 1, "", nil},
		{"empty block", "---\n---\n", 1, "", nil},
		{"unknown key", "---\nsummary: s\nowner: me\n---\n", 1, "", nil},
		{"summary not a string", "---\nsummary: 5\n---\n", 1, "", nil},
		{"summary blank", "---\nsummary: \"  \"\n---\n", 1, "", nil},
		{"summary too long", "---\nsummary: " + long + "\n---\n", 1, "", nil},
		{"summary at the ceiling", "---\nsummary: " + long[1:] + "\n---\n", 0, "", nil},
		{"status outside the enum", "---\nsummary: s\nstatus: done\n---\n", 1, "", nil},
		{"status as a boolean", "---\nsummary: s\nstatus: yes\n---\n", 1, "", nil},
		{"rests_on not a list", "---\nsummary: s\nrests_on: widget.contract.a\n---\n", 1, "", nil},
		{"rests_on repeats an id", "---\nsummary: s\nrests_on: [a.b.c, a.b.c]\n---\n", 1, "", []string{"a.b.c"}},
		{"not a mapping", "---\n- a\n---\n", 1, "", nil},
		{"invalid yaml", "---\nsummary: [unclosed\n---\n", 1, "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fm, _, problems := parseFrontmatter(tc.in)
			if len(problems) != tc.problems {
				t.Fatalf("problems = %q, want %d", problems, tc.problems)
			}
			if fm.status != tc.status || !reflect.DeepEqual(fm.restsOn, tc.restsOn) {
				t.Fatalf("frontmatter = %+v, want status %q rests_on %v", fm, tc.status, tc.restsOn)
			}
		})
	}
}

// TestFromFiles_BriefFields pins what a Brief carries: the id and path, the
// title from the first level-1 heading or else the slug, the default status,
// the body after the frontmatter, the word count, the de-duplicated image list,
// and a digest that does not move with line endings.
func TestFromFiles_BriefFields(t *testing.T) {
	cfg := testConfig(t, t.TempDir(), "")
	lf := okFront + "Intro ![A](a.png).\n\n# Checkout flow\n\nSee ![A again](a.png) and ![B](b.svg).\n"
	s := FromFiles(cfg, tree(map[string]File{
		"checkout/flow.md":       md(lf),
		"checkout/a.png":         {Regular: true, Size: 7},
		"checkout/b.svg":         {Regular: true, Size: 9},
		"checkout/order-plan.md": md(okFront + "no heading here\n"),
	}))
	if got := s.Findings(nil); len(got) != 0 {
		t.Fatalf("unexpected findings: %v", rulesAndPaths(got))
	}
	flow, ok := s.Lookup("briefs/checkout/flow.md")
	if !ok {
		t.Fatal("lookup by path failed")
	}
	if byID, _ := s.Lookup("checkout.flow"); byID.Path != flow.Path {
		t.Fatal("lookup by id must find the same brief")
	}
	if flow.ID != "checkout.flow" || flow.Folder != "checkout" || flow.Title != "Checkout flow" || flow.Status != StatusDraft {
		t.Fatalf("unexpected brief: %+v", flow)
	}
	if strings.Contains(flow.Body, "summary:") {
		t.Fatalf("body must not carry the frontmatter: %q", flow.Body)
	}
	// Intro / Checkout flow / See ... and: the image references are not words.
	if flow.Words != 5 {
		t.Fatalf("words = %d, want 5", flow.Words)
	}
	wantImages := []Image{{Name: "a.png", Bytes: 7, Present: true}, {Name: "b.svg", Bytes: 9, Present: true}}
	if !reflect.DeepEqual(flow.Images, wantImages) {
		t.Fatalf("images = %+v, want %+v", flow.Images, wantImages)
	}
	plan, _ := s.Lookup("checkout.order-plan")
	if plan.Title != "Order Plan" {
		t.Fatalf("fallback title = %q, want %q", plan.Title, "Order Plan")
	}
	if !reflect.DeepEqual(s.Folders, []Folder{{Name: "checkout", Count: 2}}) {
		t.Fatalf("folders = %+v", s.Folders)
	}

	crlf := FromFiles(cfg, tree(map[string]File{"checkout/flow.md": md(strings.ReplaceAll(lf, "\n", "\r\n")), "checkout/a.png": {Regular: true}, "checkout/b.svg": {Regular: true}}))
	if crlf.Briefs[0].Digest != flow.Digest {
		t.Fatal("a CRLF checkout of the same brief must carry the same digest")
	}
}

// TestFromFiles_CapsFollowTheirOverrides drives each cap past a lowered
// override, which is also the proof that each override key is read: a cap that
// ignored its key would stay at the default and raise nothing here.
func TestFromFiles_CapsFollowTheirOverrides(t *testing.T) {
	cfg := testConfig(t, t.TempDir(), "max_brief_words: 3\nmax_brief_images: 1\nmax_brief_image_bytes: 100\nmax_briefs_per_folder: 1\nmax_briefs: 2\n")
	s := FromFiles(cfg, tree(map[string]File{
		"a/one.md":   md(okFront + "one two three four\n"),
		"a/two.md":   md(okFront + "![x](x.png) ![y](y.png)\n"),
		"a/x.png":    {Regular: true, Size: 101},
		"a/y.png":    {Regular: true, Size: 5},
		"b/three.md": md(okFront + "ok\n"),
	}))
	got := rulesAndPaths(s.Findings(nil))
	want := []string{
		"brief-folder-cap briefs/a/",
		"brief-image-cap briefs/a/two.md",
		"brief-image-cap briefs/a/two.md",
		"brief-total-cap briefs/",
		"brief-word-cap briefs/a/one.md",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %v, want %v", got, want)
	}
	for _, f := range s.Findings(nil) {
		if !strings.Contains(f.Message, "the human's call, only on their explicit approval") {
			t.Errorf("cap finding %s must say raising the cap is the human's call: %s", f.LintName, f.Message)
		}
	}

	defaults := FromFiles(testConfig(t, t.TempDir(), ""), tree(map[string]File{"a/one.md": md(okFront + "one two three four\n")}))
	if got := defaults.Findings(nil); len(got) != 0 {
		t.Fatalf("the defaults must not fire on a small brief: %v", rulesAndPaths(got))
	}
}

// TestFindings_DuplicateMessageIsBoundedInGroupSize pins that a
// brief-rests-on-duplicate finding names ONE other brief and counts the rest,
// so each message is bounded by two paths whatever the group size. Naming
// every other member made a group of k briefs O(k²) bytes of findings: at
// k = 2,000 (reachable under raised caps) about 88 MB.
func TestFindings_DuplicateMessageIsBoundedInGroupSize(t *testing.T) {
	cfg := testConfig(t, t.TempDir(), "")
	const k = 2000
	files := make([]File, 0, k)
	for i := 0; i < k; i++ {
		f := md("---\nsummary: s\nrests_on: [widget.contract.a]\n---\n")
		f.Rel = fmt.Sprintf("f%04d/b.md", i)
		files = append(files, f)
	}
	s := FromFiles(cfg, files)
	var dup []lint.Finding
	for _, f := range s.Findings([]model.Claim{{ID: "widget.contract.a"}}) {
		if f.LintName == RuleRestsOnDuplicate {
			dup = append(dup, f)
		}
	}
	if len(dup) != k {
		t.Fatalf("every member of the group is a finding: got %d, want %d", len(dup), k)
	}
	const maxMessage = 256
	total, longest := 0, 0
	for _, f := range dup {
		total += len(f.Message)
		longest = max(longest, len(f.Message))
		if len(f.Message) > maxMessage {
			t.Fatalf("a duplicate message must be bounded (≤%d bytes) whatever the group size; got %d bytes", maxMessage, len(f.Message))
		}
	}
	first, second := dup[0], dup[1]
	if !strings.Contains(first.Message, "briefs/f0001/b.md and 1998 other brief(s)") ||
		!strings.Contains(second.Message, "briefs/f0000/b.md and 1998 other brief(s)") {
		t.Fatalf("a finding names one other member and counts the rest, never itself:\n%s\n%s", first.Message, second.Message)
	}
	t.Logf("k=%d duplicate findings: %d message bytes in total, longest %d (bound %d)", k, total, longest, maxMessage)
}

// TestFindings_RestsOnRules pins the two claim-aware rules: an id no claim
// carries is an error, and two briefs with the same non-empty rests_on set are
// a warning on each — but two briefs resting on nothing are not duplicates.
func TestFindings_RestsOnRules(t *testing.T) {
	cfg := testConfig(t, t.TempDir(), "")
	withRests := func(ids string) File {
		return md("---\nsummary: s\nrests_on: [" + ids + "]\n---\n")
	}
	s := FromFiles(cfg, tree(map[string]File{
		"a/one.md":   withRests("widget.contract.a, widget.contract.b"),
		"a/two.md":   withRests("widget.contract.b, widget.contract.a"),
		"a/three.md": withRests("widget.contract.ghost"),
		"a/four.md":  md(okFront),
		"a/five.md":  md(okFront),
	}))
	claims := []model.Claim{{ID: "widget.contract.a"}, {ID: "widget.contract.b"}}
	var got []string
	for _, f := range s.Findings(claims) {
		got = append(got, string(f.Severity)+" "+f.LintName+" "+f.ClaimID)
	}
	want := []string{
		"error brief-rests-on-unknown briefs/a/three.md",
		"warning brief-rests-on-duplicate briefs/a/one.md",
		"warning brief-rests-on-duplicate briefs/a/two.md",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findings = %v, want %v", got, want)
	}
}
