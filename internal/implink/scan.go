// scan.go implements the automatic, tag-driven half of claim-to-code
// linking: instead of an agent (or human) explicitly running "dossierx implink
// set" once per claim, Scan walks a project's declared cfg.SourceDirs
// looking for a "dossierx-claim: <id>" comment anywhere in a text file and, for
// every one it finds, calls the exact same Set logic every explicit link
// already goes through — same validation, same artifact, same drift
// detection. Nothing about Set or the on-disk artifact format changes for
// this: Scan is purely a second, automatic caller of it.
//
// Design intent (this is the point the whole feature exists for): a tag
// written in source, alone, is meant to be sufficient for the
// documentation to know about it. No separate command to remember to run
// per claim — Scan is meant to be invoked as part of "dossierx check", the one
// command a project runs routinely, so code and docs cannot silently drift
// apart from each other over the following six months or a year the way a
// manually-invoked-only linking step eventually would.
//
// A tag naming a claim that does not exist, or that exists but is not
// locked, is a hard error (ScanReport.Errors), not a silent skip — the
// same "no mess left lying around" requirement that shaped every other gate
// in this package. Scan reports its own findings; it is check's caller
// (cmd/dossierx) that decides what a non-empty Errors list means for the
// command's exit code, mirroring how lint findings vs. lint errors are
// already split between this engine's finding-severity and its callers.
package implink

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// tagPattern matches a "dossierx-claim: <id>" marker anywhere in a line,
// regardless of what comment syntax (//, #, --, /* */, ...) surrounds it —
// Scan never looks at comment syntax at all, only at this literal marker
// string, which is what keeps it working identically across any source
// language a project happens to use.
var tagPattern = regexp.MustCompile(`dossierx-claim:\s*([A-Za-z0-9_.\-]+)`)

// symbolPatterns is a small, deliberately shallow set of "the next line
// looks like it declares a named symbol" heuristics across a handful of
// common source shapes. This is NOT a parser for any of these languages —
// it exists only to give a scanned link a readable symbol label the way an
// explicit "implink set --symbol" call already can; ScanReport's entries
// remain correct (the file is still linked) even when none of these match
// and the captured symbol is left blank. An agent or human can always
// override it precisely with an explicit "implink set --symbol" call
// afterward, same as any other Set call.
var symbolPatterns = []*regexp.Regexp{
	regexp.MustCompile(`^\s*(?:async\s+)?def\s+(\w+)`),                                       // Python function/method
	regexp.MustCompile(`^\s*class\s+(\w+)`),                                                  // Python class
	regexp.MustCompile(`^\s*func\s*(?:\([^)]*\)\s*)?(\w+)`),                                  // Go func / method
	regexp.MustCompile(`^\s*(?:export\s+)?(?:async\s+)?function\s+(\w+)`),                    // JS/TS function
	regexp.MustCompile(`^\s*(?:export\s+)?const\s+(\w+)\s*=`),                                // JS/TS const-fn
	regexp.MustCompile(`^\s*(?:public|private|protected)?\s*(?:static\s+)?\w+\s+(\w+)\s*\(`), // Java/C#-shaped method
}

// maxScanFileSize skips any file larger than this rather than reading it in
// full — a generic text scan has no business trying to regex-search a
// multi-megabyte binary or data file that happened to land inside
// cfg.SourceDirs; a real source file is never this large in practice.
const maxScanFileSize = 5 << 20 // 5 MiB

// ScanMatch is one "dossierx-claim: <id>" tag Scan found, whether or not it
// resolved to a valid, linkable claim (see ScanReport.Errors for the
// invalid ones).
type ScanMatch struct {
	ClaimID string
	File    string // project-relative, slash-separated
	Line    int    // 1-based line number the tag itself was found on
	Symbol  string // best-effort; may be empty
}

// ScanError is one tag Scan found that could not be reconciled into a
// link — the claim id it names does not exist at all, or exists but is not
// yet locked. Both are reported, never silently skipped: an unbacked or
// premature tag is exactly the "mess" this feature exists to prevent.
type ScanError struct {
	File    string
	Line    int
	ClaimID string
	Message string
}

// ScanReport is Scan's full result: how much it looked at, what it found,
// what it successfully linked, and what it could not.
type ScanReport struct {
	FilesScanned int
	Matches      []ScanMatch // every valid tag Scan reconciled into a link
	Errors       []ScanError // every tag Scan found but could not reconcile
}

