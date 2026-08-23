package lint

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// writeSourceFile writes rel (a slash-separated path) under cfg's own
// directory and returns the lowercase hex sha256 of what it wrote — the value
// a correctly-recorded source would carry.
func writeSourceFile(t *testing.T, cfg *config.Config, rel, content string) string {
	t.Helper()
	full := filepath.Join(cfg.Dir(), filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}

func internalSource(path, sha string) model.Source {
	return model.Source{
		Ref:    1,
		Kind:   model.SourceKindInternal,
		Title:  "Extraction ledger",
		Path:   path,
		SHA256: sha,
	}
}

func runDrift(t *testing.T, cfg *config.Config, s model.Source) []Finding {
	t.Helper()
	return (sourceInternalDriftLint{}).Check(
		[]model.Claim{{ID: "widget.internals.fields", Sources: []model.Source{s}}}, cfg)
}

func TestSourceInternalDriftMatchingWholeFile(t *testing.T) {
	cfg := testConfig(t)
	sum := writeSourceFile(t, cfg, "sources/decision-log.md", "The ceiling was set to 100/min.\n")

	if f := runDrift(t, cfg, internalSource("sources/decision-log.md", sum)); len(f) != 0 {
		t.Fatalf("a file that still matches its hash must produce nothing, got %+v", f)
	}
}

func TestSourceInternalDriftUppercaseRecordedHashStillMatches(t *testing.T) {
	cfg := testConfig(t)
	sum := writeSourceFile(t, cfg, "sources/decision-log.md", "The ceiling was set to 100/min.\n")

	if f := runDrift(t, cfg, internalSource("sources/decision-log.md", strings.ToUpper(sum))); len(f) != 0 {
		t.Fatalf("hex case must not be mistaken for drift, got %+v", f)
	}
}

func TestSourceInternalDriftContentChanged(t *testing.T) {
	cfg := testConfig(t)
	stale := writeSourceFile(t, cfg, "sources/decision-log.md", "The ceiling was set to 100/min.\n")
	writeSourceFile(t, cfg, "sources/decision-log.md", "The ceiling was raised to 500/min.\n")

	findings := runDrift(t, cfg, internalSource("sources/decision-log.md", stale))
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "no longer matches") {
		t.Errorf("message does not describe drift: %s", findings[0].Message)
	}
	if findings[0].Severity != SeverityError {
		t.Errorf("Severity = %q, want error", findings[0].Severity)
	}
}

// TestSourceInternalDriftMissingHashIsAFinding is the rule this file exists
// to protect: a check that cannot execute is reported, never waved through.
func TestSourceInternalDriftMissingHashIsAFinding(t *testing.T) {
	cfg := testConfig(t)
	writeSourceFile(t, cfg, "sources/decision-log.md", "The ceiling was set to 100/min.\n")

	findings := runDrift(t, cfg, internalSource("sources/decision-log.md", ""))
	if len(findings) != 1 {
		t.Fatalf("an unhashed internal source must report exactly once, got %+v", findings)
	}
	if !strings.Contains(findings[0].Message, "records no sha256") {
		t.Errorf("message does not name the missing hash: %s", findings[0].Message)
	}
}

func TestSourceInternalDriftUnreadableFileIsAFinding(t *testing.T) {
	cfg := testConfig(t)
	findings := runDrift(t, cfg, internalSource("sources/never-written.md",
		"0000000000000000000000000000000000000000000000000000000000000000"))
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "cannot read path") {
		t.Errorf("message does not name the unreadable file: %s", findings[0].Message)
	}
}

// TestSourceInternalDriftMissingHashAndMissingFile reports both halves in one
// run, so an author pasting in a hash learns the file is absent immediately
// rather than on the next pass.
func TestSourceInternalDriftMissingHashAndMissingFile(t *testing.T) {
	cfg := testConfig(t)
	findings := runDrift(t, cfg, internalSource("sources/never-written.md", ""))
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(findings), findings)
	}
}

// TestSourceInternalDriftNoProjectDirectory pins the in-memory-corpus case:
// with nothing to resolve against, the hash was not checked, and that is a
// finding rather than a pass.
func TestSourceInternalDriftNoProjectDirectory(t *testing.T) {
	findings := runDrift(t, nil, internalSource("sources/decision-log.md",
		"0000000000000000000000000000000000000000000000000000000000000000"))
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "cannot resolve path") {
		t.Errorf("message does not name the unresolvable path: %s", findings[0].Message)
	}
}

// TestSourceInternalDriftMissingPathDefersToSourceShape: source-shape already
// says what to do about an internal source with no path, and saying it twice
// would send the author after two defects.
func TestSourceInternalDriftMissingPathDefersToSourceShape(t *testing.T) {
	cfg := testConfig(t)
	findings := runDrift(t, cfg, internalSource("",
		"0000000000000000000000000000000000000000000000000000000000000000"))
	if len(findings) != 0 {
		t.Fatalf("expected no findings (source-shape owns the missing path), got %+v", findings)
	}
}

