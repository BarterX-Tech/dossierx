// brief_lock.go is the write side of the "dossierx brief" noun (NIT-205):
// brief lock, brief unlock and brief reaudit.
//
// A brief's approval is a record in build/ledger/lock-store.json's `briefs`
// map (lock.BriefRecord): the brief's lock hash (summary, rests_on, body), the
// human's --reason, the time, the approved text, and one baseline per rests_on
// claim — that claim's ContentHash as the brief read it. The brief's own file
// carries `status: locked`, exactly as a claim or the constitution does.
//
// THE APPROVAL CONTRACT. A brief's lock, review or findings never gate a
// claim: nothing here is read by `claim lock`, a claim's review_pending, the
// claim graph or any claim hash. A claim that cites a brief still owns that
// pin (`source-internal-drift` on the claim). The baselines point from the
// brief to the claims, and only a claim moving makes the BRIEF review-pending
// (internal/briefs/lockstate.go).
//
// LOCKING. The claims sentinel first (every authored-file write takes it: the
// brief file is authored content, and the comment ops on a brief take it too,
// so a thread cannot land between the open-thread gate and the lock), then the
// lock-store sentinel — the project-wide order claims -> lock-store. Inside
// them the brief and the claims are read fresh; the reads before them only
// served the dry run.
//
// ORDER OF WRITES. The brief's status line first, then the store. A failure
// between the two leaves `status: locked` with no record, which check reports
// as brief-unrecorded: loud, never a record that approves a file saying draft.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/atomicfile"
	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// briefLockData is `brief lock`'s payload: the record just written.
type briefLockData struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	// Relocked is true when this lock replaces an earlier approval record of
	// the brief — a standing one (re-locking an edited brief) or one released
	// by brief unlock (unlock, edit, lock) — and false on a brief's first
	// lock. It is exactly when Carried can be non-empty.
	Relocked  bool              `json:"relocked"`
	Hash      string            `json:"hash"`
	LockedAt  string            `json:"locked_at"`
	Reason    string            `json:"reason"`
	Baselines map[string]string `json:"baselines"`
	// Carried is the rests_on ids whose baselines a re-lock kept from the
	// earlier record, standing or released, rather than re-reading (always
	// empty on a first lock).
	Carried []string `json:"carried_baselines"`
}

// briefUnlockData is `brief unlock`'s payload.
type briefUnlockData struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Released bool   `json:"released"`
	Reason   string `json:"reason"`
}

// briefReauditData is `brief reaudit`'s payload, the preview and the write
// alike: the per-claim diff, and on --confirm the baselines now recorded.
type briefReauditData struct {
	ID            string                `json:"id"`
	Path          string                `json:"path"`
	LockState     string                `json:"lock_state"`
	ReviewPending bool                  `json:"review_pending"`
	Trigger       string                `json:"review_pending_trigger"`
	ChangedClaims []briefs.ChangedClaim `json:"changed_claims"`
	Confirmed     bool                  `json:"confirmed"`
	Reason        string                `json:"reason,omitempty"`
}

// briefTarget resolves a brief argument against a freshly loaded tree, with the
// brief_not_found refusal brief show gives.
func briefTarget(set *briefs.Set, arg, verb string) (briefs.Brief, error) {
	b, ok := set.Lookup(arg)
	if !ok {
		return briefs.Brief{}, cliout.Errorf(cliout.CodeBriefNotFound, "%s: no brief at %q", verb, arg).
			WithHint(fmt.Sprintf("run: dossierx brief list — and pass the path it prints (%s/<folder>/<slug>.md) or the <folder>.<slug> id", set.DisplayDir))
	}
	return b, nil
}

// briefErrorFindings is every ERROR finding about b — on its path, its folder or
// the tree — out of the brief rule set evaluated against claims. A brief lock
// signs the brief, so a brief that `check` refuses is not signed.
func briefErrorFindings(set *briefs.Set, b briefs.Brief, claims []model.Claim) []lint.Finding {
	var out []lint.Finding
	for _, f := range findingsAbout(set.Findings(claims), b.Path) {
		if f.Severity != lint.SeverityWarning {
			out = append(out, f)
		}
	}
	return out
}