// Summary renders a one-line, human-readable overview of a scan run,
// matching the terse style of StatusReport.Summary and buildorder's own
// CLI-facing summary lines.
func (r *ScanReport) Summary() string {
	return fmt.Sprintf(
		"impl-links: scanned %d file(s), found %d tag(s), reconciled %d link(s) (%d error(s))",
		r.FilesScanned, len(r.Matches)+len(r.Errors), len(r.Matches), len(r.Errors),
	)
}

// Scan walks every directory in cfg.SourceDirs, finds every "dossierx-claim:
// <id>" tag in every text file under them, and — for each tag naming a
// claim that exists and is locked — calls Set with that claim's own Module
// (never a caller-supplied one: a scanned tag names only a claim id, so its
// module is looked up, not asserted by the caller the way explicit
// "implink set --module" requires). A tag naming an unknown or not-yet-
// locked claim is recorded in ScanReport.Errors instead of being silently
// dropped.
//
// Scan is a no-op (a zero-value, all-zero ScanReport, nil error) when
// cfg.SourceDirs is empty — the zero-cost-when-unused contract every
// optional feature in this engine follows; a project that has never set
// source_dirs sees no behavior change at all from this function existing.
//
// IT WALKS FIRST AND WRITES SECOND, one artifact write per module, holding that
// module's sentinel (lock.AcquireFileLock over its ArtifactPath) across the
// whole batch. Both halves of that matter, and the first one is the bug it
// fixes: Scan used to call Set once per tag, and Set is a bare
// load-mutate-write with no sentinel at all, so a `dossierx claim link` running
// concurrently with a `dossierx check` was a plain lost update. `claim link`
// took the sentinel and check ignored it; measured on a project with 6000 tags,
// five of ten runs reported claim link ok:true and left no trace of the link in
// the artifact. That is unrecoverable data — `claim link` exists precisely for
// the links scanning cannot reach, so a re-scan never restores them.
//
// Holding the sentinel means a scan can now FAIL where it used to write
// regardless (another process holds the lock, or the ten-second acquisition
// times out). That is the correct direction for a writer: the caller's run stops
// and says so, instead of silently discarding somebody else's write. The batch
// also removes the N-renames-per-scan the per-tag write cost.
func Scan(claims []model.Claim, cfg *config.Config) (*ScanReport, error) {
	report := &ScanReport{}
	if cfg == nil || len(cfg.SourceDirs) == 0 {
		return report, nil
	}

	// Every tag that named a real, locked claim, in walk order, keyed by the
	// module whose artifact it belongs in. Nothing is written during the walk.
	pending := map[string][]ScanMatch{}

	for _, root := range cfg.SourceDirs {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return err
			}
			if info.Size() > maxScanFileSize || info.Size() == 0 {
				return nil
			}

			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !isProbablyText(data) {
				return nil
			}
			report.FilesScanned++

			relFile, err := filepath.Rel(cfg.Dir(), path)
			if err != nil {
				return err
			}
			relFile = filepath.ToSlash(relFile)

			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				m := tagPattern.FindStringSubmatch(line)
				if m == nil {
					continue
				}
				claimID := m[1]
				lineNo := i + 1
				symbol := captureSymbol(lines, i)

				claim, ok := findByID(claims, claimID)
				if !ok {
					report.Errors = append(report.Errors, ScanError{
						File: relFile, Line: lineNo, ClaimID: claimID,
						Message: "no such claim (check for a typo)",
					})
					continue
				}
				if claim.Status != model.StatusLocked {
					report.Errors = append(report.Errors, ScanError{
						File: relFile, Line: lineNo, ClaimID: claimID,
						Message: fmt.Sprintf("claim is not locked (status %q)", claim.Status),
					})
					continue
				}

				pending[claim.Module] = append(pending[claim.Module], ScanMatch{
					ClaimID: claimID, File: relFile, Line: lineNo, Symbol: symbol,
				})
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("implink: scan %q: %w", root, err)
		}
	}

	if err := reconcile(report, pending, claims, cfg); err != nil {
		return nil, err
	}

	sort.Slice(report.Matches, func(i, j int) bool {
		if report.Matches[i].File != report.Matches[j].File {
			return report.Matches[i].File < report.Matches[j].File
		}
		return report.Matches[i].Line < report.Matches[j].Line
	})
	sort.Slice(report.Errors, func(i, j int) bool {
		if report.Errors[i].File != report.Errors[j].File {
			return report.Errors[i].File < report.Errors[j].File
		}
		return report.Errors[i].Line < report.Errors[j].Line
	})

	return report, nil
}

