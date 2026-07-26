// comment_races_test.go covers the comment-specific cross-process races that
// Phase 0 deferred until "dossierx comment add" existed. Each spawns two real
// binary invocations concurrently on the SAME claim; the project-wide claims
// sentinel must serialize their whole-file load->mutate->SaveClaim so the
// outcome is always ONE of the two serialized orders — never torn state, never
// a lost comment, never a lock-gate bypass. (-race cannot see a cross-PROCESS
// race, so these assert on the resulting file/CLI state, per the plan.)
package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/comments"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// countThreads returns the number of comment threads "dossierx comment list"
// reports for id — which also proves the claim file still parses (a torn write
// would fail the load or the envelope decode).
//
// The command's pre-v0.3.0 "--json" flag emitted a bare array; the machine
// surface is the standard envelope now, so this reads data.count.
func countThreads(t *testing.T, root, cfgPath, id string) int {
	t.Helper()
	out, stderr, code := run(t, root, "--config", cfgPath, "--format", "json", "comment", "list", id)
	if code != 0 {
		t.Fatalf("comment list for %s exited %d (stderr: %s)", id, code, stderr)
	}
	var env struct {
		Data struct {
			Count   int              `json:"count"`
			Threads []map[string]any `json:"threads"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("comment list for %s is not a single envelope: %v\nout: %s", id, err, out)
	}
	if env.Data.Count != len(env.Data.Threads) {
		t.Fatalf("comment list count/threads disagree for %s: %d vs %d", id, env.Data.Count, len(env.Data.Threads))
	}
	return env.Data.Count
}

// runPair runs two binary invocations concurrently in root and waits for both.
// run() only t.Fatal's if the binary can't be exec'd at all (never on a
// non-zero exit), so calling it from goroutines is safe here.
func runPair(t *testing.T, root string, argsA, argsB []string) {
	t.Helper()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); run(t, root, argsA...) }()
	go func() { defer wg.Done(); run(t, root, argsB...) }()
	wg.Wait()
}

const raceIterations = 6

// comment add vs lock on the same claim: whichever order wins, the added
// comment is never lost, and the lock gate is never bypassed — a claim that
// ends up locked WITH an open thread must be review_pending (that can only be
// the lock-then-add order), never locked-with-an-open-thread-and-no-flag.
func TestCommentRace_AddVsLock(t *testing.T) {
	for i := 0; i < raceIterations; i++ {
		root := t.TempDir()
		cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
		claimPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "raced claim."})

		runPair(t, root,
			[]string{"--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "human", "--body", "race body"},
			[]string{"--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "test fixture"},
		)

		if n := countThreads(t, root, cfgPath, "widget.contract.main"); n != 1 {
			t.Fatalf("iter %d: expected the added comment to survive (exactly 1 thread), got %d", i, n)
		}
		final := llReadFile(t, claimPath)
		if strings.Contains(final, "status: locked") && !strings.Contains(final, "review_pending: true") {
			t.Fatalf("iter %d: lock-gate bypass — claim locked with an open thread but NOT review_pending:\n%s", i, final)
		}
	}
}

// comment add vs unlock: both orders end draft with the comment intact (unlock
// preserves Comments; add on a draft never sets review_pending).
func TestCommentRace_AddVsUnlock(t *testing.T) {
	for i := 0; i < raceIterations; i++ {
		root := t.TempDir()
		cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
		claimPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "raced claim."})
		if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "test fixture"); code != 0 {
			t.Fatalf("iter %d: lock setup: %s", i, stderr)
		}

		runPair(t, root,
			[]string{"--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "human", "--body", "race body"},
			[]string{"--config", cfgPath, "claim", "unlock", "widget.contract.main", "--reason", "test fixture"},
		)

		if n := countThreads(t, root, cfgPath, "widget.contract.main"); n != 1 {
			t.Fatalf("iter %d: expected the comment to survive add-vs-unlock (exactly 1 thread), got %d", i, n)
		}
		if final := llReadFile(t, claimPath); !strings.Contains(final, "status: draft") {
			t.Fatalf("iter %d: expected the claim draft after add-vs-unlock (both orders end draft), got:\n%s", i, final)
		}
	}
}

// comment add vs flag: both are review_pending triggers on a locked claim; both
// orders end locked + review_pending with the comment intact and the flag
// recorded.
func TestCommentRace_AddVsFlag(t *testing.T) {
	for i := 0; i < raceIterations; i++ {
		root := t.TempDir()
		cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
		claimPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "raced claim."})
		if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "test fixture"); code != 0 {
			t.Fatalf("iter %d: lock setup: %s", i, stderr)
		}

		runPair(t, root,
			[]string{"--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "human", "--body", "race body"},
			[]string{"--config", cfgPath, "claim", "flag", "widget.contract.main", "--claim-says", "x", "--now-does", "y", "--reason", "raced"},
		)

		if n := countThreads(t, root, cfgPath, "widget.contract.main"); n != 1 {
			t.Fatalf("iter %d: expected the comment to survive add-vs-flag (exactly 1 thread), got %d", i, n)
		}
		final := llReadFile(t, claimPath)
		if !strings.Contains(final, "status: locked") || !strings.Contains(final, "review_pending: true") {
			t.Fatalf("iter %d: expected locked + review_pending after add-vs-flag, got:\n%s", i, final)
		}
	}
}

// firstThreadID returns the id of the first thread "dossierx comment list"
// reports for id (the fixture always has exactly one).
func firstThreadID(t *testing.T, root, cfgPath, id string) string {
	t.Helper()
	out, stderr, code := run(t, root, "--config", cfgPath, "--format", "json", "comment", "list", id)
	if code != 0 {
		t.Fatalf("comment list for %s exited %d (stderr: %s)", id, code, stderr)
	}
	// The envelope's threads are model.Comment values, which carry no json
	// tags, so each thread's fields arrive under their Go names ("ID").
	var env struct {
		Data struct {
			Threads []map[string]any `json:"threads"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("comment list for %s is not a single envelope: %v\nout: %s", id, err, out)
	}
	if len(env.Data.Threads) == 0 {
		t.Fatalf("expected at least one thread on %s, got none", id)
	}
	tid, ok := env.Data.Threads[0]["ID"].(string)
	if !ok || tid == "" {
		t.Fatalf("first thread on %s has no id: %v", id, env.Data.Threads[0])
	}
	return tid
}

// resolve vs flag on the same locked claim, the flag-orphan race: both are
// review_pending inputs, but resolve CLEARS review_pending (its trigger, the
// last open thread, is gone) while flag SETS it. Whichever serialized order
// wins, the claim must end review_pending:true with the flag recorded — never
// review_pending:false with an orphaned flag. That holds only because resolve
// re-reads the flag store FRESH inside the claims sentinel; a snapshot taken
// before the sentinel would miss a flag committed first and clear it.
//
// The resolve half is driven through internal/comments directly rather than
// through the CLI, because v0.3.0 removed "comment resolve" from the CLI (it
// lives in the viewer, where the rights holder is — see cmd/dossierx/comment.go).
// That is NOT a weaker test: the sentinel it exercises is a real cross-process
// file lock, this test binary is a genuinely separate process from the "claim
// flag" subprocess it races, and comments.Deps.Resolve is the exact same code
// path serve's HTTP handler calls. What is lost is only the argv plumbing,
// which internal/serve's own tests cover.
func TestCommentRace_ResolveVsFlag(t *testing.T) {
	for i := 0; i < raceIterations; i++ {
		root := t.TempDir()
		cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
		claimPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "raced claim."})
		if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", "widget.contract.main", "--reason", "test fixture"); code != 0 {
			t.Fatalf("iter %d: lock setup: %s", i, stderr)
		}
		// Open a thread on the locked claim (this alone sets review_pending).
		if _, stderr, code := run(t, root, "--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "human", "--body", "please look"); code != 0 {
			t.Fatalf("iter %d: comment add setup: %s", i, stderr)
		}
		tid := firstThreadID(t, root, cfgPath, "widget.contract.main")

		cfg, err := config.LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("iter %d: load config: %v", i, err)
		}
		deps := &comments.Deps{
			Cfg:           cfg,
			LockStorePath: filepath.Join(cfg.Dir(), ".dossierx-lock-store.json"),
			FlagStorePath: filepath.Join(cfg.Dir(), ".dossierx-flag-store.json"),
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			// A refusal here is a legitimate serialized outcome in principle;
			// the assertions below are on the resulting STATE either way.
			deps.Resolve("widget.contract.main", tid, model.CommentRoleHuman) //nolint:errcheck // outcome asserted via on-disk state below
		}()
		go func() {
			defer wg.Done()
			run(t, root, "--config", cfgPath, "claim", "flag", "widget.contract.main",
				"--claim-says", "x", "--now-does", "y", "--reason", "raced")
		}()
		wg.Wait()

		// The flag stands (flag never gets deleted by resolve), so the claim MUST
		// still be review_pending — otherwise the flag is orphaned.
		final := llReadFile(t, claimPath)
		if !strings.Contains(final, "status: locked") || !strings.Contains(final, "review_pending: true") {
			t.Fatalf("iter %d: flag-orphan race — resolve cleared review_pending while a flag stands (expected locked + review_pending:true):\n%s", i, final)
		}
		flagStore := llReadFile(t, root+"/.dossierx-flag-store.json")
		if !strings.Contains(flagStore, "widget.contract.main") {
			t.Fatalf("iter %d: expected the flag recorded in the flag store, got:\n%s", i, flagStore)
		}
	}
}