// rewriteBriefFile rewrites a brief's file in place, keeping its mode, after
// checking it is still the regular file (not a link) it was when read, and
// still holds the bytes it was read with.
func rewriteBriefFile(cfg *config.Config, b briefs.Brief, before, after []byte) error {
	file := briefs.FilePath(cfg, b)
	info, err := os.Lstat(file)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", b.Path)
	}
	now, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	if string(now) != string(before) {
		return fmt.Errorf("%s changed while it was being written; nothing was written, run the command again", b.Path)
	}
	return atomicfile.Write(file, after, info.Mode().Perm())
}

// briefBytesAsLoaded reads a brief's bytes once more under the sentinels and
// requires them to be the bytes the tree was just parsed from, so the hash a
// lock records is the hash of exactly the file it rewrites: an editor's save
// landing between discovery and this read is refused, never signed under the
// old bytes' hash.
func briefBytesAsLoaded(cfg *config.Config, b briefs.Brief) ([]byte, error) {
	raw, err := readBriefFile(cfg, b)
	if err != nil {
		return nil, err
	}
	if strings.ReplaceAll(string(raw), "\r\n", "\n") != b.Content {
		return nil, fmt.Errorf("%s changed while it was being read; nothing was written, run the command again", b.Path)
	}
	return raw, nil
}

// readBriefFile reads a brief's bytes, refusing a link.
func readBriefFile(cfg *config.Config, b briefs.Brief) ([]byte, error) {
	file := briefs.FilePath(cfg, b)
	info, err := os.Lstat(file)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", b.Path)
	}
	return os.ReadFile(file)
}