func TestSourceInternalDriftExternalSourceIsIgnored(t *testing.T) {
	cfg := testConfig(t)
	findings := runDrift(t, cfg, wellFormedExternal())
	if len(findings) != 0 {
		t.Fatalf("expected no findings for an external source, got %+v", findings)
	}
}

const driftLedger = `{"id":"REQ-17","text":"rate limit is 100/min"}
{"id":"REQ-18","text":"burst ceiling is 500"}
{"id":"REQ-19","text":"retries are capped at 3"}
`

// jsonlLineHash is the hash a correctly recorded record_id source carries: the
// raw line as authored, with no line terminator.
func jsonlLineHash(t *testing.T, line string) string {
	t.Helper()
	sum := sha256.Sum256([]byte(line))
	return hex.EncodeToString(sum[:])
}

func TestSourceInternalDriftRecordIDPinsOneLine(t *testing.T) {
	cfg := testConfig(t)
	writeSourceFile(t, cfg, "sources/ledger.jsonl", driftLedger)

	s := internalSource("sources/ledger.jsonl",
		jsonlLineHash(t, `{"id":"REQ-18","text":"burst ceiling is 500"}`))
	s.RecordID = "REQ-18"

	if f := runDrift(t, cfg, s); len(f) != 0 {
		t.Fatalf("the pinned record still matches, so nothing should fire: %+v", f)
	}
}

// TestSourceInternalDriftRecordIDIgnoresOtherRecords is the reason RecordID
// exists: a shared registry file changes constantly for reasons unrelated to
// any one claim, and a whole-file hash would report drift on every edit.
func TestSourceInternalDriftRecordIDIgnoresOtherRecords(t *testing.T) {
	cfg := testConfig(t)
	writeSourceFile(t, cfg, "sources/ledger.jsonl", driftLedger+
		`{"id":"REQ-20","text":"added later, nothing to do with this claim"}`+"\n")

	s := internalSource("sources/ledger.jsonl",
		jsonlLineHash(t, `{"id":"REQ-18","text":"burst ceiling is 500"}`))
	s.RecordID = "REQ-18"

	if f := runDrift(t, cfg, s); len(f) != 0 {
		t.Fatalf("an unrelated record's edit must not read as drift: %+v", f)
	}
}

func TestSourceInternalDriftRecordIDContentChanged(t *testing.T) {
	cfg := testConfig(t)
	writeSourceFile(t, cfg, "sources/ledger.jsonl",
		`{"id":"REQ-18","text":"burst ceiling is 900"}`+"\n")

	s := internalSource("sources/ledger.jsonl",
		jsonlLineHash(t, `{"id":"REQ-18","text":"burst ceiling is 500"}`))
	s.RecordID = "REQ-18"

	findings := runDrift(t, cfg, s)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, `record "REQ-18"`) {
		t.Errorf("message does not name the record that drifted: %s", findings[0].Message)
	}
}

func TestSourceInternalDriftRecordIDNotFound(t *testing.T) {
	cfg := testConfig(t)
	writeSourceFile(t, cfg, "sources/ledger.jsonl", driftLedger)

	s := internalSource("sources/ledger.jsonl",
		"0000000000000000000000000000000000000000000000000000000000000000")
	s.RecordID = "REQ-99"

	findings := runDrift(t, cfg, s)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "no line has a top-level") {
		t.Errorf("message does not name the missing record: %s", findings[0].Message)
	}
}

func TestSourceInternalDriftRecordIDNotUnique(t *testing.T) {
	cfg := testConfig(t)
	writeSourceFile(t, cfg, "sources/ledger.jsonl",
		`{"id":"REQ-18","text":"one"}`+"\n"+`{"id":"REQ-18","text":"two"}`+"\n")

	s := internalSource("sources/ledger.jsonl",
		"0000000000000000000000000000000000000000000000000000000000000000")
	s.RecordID = "REQ-18"

	findings := runDrift(t, cfg, s)
	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "2 lines share that id") {
		t.Errorf("message does not name the collision: %s", findings[0].Message)
	}
}

// TestSourceInternalDriftHashesTheRawLine: the recorded hash is taken over the
// bytes as authored, so a reformatting of the record IS drift. A canonicalized
// hash would forgive it silently.
func TestSourceInternalDriftHashesTheRawLine(t *testing.T) {
	cfg := testConfig(t)
	writeSourceFile(t, cfg, "sources/ledger.jsonl",
		`{ "id": "REQ-18", "text": "burst ceiling is 500" }`+"\n")

	s := internalSource("sources/ledger.jsonl",
		jsonlLineHash(t, `{"id":"REQ-18","text":"burst ceiling is 500"}`))
	s.RecordID = "REQ-18"

	if f := runDrift(t, cfg, s); len(f) != 1 {
		t.Fatalf("a reformatted record must report as drift, got %+v", f)
	}
}

func TestSourceInternalDriftEmptyInput(t *testing.T) {
	if f := (sourceInternalDriftLint{}).Check(nil, nil); len(f) != 0 {
		t.Fatalf("expected no findings over no claims, got %+v", f)
	}
}