// reconcile applies every pending tag to its module's artifact and persists each
// module's artifact ONCE, under that module's own sentinel.
//
// The sentinel is the whole point (see Scan): it is the same lock
// "dossierx claim link" takes over the same path, so the two writers are finally
// serialized against each other instead of each doing an unguarded
// load-mutate-write. A failure to acquire it is returned to the caller rather
// than swallowed — a scan that cannot take the lock must not write, and a
// writer that cannot write must say so.
//
// A per-tag reconciliation failure is recorded in the report exactly as before
// and does not abandon the module: the remaining tags are still applied, and the
// artifact is still written, so one bad tag cannot cost a module every link it
// legitimately has.
func reconcile(report *ScanReport, pending map[string][]ScanMatch, claims []model.Claim, cfg *config.Config) error {
	// Map iteration is random and this function writes files; the order modules
	// are locked in is therefore fixed, which also keeps two concurrent scans
	// from taking two modules' sentinels in opposite orders.
	modules := make([]string, 0, len(pending))
	for module := range pending {
		modules = append(modules, module)
	}
	sort.Strings(modules)

	for _, module := range modules {
		if err := reconcileModule(report, module, pending[module], claims, cfg); err != nil {
			return err
		}
	}
	return nil
}

func reconcileModule(report *ScanReport, module string, matches []ScanMatch, claims []model.Claim, cfg *config.Config) error {
	path := ArtifactPath(cfg, module)

	release, err := lock.AcquireFileLock(path)
	if err != nil {
		return fmt.Errorf("implink: reconcile module %q: %w", module, err)
	}
	defer release()

	artifact, err := LoadArtifact(path)
	if err != nil {
		if !errors.Is(err, ErrNoArtifact) {
			return err
		}
		artifact = &Artifact{Module: module}
	}

	applied := 0
	for _, m := range matches {
		if err := applyLink(artifact, claims, cfg, module, m.ClaimID, m.File, m.Symbol); err != nil {
			report.Errors = append(report.Errors, ScanError{
				File: m.File, Line: m.Line, ClaimID: m.ClaimID, Message: err.Error(),
			})
			continue
		}
		applied++
		report.Matches = append(report.Matches, m)
	}
	if applied == 0 {
		// Nothing to persist. Writing anyway would rewrite (and re-stamp) an
		// artifact this scan did not change, which is exactly the needless churn
		// the batch exists to remove.
		return nil
	}
	return WriteArtifact(artifact, path)
}

// captureSymbol applies symbolPatterns to the few lines immediately
// following a tag comment (the tag itself is expected to sit directly
// above the symbol it documents, the same authoring convention "implink
// set --symbol" callers already follow by hand) and returns the first
// match's captured name, or "" if none of the shallow patterns match
// within that short lookahead — see symbolPatterns' doc comment for why
// this is deliberately best-effort, not a real parse.
func captureSymbol(lines []string, tagLineIdx int) string {
	const lookahead = 3
	for i := tagLineIdx + 1; i < len(lines) && i <= tagLineIdx+lookahead; i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		// Another comment line (e.g. a multi-line doc comment continuing
		// past the tag) — keep looking rather than giving up immediately.
		if strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "*") || strings.HasPrefix(trimmed, "--") {
			continue
		}
		for _, pat := range symbolPatterns {
			if m := pat.FindStringSubmatch(lines[i]); m != nil {
				return m[len(m)-1]
			}
		}
		// First non-comment, non-blank line didn't match any known shape —
		// stop looking rather than risk capturing an unrelated identifier
		// further down.
		return ""
	}
	return ""
}

// isProbablyText is a cheap binary-file guard: a NUL byte anywhere in the
// first chunk of a file is a reliable enough signal that it isn't source
// text worth regex-scanning, without needing a MIME/content-type library
// dependency for what is otherwise a plain text search.
func isProbablyText(data []byte) bool {
	n := len(data)
	if n > 8192 {
		n = 8192
	}
	for i := 0; i < n; i++ {
		if data[i] == 0 {
			return false
		}
	}
	return true
}