func newBriefLockCmd() *cobra.Command {
	var reason string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "lock <path-or-id>",
		Short: "Lock a brief: record its content hash, the human's --reason and a baseline per rests_on claim",
		Long: "Lock a brief. Its status becomes locked and the lock store records the brief's\n" +
			"hash (summary, rests_on and body), the human's --reason, the approved text and,\n" +
			"for every claim it rests on, that claim's content hash as a baseline: when one\n" +
			"of those claims later changes, the brief is review_pending. A lock over an\n" +
			"earlier approval (an edited brief re-locked, or a lock after brief unlock)\n" +
			"keeps that approval's baselines for every rests_on claim still listed, so a\n" +
			"claim that moved stays review_pending until brief reaudit --confirm. A brief\n" +
			"that is locked and unchanged is refused already_locked; an open comment\n" +
			"thread on it is refused comment_open. Locking a brief gates no claim.",
		Args: cobra.ExactArgs(1),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			cfg, err := loadConfig()
			if err != nil {
				return cmdResult{}, err
			}
			if dryRun {
				claims, err := loadClaims(cfg)
				if err != nil {
					return cmdResult{}, err
				}
				set := briefs.Load(cfg)
				b, err := briefTarget(set, args[0], "brief lock")
				if err != nil {
					return cmdResult{}, err
				}
				store, storeErr := lock.LoadStore(storePath(cfg))
				return dryRunResult(cmd, "brief lock", briefLockDryRun(cfg, set, b, claims, store, storeErr, reason)), nil
			}
			if err := requireReason("brief lock", reason); err != nil {
				return cmdResult{}, err
			}
			gitignoreWarnings, err := refuseIfStoresGitignored(cfg, "brief lock")
			if err != nil {
				return cmdResult{}, err
			}
			releaseClaims, err := lock.AcquireFileLock(claimsSentinelPath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "brief lock: %w", err)
			}
			defer releaseClaims()
			claims, err := loadClaims(cfg)
			if err != nil {
				return cmdResult{}, err
			}
			set := briefs.Load(cfg)
			b, err := briefTarget(set, args[0], "brief lock")
			if err != nil {
				return cmdResult{}, err
			}
			raw, err := briefBytesAsLoaded(cfg, b)
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "brief lock: %w", err)
			}
			if fs := briefErrorFindings(set, b, claims); len(fs) > 0 {
				return cmdResult{}, cliout.Errorf(cliout.CodeLintFailed, "brief lock: refused, %s has %d error finding(s)", b.Path, len(fs)).
					WithDetails(map[string]any{"lint_findings": briefFindingsData(fs)}).
					WithHint("fix the findings (dossierx check --validate names them), then lock again; a lock signs a brief check would refuse")
			}
			if n := b.OpenThreads(); n > 0 {
				return cmdResult{}, briefCommentOpen("brief lock", b, n)
			}
			storeRelease, err := lock.AcquireFileLock(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "brief lock: %w", err)
			}
			defer storeRelease()
			store, err := lock.LoadStore(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "brief lock: %w", err).
					WithHint("restore " + config.LockStoreDisplayPath + " from version control; nothing was written")
			}
			rec, has := store.BriefRecordFor(b.ID)
			standing := has && !rec.Released()
			if b.Status == briefs.StatusLocked && standing && briefs.ContentMoved(b, rec) == "" {
				return cmdResult{}, cliout.Errorf(cliout.CodeAlreadyLocked,
					"brief lock: %s is already locked and unchanged since %s", b.Path, rec.At).
					WithHint("edit the brief first; a lock signs a change, and there is none. To accept claims that moved under it, dossierx brief reaudit " + b.Path)
			}
			if err := crossPreLedger(cfg, store, claims, "brief lock"); err != nil {
				return cmdResult{}, err
			}
			// The store is saved below whatever the on-load migrations did.
			_, adopted := prepareStore(cfg, store, claims)

			// A lock over an earlier record carries its baselines forward,
			// whether that record stands (a re-lock of an edited brief) or an
			// unlock released it (unlock, edit, lock): the lock approves the
			// brief's own words, and a claim that moved under the brief stays
			// review_pending until brief reaudit shows it. Releasing a record
			// ends the approval, not the reading the brief still owes a moved
			// claim — otherwise unlock then lock would accept it unseen.
			var prev *lock.BriefRecord
			if has {
				prev = &rec
			}
			hashes, receipts, carried, unknown := briefs.RelockBaselines(b, claims, prev)
			if len(unknown) > 0 {
				// briefErrorFindings has already refused an unknown id; this is
				// the belt to its braces, so a baseline is never silently missing.
				return cmdResult{}, cliout.Errorf(cliout.CodeLintFailed, "brief lock: %s rests on %s, which is not a claim", b.Path, strings.Join(unknown, ", "))
			}
			locked, err := briefs.SetStatus(raw, briefs.StatusLocked)
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "brief lock: %s: %w", b.Path, err)
			}
			if string(locked) != string(raw) {
				if err := rewriteBriefFile(cfg, b, raw, locked); err != nil {
					return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "brief lock: %w", err)
				}
			}
			restsOn := append([]string{}, b.RestsOn...)
			at := time.Now().UTC().Format(time.RFC3339Nano)
			lock.RecordBriefApproval(store, b.ID, lock.BriefRecord{
				Path:      b.Path,
				Hash:      b.LockHash,
				At:        at,
				Actor:     lock.DefaultActor(),
				Reason:    reason,
				Approved:  lock.BriefApproved{Summary: b.Summary, RestsOn: restsOn, Markdown: b.Body},
				Images:    b.ImageDigests(),
				Baselines: hashes,
				Receipts:  receipts,
			})
			if err := store.Save(); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "brief lock: %w", err).
					WithHint(fmt.Sprintf("%s now says status: locked with no record, which check reports as brief-unrecorded; fix the write failure and run brief lock again", b.Path))
			}
			if carried == nil {
				carried = []string{}
			}
			data := briefLockData{ID: b.ID, Path: b.Path, Relocked: has, Hash: b.LockHash, LockedAt: at, Reason: reason, Baselines: hashes, Carried: carried}
			return cmdResult{
				Warnings: append(gitignoreWarnings, adoptionWarnings(adopted)...),
				Data:     data,
				Text: func() {
					verb := "locked"
					if data.Relocked {
						verb = "re-locked"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "brief lock: %s %s (hash %s, %d rests_on baseline(s))\n", verb, b.Path, b.LockHash, len(hashes))
				},
			}, nil
		}),
	}
	cmd.Flags().StringVar(&reason, "reason", "", "the human's approving words (required; recorded in the lock store)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what locking would do, and write nothing")
	return cmd
}

