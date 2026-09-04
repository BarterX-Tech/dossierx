package serve_test

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gsGit runs one git command in dir with an isolated configuration. A missing
// git binary is a FAILURE here, not a skip: the guard under test is the one
// that needs git to answer, and a machine that cannot run it has not checked
// anything.
func gsGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("git is not on PATH; the gitignore guard cannot be exercised: %v", err)
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func statusRules(t *testing.T, base string) (ok bool, rules []string) {
	t.Helper()
	resp, data := do(t, http.MethodGet, base+"/api/status", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/status: got %d, want 200 (body=%s)", resp.StatusCode, data)
	}
	var dto ledgerStatusDTO
	if err := json.Unmarshal(data, &dto); err != nil {
		t.Fatalf("decode status: %v (body=%s)", err, data)
	}
	for _, f := range dto.LedgerFindings {
		rules = append(rules, f.Rule)
	}
	return dto.OK, rules
}

// The gitignore verdict is NEVER cached for the life of the server. The claims
// watcher polls claims_dir only, and every .gitignore that matters — the
// repository top level's, the project root's — sits outside it by
// construction, so a cached verdict would stay clean for the rest of the
// session while serve's own comment and flag writes landed in an ignored
// directory. Writing build/ into .gitignore while the server is up must show
// up on the very next /api/status.
func TestStatus_ReportsStoreGitignoredWrittenWhileServing(t *testing.T) {
	_, base, root := startServer(t, baseConfig, map[string]string{
		"claims/one.yaml": draftClaim("widget.contract.one"),
	})
	gsGit(t, root, "init", "-q", "-b", "main")

	if ok, rules := statusRules(t, base); !ok || len(rules) != 0 {
		t.Fatalf("precondition: a fresh repository with no .gitignore must be clean, got ok=%v rules=%v", ok, rules)
	}

	writeFile(t, filepath.Join(root, ".gitignore"), "build/\n")

	ok, rules := statusRules(t, base)
	found := false
	for _, r := range rules {
		if r == "store-gitignored" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the next /api/status after .gitignore gained build/ must carry store-gitignored, got %v", rules)
	}
	if ok {
		t.Fatalf("ok:true beside a store-gitignored finding tells the strip to render a green project while the ledger cannot reach the repository")
	}

	// And back again: removing the pattern clears it on the next poll, so
	// the verdict is read each time and not latched either way.
	if err := os.Remove(filepath.Join(root, ".gitignore")); err != nil {
		t.Fatalf("remove .gitignore: %v", err)
	}
	if ok, rules := statusRules(t, base); !ok || len(rules) != 0 {
		t.Fatalf("after the pattern is gone the next poll must be clean, got ok=%v rules=%v", ok, rules)
	}
}