// comment add vs a review_pending-flipping check: check reconciles main to
// review_pending (its dependency drifted) while a comment is added; the
// comments block must survive whichever order wins.
func TestCommentRace_AddVsCheck(t *testing.T) {
	for i := 0; i < raceIterations; i++ {
		root := t.TempDir()
		cfgPath := llWriteConfig(t, root, []string{"contract"}, []string{"widget"}, "")
		depPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.dep", facet: "contract", module: "widget", status: "draft", body: "dep, v1."})
		mainPath := llWriteClaim(t, root, llClaimSpec{id: "widget.contract.main", facet: "contract", module: "widget", status: "draft", body: "main on dep.", restsOn: []string{"widget.contract.dep"}})
		for _, id := range []string{"widget.contract.dep", "widget.contract.main"} {
			if _, stderr, code := run(t, root, "--config", cfgPath, "claim", "lock", id, "--reason", "test fixture"); code != 0 {
				t.Fatalf("iter %d: lock %s: %s", i, id, stderr)
			}
		}
		// Drift the dependency so a concurrent check WANTS to flip main.
		drifted := strings.Replace(llReadFile(t, depPath), "dep, v1.", "dep, v2.", 1)
		if err := os.WriteFile(depPath, []byte(drifted), 0o644); err != nil {
			t.Fatalf("iter %d: rewrite dep: %v", i, err)
		}

		runPair(t, root,
			[]string{"--config", cfgPath, "comment", "add", "widget.contract.main", "--as", "human", "--body", "race body"},
			[]string{"--config", cfgPath, "check"},
		)

		if n := countThreads(t, root, cfgPath, "widget.contract.main"); n != 1 {
			t.Fatalf("iter %d: expected the comment to survive add-vs-check (exactly 1 thread), got %d", i, n)
		}
		if final := llReadFile(t, mainPath); !strings.Contains(final, "status: locked") {
			t.Fatalf("iter %d: expected main still locked after add-vs-check, got:\n%s", i, final)
		}
	}
}