// briefCommentOpen is the comment_open refusal.
func briefCommentOpen(verb string, b briefs.Brief, n int) error {
	return cliout.Errorf(cliout.CodeCommentOpen, "%s: refused, %s has %d open comment thread(s)", verb, b.Path, n).
		WithHint(fmt.Sprintf("run: dossierx comment list %s --open — reply on the thread; resolving it is the human's, in the served viewer's comment rail on the brief's page, and that is the yes this waits for; until then this refusal stands", b.Path))
}

// briefLockDryRun previews brief lock: every refusal the write path makes, in
// its order, without taking a sentinel or writing a byte.
func briefLockDryRun(cfg *config.Config, set *briefs.Set, b briefs.Brief, claims []model.Claim, store *lock.Store, storeErr error, reason string) *cliout.DryRun {
	dr := cliout.NewDryRun("lock brief "+b.Path).Transition(string(b.Status), string(briefs.StatusLocked))
	if strings.TrimSpace(reason) == "" {
		dr.Lacking("--reason")
	}
	storesArePrecondition(dr, cfg)
	errs := briefErrorFindings(set, b, claims)
	dr.Require("brief_has_no_error_findings", len(errs) == 0, boolDetail(len(errs) == 0,
		"no error finding on the brief, its folder or the tree",
		fmt.Sprintf("%d error finding(s), the first: %s: %s", len(errs), firstFindingRule(errs), firstFindingMessage(errs))))
	open := b.OpenThreads()
	dr.Require("no_open_comment_threads", open == 0, boolDetail(open == 0,
		"no open comment thread on the brief",
		fmt.Sprintf("%d open comment thread(s): the human resolves them first (comment_open)", open)))
	dr.Require("store_readable", storeErr == nil, boolDetail(storeErr == nil,
		config.LockStoreDisplayPath+" loads", fmt.Sprintf("%v", storeErr)))
	var prev *lock.BriefRecord
	if storeErr == nil {
		rec, has := store.BriefRecordFor(b.ID)
		already := b.Status == briefs.StatusLocked && has && !rec.Released() && briefs.ContentMoved(b, rec) == ""
		dr.Require("not_already_locked", !already, boolDetail(!already,
			"the brief is not locked-and-unchanged",
			"already locked and unchanged since "+rec.At))
		if has {
			prev = &rec
		}
	}
	preLedgerPrecondition(dr, cfg, claims)
	hashes, _, carried, _ := briefs.RelockBaselines(b, claims, prev)
	dr.Effect(fmt.Sprintf("%s's status is set to locked; every other byte stays as written", b.Path))
	dr.Effect(fmt.Sprintf("the lock store records the brief's hash, the sha256 of each image it references, your --reason, its approved text and %d rests_on baseline(s); a later change to one of those claims makes the brief review_pending", len(hashes)))
	if len(carried) > 0 {
		dr.Effect(fmt.Sprintf("a re-lock: %d baseline(s) are kept from the earlier approval, standing or released by brief unlock (%s), so a claim that moved under the brief stays review_pending until brief reaudit --confirm", len(carried), strings.Join(carried, ", ")))
	}
	dr.Effect("no claim is touched: a brief gates no claim and sets review_pending on none")
	if carried == nil {
		carried = []string{}
	}
	dr.Propose("reason", reason).Propose("baselines", hashes).Propose("carried_baselines", carried)
	return dr
}

