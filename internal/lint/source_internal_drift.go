// source_internal_drift.go implements the "source-internal-drift" lint: an
// internal source's recorded sha256 must exist, the file it names must be
// readable, and the hash the engine computes now must equal the hash the
// author recorded then.
//
// This is the one lint in the family that reads the filesystem, and it is the
// reason internal sources are worth having. An external citation can only be
// dated; an internal one can be PINNED, so drift in the evidence under a claim
// stops being a thing a reader might notice and becomes a thing the gate
// reports. That is the whole trade the external/internal split was made for —
// see model.SourceKind.
//
// THE THREE FAILURES ARE ONE FAILURE, and this is the part to read before
// changing anything here.
//
// A missing sha256, an unreadable file and a mismatched hash are reported by
// this rule with equal weight, because from the gate's position they are
// indistinguishable: in all three cases, NOTHING VERIFIED THAT THIS CITATION
// STILL SAYS WHAT THE CLAIM SAYS IT SAYS. Only the third is drift in the
// literal sense; the first two are a check that could not execute. This
// repository's governing rule (CLAUDE.md) is that such a check is not a pass:
// "a skipped check is indistinguishable from a pass over zero assertions", and
// there is no result here that means "we did not look" and reads as "it is
// fine". model.Source.SHA256's own doc comment commits to the same treatment
// for the missing-hash case in as many words.
//
// The temptation this note exists to refuse is the obvious one: an internal
// source with no sha256 looks like an author who has not filled the field in
// yet, and skipping it looks kind. It is not kind. An unhashed internal source
// is the ONLY kind of citation in the system that renders to the reader as
// verified provenance and is verified by nothing, and it survives a lock:
// sources are signed by lock.LockedClaimHash, so a locked claim citing an
// unhashed file carries a signature over a citation nobody can check, which is
// the precise failure the Source type was introduced to end.
//
// HASHING RULE. With record_id unset, the sha256 pins the whole file. With
// record_id set, it pins the ONE JSONL line whose top-level "id" equals it —
// the raw line as written, minus its line terminator, hashed as bytes. Not the
// re-serialized JSON: a canonicalizing hash would quietly forgive a
// reformatting of the record, and "the bytes on that line changed" is exactly
// the event a reader needs told. Zero matching lines and more than one are
// both findings, for the same reason a mismatch is: the anchor named something
// the file cannot resolve, so nothing was checked.
//
// PATHS RESOLVE AGAINST THE CONFIG'S OWN DIRECTORY, never the process working
// directory — the same anchor claims_dir and source_dirs use (config.Dir). A
// path resolved against the cwd would make the same corpus pass from the
// project root and fail from a subdirectory, which turns a verdict about
// content into a verdict about where someone stood when they ran it.
//
// WHAT THIS COSTS. Unlike every other lint in this package, this one cannot
// run meaningfully over an in-memory corpus with no project directory behind
// it. That case is reported rather than skipped, on the same rule as the other
// three: a hash that could not be resolved was not checked.
package lint

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

func init() {
	Registry = append(Registry, sourceInternalDriftLint{})
}

type sourceInternalDriftLint struct{}

// Name returns this lint's rule name.
func (sourceInternalDriftLint) Name() string { return "source-internal-drift" }

