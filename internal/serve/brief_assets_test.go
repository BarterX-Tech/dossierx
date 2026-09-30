package serve_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// briefAssetProject is a served project with one brief referencing one image,
// an image nobody references beside it, and a file outside the briefs tree.
func briefAssetProject(t *testing.T) (base, root string) {
	t.Helper()
	_, base, root = startServer(t, baseConfig, map[string]string{
		"claims/one.yaml":         draftClaim("widget.contract.one"),
		"briefs/flow/overview.md": "---\nsummary: The flow.\n---\n# Flow\n\n![The flow](flow-diagram.png)\n",
	})
	writeBytes(t, filepath.Join(root, "briefs", "flow", "flow-diagram.png"), pngBytes)
	writeBytes(t, filepath.Join(root, "briefs", "flow", "unreferenced.png"), []byte("nobody points at me"))
	writeBytes(t, filepath.Join(root, "secret.png"), []byte("outside the briefs tree"))
	return base, root
}

// TestBriefAsset_ServesTheImageThePageReferences is the serve half of "each
// brief page renders its images" (NIT-197): the src the rendered page carries
// is answered with the file's bytes under the closed-set content type and the
// sandboxing headers the claim-image route sends.
func TestBriefAsset_ServesTheImageThePageReferences(t *testing.T) {
	base, _ := briefAssetProject(t)

	page, body := do(t, http.MethodGet, base+"/", "")
	if page.StatusCode != http.StatusOK {
		t.Fatalf("GET /: status %d", page.StatusCode)
	}
	const wantSrc = `src="brief-assets/flow/flow-diagram.png"`
	if !strings.Contains(string(body), wantSrc) {
		t.Fatalf("the rendered viewer does not carry %s", wantSrc)
	}

	resp, data := do(t, http.MethodGet, base+"/brief-assets/flow/flow-diagram.png", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET the image: status %d, body %q", resp.StatusCode, data)
	}
	if string(data) != string(pngBytes) {
		t.Errorf("served %q, want the fixture bytes", data)
	}
	for header, want := range map[string]string{
		"Content-Type":            "image/png",
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": "default-src 'none'; sandbox",
	} {
		if got := resp.Header.Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

// TestBriefAsset_OnlyWhatABriefReferences is the allowlist: an image in a
// brief folder that no brief references, a file outside the tree reached by
// traversal, a name outside the brief tree's own rule and a folder that does
// not exist are all a bare 404.
func TestBriefAsset_OnlyWhatABriefReferences(t *testing.T) {
	base, _ := briefAssetProject(t)
	for _, p := range []string{
		"/brief-assets/flow/unreferenced.png",
		"/brief-assets/flow/..%2F..%2Fsecret.png",
		"/brief-assets/..%2Fsecret.png/x.png",
		"/brief-assets/flow/Flow-Diagram.png",
		"/brief-assets/nosuch/flow-diagram.png",
		"/brief-assets/flow/overview.md",
	} {
		assertNotFound(t, base, p)
	}
}

// TestBriefAsset_ASymlinkedImageIsNotServed pins that a referenced image
// swapped for a link after discovery is refused at the route: the file would
// otherwise be a way to read anything the serve process can.
func TestBriefAsset_ASymlinkedImageIsNotServed(t *testing.T) {
	base, root := briefAssetProject(t)
	img := filepath.Join(root, "briefs", "flow", "flow-diagram.png")
	if err := os.Remove(img); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "secret.png"), img); err != nil {
		t.Skipf("symlinks unavailable here: %v", err)
	}
	assertNotFound(t, base, "/brief-assets/flow/flow-diagram.png")
}