func firstFindingRule(fs []lint.Finding) string {
	if len(fs) == 0 {
		return ""
	}
	return fs[0].LintName
}

func firstFindingMessage(fs []lint.Finding) string {
	if len(fs) == 0 {
		return ""
	}
	return fs[0].ClaimID + " " + fs[0].Message
}

func newBriefUnlockCmd() *cobra.Command {
	var reason string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "unlock <path-or-id>",
		Short: "Unlock a locked brief back to draft; --reason stamps the release on its record",
		Long: "Unlock a brief. Its status becomes draft and its record in the lock store is\n" +
			"kept and stamped released with --reason, as claim unlock does, so the evidence\n" +
			"that it was approved survives. Always allowed on a locked brief: it is the way\n" +
			"to edit one.",
		Args: cobra.ExactArgs(1),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			cfg, err := loadConfig()
			if err != nil {
				return cmdResult{}, err
			}
			if dryRun {
				set := briefs.Load(cfg)
				b, err := briefTarget(set, args[0], "brief unlock")
				if err != nil {
					return cmdResult{}, err
				}
				dr := cliout.NewDryRun("unlock brief "+b.Path).Transition(string(b.Status), string(briefs.StatusDraft))
				if strings.TrimSpace(reason) == "" {
					dr.Lacking("--reason")
				}
				dr.Require("brief_is_locked", b.Status == briefs.StatusLocked, boolDetail(b.Status == briefs.StatusLocked,
					"the brief's status is locked", "the brief is already a draft (not_locked)"))
				dr.Effect(fmt.Sprintf("%s's status is set to draft; every other byte stays as written", b.Path))
				dr.Effect("its record in the lock store is kept and stamped released with your --reason")
				dr.Propose("reason", reason)
				return dryRunResult(cmd, "brief unlock", dr), nil
			}
			if err := requireReason("brief unlock", reason); err != nil {
				return cmdResult{}, err
			}
			releaseClaims, err := lock.AcquireFileLock(claimsSentinelPath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "brief unlock: %w", err)
			}
			defer releaseClaims()
			set := briefs.Load(cfg)
			b, err := briefTarget(set, args[0], "brief unlock")
			if err != nil {
				return cmdResult{}, err
			}
			if b.Status != briefs.StatusLocked {
				return cmdResult{}, cliout.Errorf(cliout.CodeNotLocked, "brief unlock: %s is already a draft", b.Path)
			}
			raw, err := readBriefFile(cfg, b)
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "brief unlock: %w", err)
			}
			storeRelease, err := lock.AcquireFileLock(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "brief unlock: %w", err)
			}
			defer storeRelease()
			store, err := lock.LoadStore(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "brief unlock: %w", err)
			}
			draft, err := briefs.SetStatus(raw, briefs.StatusDraft)
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "brief unlock: %s: %w", b.Path, err)
			}
			// The release first, then the file. A failure between the two
			// leaves the file saying locked on a released record, which check
			// reports as brief-unrecorded — loud, and a re-run of this command
			// finishes it. The reverse order left a draft on a standing record,
			// which is brief-orphan now but used to be silent, and which this
			// command would then refuse as not locked.
			released := lock.ReleaseBriefApproval(store, b.ID, lock.Approval{Actor: lock.DefaultActor(), Reason: reason})
			if released {
				if err := store.Save(); err != nil {
					return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "brief unlock: %w", err).
						WithHint("nothing was written; fix the write failure and run brief unlock again")
				}
			}
			if err := rewriteBriefFile(cfg, b, raw, draft); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "brief unlock: %w", err).
					WithHint(fmt.Sprintf("the approval is released but %s still says status: locked (check reports brief-unrecorded): fix the write failure and run: dossierx brief unlock %s --reason \"<their words>\"", b.Path, b.Path))
			}
			data := briefUnlockData{ID: b.ID, Path: b.Path, Released: released, Reason: reason}
			return cmdResult{
				Data: data,
				Text: func() {
					fmt.Fprintf(cmd.OutOrStdout(), "brief unlock: %s is a draft (record released: %v)\n", b.Path, released)
				},
			}, nil
		}),
	}
	cmd.Flags().StringVar(&reason, "reason", "", "the human's words for the release (required; stamped on the record)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what unlocking would do, and write nothing")
	return cmd
}