// Check reports every internal source whose content hash cannot be confirmed:
// no recorded hash, no readable file, no resolvable record, or a hash that no
// longer matches.
func (sourceInternalDriftLint) Check(claims []model.Claim, cfg *config.Config) []Finding {
	root := ""
	if cfg != nil {
		root = cfg.Dir()
	}

	var findings []Finding
	for _, c := range claims {
		for i, s := range c.Sources {
			if !s.IsInternal() {
				continue
			}
			where := fmt.Sprintf("sources[%d]", i)
			add := func(msg string) {
				findings = append(findings, Finding{
					LintName: "source-internal-drift",
					ClaimID:  c.ID,
					Severity: SeverityError,
					Message:  where + ": " + msg,
				})
			}

			recorded := strings.ToLower(strings.TrimSpace(s.SHA256))
			if recorded == "" {
				add("internal source records no sha256, so nothing pins it; an unhashed internal citation reads to a reader as verified provenance and is verified by nothing, and it survives a lock unchanged")
				// Deliberately not a "continue": the file may ALSO be
				// unreadable, and an author fixing this by pasting in a hash
				// should learn that in the same run rather than the next one.
			}

			if strings.TrimSpace(s.Path) == "" {
				// source-shape already reports the missing path, in a message
				// that says what to do about it. Repeating it here would send
				// the author looking for two defects where there is one.
				continue
			}
			if root == "" {
				add(fmt.Sprintf("cannot resolve path %q: this corpus was loaded with no project directory to resolve against, so the hash was not checked — and a check that did not execute is not a pass", s.Path))
				continue
			}

			full := filepath.Join(root, filepath.FromSlash(s.Path))
			data, err := os.ReadFile(full)
			if err != nil {
				add(fmt.Sprintf("cannot read path %q (relative to the project config's directory): %v; the citation names a file that is not there to be checked", s.Path, err))
				continue
			}

			payload := data
			if s.RecordID != "" {
				line, err := jsonlRecordLine(data, s.RecordID)
				if err != nil {
					add(fmt.Sprintf("record_id %q in %q: %v", s.RecordID, s.Path, err))
					continue
				}
				payload = line
			}

			sum := sha256.Sum256(payload)
			computed := hex.EncodeToString(sum[:])
			if recorded == "" {
				// Nothing to compare against; the missing-hash finding above
				// is the whole story, and naming the current hash here would
				// invite an author to paste it in without reading the file.
				continue
			}
			if computed == recorded {
				continue
			}
			add(fmt.Sprintf(
				"%s no longer matches its recorded sha256: recorded %s, computed %s. The evidence under this claim changed after the citation was written, so the claim may now be resting on something the file no longer says",
				sourceHashSubject(s), recorded, computed))
		}
	}
	return findings
}

// sourceHashSubject names what was hashed, so a mismatch message distinguishes
// "the file changed" from "that one record changed" without the reader having
// to go back to the YAML to see whether record_id was set.
func sourceHashSubject(s model.Source) string {
	if s.RecordID != "" {
		return fmt.Sprintf("record %q of %q", s.RecordID, s.Path)
	}
	return fmt.Sprintf("file %q", s.Path)
}

// jsonlRecordLine returns the raw bytes of the single line in a JSONL file
// whose top-level "id" equals recordID, with its line terminator removed.
//
// The returned slice is the line AS AUTHORED — key order, spacing and all —
// because that is what the recorded sha256 was taken over. Re-serializing it
// would make the hash stable across a reformatting of the record, and a
// reformatting is a change to the evidence a reader may need to see.
//
// A line that is not valid JSON, or whose "id" is not a JSON string, simply
// does not match: it cannot be the record the author pinned. If that line WAS
// the record, the caller gets the not-found finding, which is the truthful
// report — the file no longer contains a resolvable record with that id.
func jsonlRecordLine(data []byte, recordID string) ([]byte, error) {
	var matches [][]byte
	var matchedLines []int

	for n, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSuffix(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &fields); err != nil {
			continue
		}
		rawID, ok := fields["id"]
		if !ok {
			continue
		}
		var id string
		if err := json.Unmarshal(rawID, &id); err != nil {
			continue
		}
		if id != recordID {
			continue
		}
		matches = append(matches, []byte(line))
		matchedLines = append(matchedLines, n+1)
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return nil, fmt.Errorf("no line has a top-level \"id\" with that value, so there is no record left to hash")
	default:
		return nil, fmt.Errorf("%d lines share that id (lines %s); an id that names more than one record pins none of them", len(matches), joinInts(matchedLines))
	}
}

// joinInts renders line numbers for the duplicate-record message.
func joinInts(ns []int) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = fmt.Sprint(n)
	}
	return strings.Join(parts, ", ")
}
