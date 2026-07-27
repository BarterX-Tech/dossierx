package implink

import (
	"os"
	"testing"
	"time"

	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// THE LOST UPDATE. Scan used to call Set once per tag, and Set is a bare
// load-mutate-write: it took no sentinel at all, while "dossierx claim link"
// took one over the very same file. So the two writers were not serialized in
// any direction, and a `dossierx claim link` that landed inside a running
// `dossierx check` was simply overwritten — reproduced at five of ten runs on a
// project with 6000 tags, with claim link reporting ok:true and the link absent
// from the artifact afterwards. That is unrecoverable: claim link exists for the
// links a scan cannot derive, so re-scanning never restores them.
//
// The deterministic form of the same thing is this test: with the artifact's
// sentinel held, a scan must NOT write. It used to write regardless — the
// sentinel provided no mutual exclusion whatsoever.
func TestScan_WaitsForTheArtifactSentinelInsteadOfWritingThroughIt(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}
	writeScanFile(t, srcDir, "impl.go", "// dossierx-claim: widget.contract.main\nfunc Widget() {}\n")

	path := ArtifactPath(cfg, "widget")
	release, err := lock.AcquireFileLock(path)
	if err != nil {
		t.Fatalf("hold the artifact sentinel: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, scanErr := Scan(claims, cfg)
		done <- scanErr
	}()

	// While the sentinel is held, the scan must not have written anything. The
	// old code wrote here — it never consulted the lock at all — which is
	// exactly how a concurrent claim link's write was destroyed.
	select {
	case err := <-done:
		t.Fatalf("Scan completed while the sentinel was held (err=%v); it must wait for the holder", err)
	case <-time.After(250 * time.Millisecond):
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no artifact written while the sentinel was held (%v)", err)
	}

	// Released: the scan proceeds and writes exactly as it always did.
	release()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Scan after release: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("Scan did not complete after the sentinel was released")
	}
	if _, err := LoadArtifact(path); err != nil {
		t.Fatalf("expected the artifact written once the sentinel was free: %v", err)
	}
}

// The other half: with the sentinel free, one scan writes each module's artifact
// ONCE, holding the lock across the whole batch. The behaviour a caller sees is
// unchanged — every tag is still reconciled — and the file is no longer rewritten
// once per tag.
func TestScan_BatchesEveryTagIntoOneArtifactWrite(t *testing.T) {
	cfg, srcDir := scanTestConfig(t, "widget")
	claims := []model.Claim{
		lockedClaim("widget.contract.one", "widget", model.BuildRoleBehavior),
		lockedClaim("widget.contract.two", "widget", model.BuildRoleBehavior),
	}
	writeScanFile(t, srcDir, "a.go", "// dossierx-claim: widget.contract.one\nfunc One() {}\n")
	writeScanFile(t, srcDir, "b.go", "// dossierx-claim: widget.contract.two\nfunc Two() {}\n")

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Matches) != 2 || len(report.Errors) != 0 {
		t.Fatalf("expected both tags reconciled, got %d match(es) and %v", len(report.Matches), report.Errors)
	}

	artifact, err := LoadArtifact(ArtifactPath(cfg, "widget"))
	if err != nil {
		t.Fatalf("LoadArtifact: %v", err)
	}
	if len(artifact.Links) != 2 {
		t.Fatalf("expected both claims linked in one artifact, got %+v", artifact.Links)
	}

	// The sentinel is released when the batch is done, so a following writer —
	// "dossierx claim link", in production — can take it immediately.
	release, err := lock.AcquireFileLock(ArtifactPath(cfg, "widget"))
	if err != nil {
		t.Fatalf("the scan must release the sentinel it took: %v", err)
	}
	release()

	// And nothing was left behind beside the artifact.
	if _, err := os.Stat(ArtifactPath(cfg, "widget") + ".lock"); !os.IsNotExist(err) {
		t.Fatalf("expected no stale lock file (%v)", err)
	}
}