func newBriefReauditCmd() *cobra.Command {
	var reason string
	var confirm, dryRun bool
	cmd := &cobra.Command{
		Use:   "reaudit <path-or-id>",
		Short: "Show what changed in the claims a locked brief rests on; --confirm --reason records the human's yes and refreshes the baselines",
		Long: "Preview, for a locked brief, every rests_on claim whose content moved since the\n" +
			"brief's baseline: its wording then and now. With --confirm and --reason, record the\n" +
			"human's yes and refresh the baselines to the claims as they read now, which clears\n" +
			"review_pending. The brief's own text and approval are untouched. Refused\n" +
			"comment_open while a thread on the brief is open.",
		Args: cobra.ExactArgs(1),
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			cfg, err := loadConfig()
			if err != nil {
				return cmdResult{}, err
			}
			if dryRun || !confirm {
				claims, err := loadClaims(cfg)
				if err != nil {
					return cmdResult{}, err
				}
				set := briefs.Load(cfg)
				b, err := briefTarget(set, args[0], "brief reaudit")
				if err != nil {
					return cmdResult{}, err
				}
				store, storeErr := lock.LoadStore(storePath(cfg))
				if storeErr != nil {
					return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "brief reaudit: %w", storeErr)
				}
				r := briefs.Evaluate(set, claims, store).Review(b)
				if dryRun {
					return dryRunResult(cmd, "brief reaudit", briefReauditDryRun(cfg, b, r, claims, reason)), nil
				}
				if r.LockState == briefs.LockDraft || r.LockState == briefs.LockUnrecorded {
					return cmdResult{}, briefNotLocked("brief reaudit", b, r)
				}
				data := briefReauditData{ID: b.ID, Path: b.Path, LockState: string(r.LockState), ReviewPending: r.ReviewPending, Trigger: r.ReviewPendingTrigger, ChangedClaims: r.ChangedClaims}
				return cmdResult{Data: data, Text: func() { writeBriefReauditText(cmd, data) }}, nil
			}
			if err := requireReason("brief reaudit", reason); err != nil {
				return cmdResult{}, err
			}
			gitignoreWarnings, err := refuseIfStoresGitignored(cfg, "brief reaudit")
			if err != nil {
				return cmdResult{}, err
			}
			releaseClaims, err := lock.AcquireFileLock(claimsSentinelPath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "brief reaudit: %w", err)
			}
			defer releaseClaims()
			claims, err := loadClaims(cfg)
			if err != nil {
				return cmdResult{}, err
			}
			set := briefs.Load(cfg)
			b, err := briefTarget(set, args[0], "brief reaudit")
			if err != nil {
				return cmdResult{}, err
			}
			storeRelease, err := lock.AcquireFileLock(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "brief reaudit: %w", err)
			}
			defer storeRelease()
			store, err := lock.LoadStore(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "brief reaudit: %w", err)
			}
			r := briefs.Evaluate(set, claims, store).Review(b)
			switch {
			case r.LockState == briefs.LockDraft || r.LockState == briefs.LockUnrecorded:
				return cmdResult{}, briefNotLocked("brief reaudit", b, r)
			case r.LockState == briefs.LockEdited:
				return cmdResult{}, cliout.Errorf(cliout.CodeIntegrityFailed, "brief reaudit: refused, %s has been edited since it was approved (brief-content-drift)", b.Path).
					WithHint("a reaudit accepts moved claims, it does not approve the brief's own edit. Restore the file from version control; or, only on the human's yes to the edit, dossierx brief unlock " + b.Path + " --reason \"…\", fix the brief, and dossierx brief lock " + b.Path + " --reason \"…\" — that lock keeps the baselines, so this reaudit is still there to run")
			case b.OpenThreads() > 0:
				return cmdResult{}, briefCommentOpen("brief reaudit", b, b.OpenThreads())
			case !r.ReviewPending:
				return cmdResult{}, cliout.Errorf(cliout.CodeNotReviewPending, "brief reaudit: %s is not review_pending: no claim it rests on has moved since its baseline", b.Path)
			}
			hashes, receipts, unknown := briefs.Baselines(b, claims)
			if len(unknown) > 0 {
				return cmdResult{}, cliout.Errorf(cliout.CodeWrongState, "brief reaudit: refused, %s rests on %s, which is no longer a claim (brief-rests-on-missing)", b.Path, strings.Join(unknown, ", ")).
					WithHint("a reaudit cannot baseline a claim that is gone: unlock the brief, rest it on the claims it now reads, and the human re-locks it")
			}
			if err := crossPreLedger(cfg, store, claims, "brief reaudit"); err != nil {
				return cmdResult{}, err
			}
			_, adopted := prepareStore(cfg, store, claims)
			ids := make([]string, 0, len(r.ChangedClaims))
			for _, c := range r.ChangedClaims {
				ids = append(ids, c.ID)
			}
			if !lock.RecordBriefReaudit(store, b.ID, hashes, receipts, b.ImageDigests(), lock.Approval{Actor: lock.DefaultActor(), Reason: reason}, ids) {
				return cmdResult{}, briefNotLocked("brief reaudit", b, r)
			}
			if err := store.Save(); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "brief reaudit: %w", err)
			}
			data := briefReauditData{ID: b.ID, Path: b.Path, LockState: string(r.LockState), ReviewPending: false, ChangedClaims: r.ChangedClaims, Confirmed: true, Reason: reason}
			return cmdResult{
				Warnings: append(gitignoreWarnings, adoptionWarnings(adopted)...),
				Data:     data,
				Text:     func() { writeBriefReauditText(cmd, data) },
			}, nil
		}),
	}
	cmd.Flags().BoolVar(&confirm, "confirm", false, "record the human's yes and refresh the baselines (requires --reason)")
	cmd.Flags().StringVar(&reason, "reason", "", "the human's words for accepting the changed claims (required with --confirm)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what --confirm would do, and write nothing")
	return cmd
}

