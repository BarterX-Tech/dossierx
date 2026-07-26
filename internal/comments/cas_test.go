package comments

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// A mutating comment op runs load->mutate->save under the claims sentinel, but
// an out-of-band writer (a text editor, a future sentinel-less writer) can still
// change the claim file inside that window. The op must capture the file token
// after load and write via SaveClaimIfUnchanged, so such an edit yields a
// matchable loader.ErrClaimFileChanged (the wired 409 claim_file_changed path)
// rather than being silently clobbered.

// withInterlude installs a mutate-internal seam that fires once, inside the
// claims sentinel between token capture and the guarded save.
func withInterlude(t *testing.T, fn func()) {
	t.Helper()
	orig := mutateInterlude
	done := false
	mutateInterlude = func() {
		if done {
			return
		}
		done = true
		fn()
	}
	t.Cleanup(func() { mutateInterlude = orig })
}

func TestMutate_OutOfBandEdit_YieldsClaimFileChanged(t *testing.T) {
	p := newProject(t, map[string]string{"a.yaml": draftAYAML})
	claimPath := filepath.Join(p.claimsDir, "a.yaml")

	// While our op holds the sentinel — after it captured the file token, before
	// it saves — a different writer rewrites the claim file out of band.
	outOfBand := []byte("id: widget.contract.a\nfacet: contract\nmodule: widget\nstatus: draft\nbody: someone else got here first\ngoverned_by:\n  type: none\n  reason: fixture\n")
	withInterlude(t, func() {
		if err := os.WriteFile(claimPath, outOfBand, 0o644); err != nil {
			t.Fatalf("out-of-band write: %v", err)
		}
	})

	_, _, err := p.deps().Add(claimA, model.CommentRoleHuman, "our concurrent comment")
	if !errors.Is(err, loader.ErrClaimFileChanged) {
		t.Fatalf("Add across an out-of-band edit: want loader.ErrClaimFileChanged, got %v", err)
	}

	// The refused save must not have clobbered the out-of-band content.
	got, err := os.ReadFile(claimPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(outOfBand) {
		t.Fatalf("optimistic save clobbered the out-of-band edit:\n%s", got)
	}
}
