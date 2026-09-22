package implink

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func initPinnedRepo(t *testing.T, dir string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "fixture@example.invalid")
	runGit(t, dir, "config", "user.name", "fixture")
	runGit(t, dir, "add", "-A")
	runGit(t, dir, "commit", "-qm", "fixture")
	return strings.TrimSpace(runGit(t, dir, "rev-parse", "HEAD"))
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func siblingScanConfig(t *testing.T, tagged string) (cfg *config.Config, sha string) {
	t.Helper()
	parent := t.TempDir()
	app := filepath.Join(parent, "app")
	corpus := filepath.Join(parent, "corpus")
	if err := os.MkdirAll(filepath.Join(corpus, "claims"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(app, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "src", "main.py"), []byte(tagged), 0o644); err != nil {
		t.Fatal(err)
	}
	sha = initPinnedRepo(t, app)
	cfgYAML := "schema_version: 1\nfacets:\n  - contract\nmodules:\n  - widget\nclaims_dir: claims\n" +
		"source_roots:\n  - path: ../app\n    repo: example/app\n    ref: " + sha + "\n"
	cfgPath := filepath.Join(corpus, "project.config.yaml")
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := config.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return loaded, sha
}

func TestScan_SourceRootsSibling_ReconcilesAndRecordsCommit(t *testing.T) {
	cfg, sha := siblingScanConfig(t, "# dossierx-claim: widget.contract.main\ndef do_thing():\n    pass\n")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}

	report, err := Scan(claims, cfg)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Errors) != 0 {
		t.Fatalf("scan errors: %+v", report.Errors)
	}
	if len(report.Matches) != 1 {
		t.Fatalf("matches = %+v", report.Matches)
	}
	if report.Matches[0].File != "../app/src/main.py" {
		t.Errorf("file = %q", report.Matches[0].File)
	}

	artifact, err := LoadArtifact(ArtifactPath(cfg, "widget"))
	if err != nil {
		t.Fatalf("LoadArtifact: %v", err)
	}
	if len(artifact.SourceRoots) != 1 {
		t.Fatalf("artifact source_roots = %+v", artifact.SourceRoots)
	}
	if artifact.SourceRoots[0].Commit != sha || artifact.SourceRoots[0].Repo != "example/app" {
		t.Fatalf("recorded root = %+v", artifact.SourceRoots[0])
	}
	if len(artifact.Links) != 1 || len(artifact.Links[0].Files) != 1 {
		t.Fatalf("links = %+v", artifact.Links)
	}
	f := artifact.Links[0].Files[0]
	if f.File != "../app/src/main.py" || f.Repo != "example/app" || f.Commit != sha {
		t.Fatalf("file link = %+v", f)
	}

	raw, err := os.ReadFile(ArtifactPath(cfg, "widget"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded Artifact
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.SourceRoots[0].Ref != sha {
		t.Errorf("json ref = %q", decoded.SourceRoots[0].Ref)
	}

	st, err := Status(claims, cfg, "widget")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if len(st.Drifted) != 0 {
		t.Fatalf("unexpected drift: %+v", st.Drifted)
	}
}

func TestSet_SourceRootsSibling_AllowsDeclaredEscape(t *testing.T) {
	cfg, sha := siblingScanConfig(t, "print('no tag')\n")
	claims := []model.Claim{lockedClaim("widget.contract.main", "widget", model.BuildRoleBehavior)}
	artifact, err := Set(claims, cfg, "widget", "widget.contract.main", "../app/src/main.py", "do_thing")
	if err != nil {
		t.Fatalf("Set: %v", err)
	}
	if artifact.Links[0].Files[0].Commit != sha {
		t.Fatalf("commit = %q", artifact.Links[0].Files[0].Commit)
	}
}

func TestScan_SourceRootsOnly_GatesLikeSourceDirs(t *testing.T) {
	cfg, _ := siblingScanConfig(t, "# dossierx-claim: widget.contract.main\ndef do_thing():\n    pass\n")
	if !cfg.ScansSource() || len(cfg.SourceDirs) != 0 {
		t.Fatalf("expected source_roots-only opt-in, dirs=%v roots=%d", cfg.SourceDirs, len(cfg.SourceRoots))
	}
}