func briefNotLocked(verb string, b briefs.Brief, r briefs.Review) error {
	return cliout.Errorf(cliout.CodeNotLocked, "%s: %s is not locked with a standing approval (%s)", verb, b.Path, r.LockState).
		WithHint("a reaudit refreshes a locked brief's baselines; a draft has none. Lock it with dossierx brief lock " + b.Path + " --reason \"…\"")
}

// briefReauditDryRun previews brief reaudit --confirm.
func briefReauditDryRun(cfg *config.Config, b briefs.Brief, r briefs.Review, claims []model.Claim, reason string) *cliout.DryRun {
	dr := cliout.NewDryRun("confirm the claims that moved under brief " + b.Path)
	if strings.TrimSpace(reason) == "" {
		dr.Lacking("--reason")
	}
	storesArePrecondition(dr, cfg)
	locked := r.LockState == briefs.LockLocked || r.LockState == briefs.LockEdited
	dr.Require("brief_is_locked", locked, boolDetail(locked, "the brief is locked with a standing approval", fmt.Sprintf("the brief is %s (not_locked)", r.LockState)))
	dr.Require("brief_unchanged_since_approval", r.LockState != briefs.LockEdited, boolDetail(r.LockState != briefs.LockEdited,
		"the brief's own text is as approved", "the brief was edited since approval (brief-content-drift): restore it from version control, or unlock, fix and lock it on the human's yes, first"))
	open := b.OpenThreads()
	dr.Require("no_open_comment_threads", open == 0, boolDetail(open == 0, "no open comment thread on the brief",
		fmt.Sprintf("%d open comment thread(s) (comment_open)", open)))
	dr.Require("review_pending", r.ReviewPending, boolDetail(r.ReviewPending,
		fmt.Sprintf("%d rests_on claim(s) moved since the baseline", len(r.ChangedClaims)),
		"no claim it rests on has moved (not_review_pending)"))
	_, _, unknown := briefs.Baselines(b, claims)
	dr.Require("rests_on_claims_exist", len(unknown) == 0, boolDetail(len(unknown) == 0,
		"every rests_on claim exists", "gone: "+strings.Join(unknown, ", ")+" (brief-rests-on-missing)"))
	preLedgerPrecondition(dr, cfg, claims)
	dr.Effect("the brief's rests_on baselines are refreshed to the claims as they read now, and your --reason is recorded; review_pending clears")
	dr.Effect("the brief's own text and approval, and every claim, are untouched")
	dr.Propose("reason", reason).Propose("changed_claims", r.ChangedClaims)
	return dr
}

