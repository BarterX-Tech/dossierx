package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// mdCorpusDir resolves testdata/markdown-cases relative to THIS file, the way
// internal/render/markdown/markdown_test.go's markdownCasesDir does, so the
// test works whatever directory "go test" was invoked from. The corpus is a
// self-authored set of claim fixtures inside this module; nothing here reaches
// outside the repository, and nothing here WRITES to it.
func mdCorpusDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	dir := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "markdown-cases")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("markdown-cases dir not found at %q: %v", dir, err)
	}
	return dir
}

// mdCorpusClaims loads every .yaml fixture in the corpus, in filename order.
//
// It decodes straight into model.Claim rather than into a local struct because
// the lint under test takes model.Claim: a fixture whose rows or steps stopped
// decoding would then be silently scanned as an empty claim, and the corpus
// differential would go quiet for the wrong reason.
func mdCorpusClaims(t *testing.T) []model.Claim {
	t.Helper()
	dir := mdCorpusDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read markdown-cases dir: %v", err)
	}
	var claims []model.Claim
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		var c model.Claim
		if err := yaml.Unmarshal(data, &c); err != nil {
			t.Fatalf("unmarshal %s: %v", e.Name(), err)
		}
		claims = append(claims, c)
	}
	if len(claims) == 0 {
		t.Fatal("no YAML fixtures found in markdown-cases")
	}
	return claims
}

// TestMarkdownSanityCorpus enumerates what markdown-sanity says about the whole
// tracked corpus, one sorted, TAB-SEPARATED "claimID<TAB>rule<TAB>message" line
// per finding on stdout and nothing else.
//
// It exists to make a change to the scanner DIFFABLE rather than eyeballed: run
// it on the release base and on the branch, filter both down to the lines this
// prints, and diff. A change that fixes false positives may only ever DELETE
// lines, and only unmatched-delimiter ones; an added line, or a deleted line
// from any other finding kind, is a regression this test's output names
// exactly.
//
// It asserts nothing itself, deliberately. Pinning the corpus's finding set as
// a golden would make every legitimate scanner change a golden update, which is
// the review step this differential replaces.
func TestMarkdownSanityCorpus(t *testing.T) {
	claims := mdCorpusClaims(t)

	var lines []string
	for _, f := range (MarkdownSanity{}).Check(claims, nil) {
		// A finding message is single-line by construction; the replacements
		// are here so that one day it isn't, the line-oriented diff this test
		// exists for does not silently become a multi-line one.
		msg := strings.NewReplacer("\n", " ", "\t", " ").Replace(f.Message)
		lines = append(lines, fmt.Sprintf("%s\t%s\t%s", f.ClaimID, f.LintName, msg))
	}
	sort.Strings(lines)
	for _, l := range lines {
		fmt.Println(l)
	}
}
