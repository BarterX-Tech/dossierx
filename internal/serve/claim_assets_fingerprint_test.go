package serve

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/loader"
)

// claim_assets_fingerprint_test.go pins the invariant the image allowlist's
// freshness rests on, at the level it actually lives at rather than through the
// HTTP surface:
//
//	scanLoadedClaimFingerprint stamps EXACTLY the files loader.LoadClaims reads.
//
// The allowlist is invalidated by comparing this fingerprint. If it enumerates
// fewer files than the loader does, a claim it does not look at can be loaded,
// rendered and indexed and then edited or deleted with no fingerprint delta —
// so its asset entry outlives the claim indefinitely, and only an unrelated
// non-excluded claim changing can ever clear it. The property is a set equality,
// so this test asserts a set equality.
//
// The second half asserts the watcher's scanFingerprint DOES still exclude what
// it means to. That is not a contradiction: the two scans answer different
// questions, and the reason for keeping both is only legible if both behaviours
// are pinned.

// fingerprintTree fixture: one claim of every shape the two scans disagree
// about, plus files neither should ever stamp.
func writeFingerprintFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	claim := func(id string) string {
		return "id: " + id + "\nfacet: contract\nmodule: widget\nstatus: draft\nlayout: card\n" +
			"body: |\n  a claim.\ngoverned_by:\n  type: none\n  reason: fixture\n"
	}
	files := map[string]string{
		// Ordinary claims, both extensions, nested.
		"a/one.yaml":              claim("widget.contract.one"),
		"a/nested/deep.yml":       claim("widget.contract.deep"),
		".archive/hidden.yaml":    claim("widget.contract.hidden"),
		"g/retry.tmp-policy.yaml": claim("widget.contract.tmpname"),
		// Not claims to anybody: the atomic writer's scratch sibling (whose
		// extension is ".tmp-<rand>", not ".yaml") and an unrelated file.
		"g/one.yaml.tmp-abc123": claim("widget.contract.scratch"),
		"g/notes.txt":           "not a claim",
	}
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return root
}

func TestScanLoadedClaimFingerprint_StampsExactlyWhatTheLoaderReads(t *testing.T) {
	root := writeFingerprintFixture(t)

	claims, err := loader.LoadClaims(root)
	if err != nil {
		t.Fatalf("LoadClaims: %v", err)
	}
	loaded := make([]string, 0, len(claims))
	for _, c := range claims {
		loaded = append(loaded, c.SourcePath)
	}
	sort.Strings(loaded)

	fp, err := scanLoadedClaimFingerprint(root)
	if err != nil {
		t.Fatalf("scanLoadedClaimFingerprint: %v", err)
	}
	stamped := make([]string, 0, len(fp))
	for path := range fp {
		stamped = append(stamped, path)
	}
	sort.Strings(stamped)

	if strings.Join(loaded, "\n") != strings.Join(stamped, "\n") {
		t.Errorf("the fingerprint and the loader disagree about which files are claims\n  loader:      %v\n  fingerprint: %v",
			loaded, stamped)
	}
	// Guard against the fixture silently degenerating: the whole point is that
	// it contains the two shapes the watcher's scan omits.
	if len(loaded) != 4 {
		t.Fatalf("fixture: loader read %d claims, want 4 (%v)", len(loaded), loaded)
	}
}

// TestScanFingerprint_StillExcludesWhatLiveReloadMustIgnore pins the OTHER
// scan's narrower rule, so the pair is legible: it must keep skipping
// dot-directories and ".tmp-" names, because live reload flapping on every
// SaveClaim's scratch file is what that exclusion is for.
func TestScanFingerprint_StillExcludesWhatLiveReloadMustIgnore(t *testing.T) {
	root := writeFingerprintFixture(t)

	fp, err := scanFingerprint(root)
	if err != nil {
		t.Fatalf("scanFingerprint: %v", err)
	}
	for _, rel := range []string{
		".archive/hidden.yaml",
		"g/retry.tmp-policy.yaml",
		"g/one.yaml.tmp-abc123",
		"g/notes.txt",
	} {
		if _, ok := fp[filepath.Join(root, filepath.FromSlash(rel))]; ok {
			t.Errorf("the watcher's fingerprint stamped %s; it must not", rel)
		}
	}
	for _, rel := range []string{"a/one.yaml", "a/nested/deep.yml"} {
		if _, ok := fp[filepath.Join(root, filepath.FromSlash(rel))]; !ok {
			t.Errorf("the watcher's fingerprint missed %s", rel)
		}
	}
}