func writeBriefReauditText(cmd *cobra.Command, d briefReauditData) {
	out := cmd.OutOrStdout()
	if d.Confirmed {
		fmt.Fprintf(out, "brief reaudit: %s baselines refreshed for %d claim(s)\n", d.Path, len(d.ChangedClaims))
		return
	}
	if len(d.ChangedClaims) == 0 {
		fmt.Fprintf(out, "brief reaudit: %s is %s; no rests_on claim has moved since its baseline\n", d.Path, d.LockState)
		return
	}
	fmt.Fprintf(out, "brief reaudit: %s is review_pending (%s); %d claim(s) changed:\n", d.Path, d.Trigger, len(d.ChangedClaims))
	for _, c := range d.ChangedClaims {
		if c.Missing {
			fmt.Fprintf(out, "  %s: gone\n", c.ID)
			continue
		}
		fmt.Fprintf(out, "  %s:\n", c.ID)
		if c.Baseline == nil || c.Current == nil {
			if c.Baseline == nil {
				fmt.Fprintf(out, "    was: (%s)\n", c.BaselineNote)
			}
			if c.Current != nil {
				fmt.Fprintf(out, "    now: %s\n", c.Current.Summary)
			}
			continue
		}
		// A line diff of the wording a reader compares — summary, body and
		// steps — so a body-only change shows up here as it does in the JSON.
		for _, line := range wordingDiff(wordingLines(c.Baseline), wordingLines(c.Current)) {
			fmt.Fprintf(out, "    %s\n", line)
		}
	}
	fmt.Fprintf(out, "brief reaudit: the human confirms with dossierx brief reaudit %s --confirm --reason \"…\"\n", d.Path)
}

// wordingLines is a claim's wording as the lines a reaudit preview compares.
func wordingLines(w *briefs.Wording) []string {
	lines := []string{"summary: " + w.Summary}
	for _, l := range strings.Split(strings.TrimRight(w.Body, "\n"), "\n") {
		lines = append(lines, "body: "+l)
	}
	for i, st := range w.Steps {
		lines = append(lines, fmt.Sprintf("step %d: %s", i+1, st))
	}
	return lines
}

// wordingDiff is a longest-common-subsequence line diff: "- " for a line only
// the baseline has, "+ " for one only the current wording has, and "  " for a
// line both keep. A claim's wording is short (its body is capped), so the
// quadratic table is small.
func wordingDiff(was, now []string) []string {
	n, m := len(was), len(now)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if was[i] == now[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	var out []string
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && was[i] == now[j]:
			out = append(out, "  "+was[i])
			i++
			j++
		case j < m && (i == n || lcs[i][j+1] >= lcs[i+1][j]):
			out = append(out, "+ "+now[j])
			j++
		default:
			out = append(out, "- "+was[i])
			i++
		}
	}
	return out
}
