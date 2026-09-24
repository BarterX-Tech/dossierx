package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/constitution"
	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// The constitution noun (NIT-6): the project-root constitution.yaml is the
// one lockable roof over every module — not a module, not a claim, not a
// graph node, never a rests_on target. Two leaves: show prints the full text
// (an agent drafts against the words, never the hash) plus the digest and
// the lock state; lock records the file's content hash and the human's
// reason in the lock store, the same integrity contract a claim gets.
//
// Both keep working while the roof is missing, draft or edited — they are
// the way out of CONSTITUTION_NOT_LOCKED, so they cannot be behind it.
func newConstitutionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "constitution",
		Short: "Show or lock the project-root constitution.yaml (the roof; not a module)",
	}
	cmd.AddCommand(newConstitutionShowCmd(), newConstitutionLockCmd())
	return commandGroup(cmd)
}

// constitutionShowData is `constitution show`'s payload: the words, the
// digest and the lock verdict, in one envelope.
type constitutionShowData struct {
	Path    string                 `json:"path"`
	Present bool                   `json:"present"`
	Status  string                 `json:"status,omitempty"`
	Digest  constitution.Digest    `json:"digest"`
	Lock    constitution.Verdict   `json:"lock"`
	Text    string                 `json:"text"`
	Section []constitution.Section `json:"sections"`
}

// constitutionVerdict is the read-only gate evaluation every caller shares:
// the file at its configured path against the store's record. The store is
// read without the sentinel, as every read-only path reads it.
func constitutionVerdict(cfg *config.Config) constitution.Verdict {
	var rec *constitution.LockRecord
	if store, err := lock.LoadStore(storePath(cfg)); err == nil {
		rec = store.Constitution
	}
	return constitution.EvaluateAt(cfg.ConstitutionPath(), rec)
}

// constitutionVerdictWith is constitutionVerdict over a store the caller
// already holds under the sentinel.
func constitutionVerdictWith(cfg *config.Config, store *lock.Store) constitution.Verdict {
	var rec *constitution.LockRecord
	if store != nil {
		rec = store.Constitution
	}
	return constitution.EvaluateAt(cfg.ConstitutionPath(), rec)
}

// constitutionGate is the refusal `claim lock` (single, batch and policy
// paths alike) and plain `check` share (NIT-26). It returns nil when the
// roof is locked and its hash matches the record; otherwise a
// CONSTITUTION_NOT_LOCKED error — or CONSTITUTION_OVER_CAP when the file is
// over the word cap, since trimming it is the first thing to do either way —
// with the verdict in details and the recovery in the hint. Hard refuse; no
// warn-and-continue, no config switch.
func constitutionGate(verb string, v constitution.Verdict) error {
	if v.Locked() && !v.OverCap {
		return nil
	}
	details := verdictDetails(v)
	if v.OverCap {
		return cliout.Errorf(cliout.CodeConstitutionOverCap,
			"%s: refused, constitution is %d of %d words", verb, v.Words, v.WordCap).
			WithDetails(details).
			WithHint("trim constitution.yaml under the cap (it is the critical brief, not a design document), then `dossierx constitution lock --reason \"<the human's words>\"`")
	}
	return cliout.Errorf(cliout.CodeConstitutionNotLocked,
		"%s: refused, %s — no module work until the constitution is locked", verb, v.Detail()).
		WithDetails(details).
		WithHint(v.Hint())
}

func verdictDetails(v constitution.Verdict) map[string]any {
	return map[string]any{
		"constitution": v,
		"state":        string(v.State),
		"path":         v.Path,
	}
}

// constitutionPrecondition is the dry-run twin of constitutionGate: a
// preview that did not name the roof would send an agent to its human for a
// yes the real run then refuses.
func constitutionPrecondition(dr *cliout.DryRun, v constitution.Verdict) {
	detail := v.Detail()
	if v.OverCap {
		detail = fmt.Sprintf("constitution is %d of %d words (CONSTITUTION_OVER_CAP)", v.Words, v.WordCap)
	}
	dr.Require("constitution_locked", v.Locked() && !v.OverCap, detail)
}

func newConstitutionShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the constitution's full text, its digest and its lock state",
		Long: "Print the project-root constitution.yaml in full — every invariant, glossary\n" +
			"entry and decision — with the word meter, the content hash and whether the\n" +
			"lock store's record still matches the file. An agent drafts module claims\n" +
			"against these words; the digest alone (counts and a hash) is not enough to\n" +
			"draft against. Works whether or not the roof is locked.",
		Args: cobra.NoArgs,
		RunE: envelopeRunE(func(cmd *cobra.Command, _ []string) (cmdResult, error) {
			cfg, err := loadConfig()
			if err != nil {
				return cmdResult{}, err
			}
			f, err := constitution.LoadOptional(cfg.ConstitutionPath())
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeInvalidConfig, "constitution show: %w", err)
			}
			data := constitutionShowData{
				Path:    cfg.ConstitutionPath(),
				Present: f != nil,
				Digest:  constitution.NewDigest(cfg.ConstitutionPath(), f),
				Lock:    constitutionVerdict(cfg),
				Text:    constitution.Text(f),
				Section: constitution.Sections(f),
			}
			if f != nil {
				data.Status = string(f.Status)
			}
			if data.Section == nil {
				data.Section = []constitution.Section{}
			}
			return cmdResult{Data: data, Text: func() { writeConstitutionShowText(cmd, data) }}, nil
		}),
	}
}

func writeConstitutionShowText(cmd *cobra.Command, d constitutionShowData) {
	out := cmd.OutOrStdout()
	if !d.Present {
		fmt.Fprintf(out, "constitution: absent (%s)\n", d.Path)
		fmt.Fprintln(out, "  lock: missing — no module claim locks until constitution.yaml exists and is locked")
		return
	}
	fmt.Fprintf(out, "constitution: %s (%s)\n", d.Path, d.Status)
	writeConstitutionDigest(cmd, d.Digest)
	fmt.Fprintf(out, "  lock: %s\n", d.Lock.Detail())
	if d.Lock.Reason != "" {
		fmt.Fprintf(out, "  locked: %s (%q)\n", d.Lock.LockedAt, d.Lock.Reason)
	}
	if d.Text != "" {
		fmt.Fprintln(out)
		fmt.Fprint(out, d.Text)
	}
}

func writeConstitutionDigest(cmd *cobra.Command, d constitution.Digest) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "  words: %d of %d\n", d.Words, d.WordCap)
	fmt.Fprintf(out, "  sections: invariants=%d glossary=%d decisions=%d\n", d.Invariants, d.Glossary, d.Decisions)
	if d.Hash != "" {
		fmt.Fprintf(out, "  hash: %s\n", d.Hash)
	}
}

// constitutionLockData is the lock's payload: the record just written and
// the digest of what it signed.
type constitutionLockData struct {
	Path   string                  `json:"path"`
	Relock bool                    `json:"relocked"`
	Record constitution.LockRecord `json:"record"`
	Digest constitution.Digest     `json:"digest"`
	// CommentDigestsAdopted names the claims whose comment blocks this lock
	// took digest coverage of, on the one run where that is honest: a fresh
	// project crossing into the ledger. Silent in warnings, exactly as
	// check's own crossing is — see prepareStore's digestStoreExisted.
	CommentDigestsAdopted []string `json:"comment_digests_adopted,omitempty"`
}

func newConstitutionLockCmd() *cobra.Command {
	var reason string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "lock",
		Short: "Lock the constitution: record its content hash and the human's --reason in the lock store",
		Long: "Lock the project-root constitution.yaml. The file's status becomes locked and\n" +
			"the lock store records the content hash and the human's --reason, so a hand\n" +
			"edit afterwards is detectable: check and claim lock then refuse\n" +
			"CONSTITUTION_NOT_LOCKED until a human locks it again. Re-locking an edited\n" +
			"roof is exactly what this command is for; a roof that is locked AND unchanged\n" +
			"is refused as already_locked. Refuses CONSTITUTION_OVER_CAP over 800 words.",
		Args: cobra.NoArgs,
		RunE: envelopeRunE(func(cmd *cobra.Command, _ []string) (cmdResult, error) {
			cfg, err := loadConfig()
			if err != nil {
				return cmdResult{}, err
			}
			f, err := constitution.Load(cfg.ConstitutionPath())
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeInvalidConfig, "constitution lock: %w", err).
					WithHint("write constitution.yaml beside project.config.yaml: status: draft, then invariants / glossary / decisions, each entry a slug, an optional title and a body")
			}
			current := constitutionVerdict(cfg)
			if constitution.OverCap(f) {
				return cmdResult{}, cliout.Errorf(cliout.CodeConstitutionOverCap,
					"constitution lock: refused, %d of %d words", constitution.WordCount(f), constitution.WordCap).
					WithDetails(verdictDetails(current)).
					WithHint("trim constitution.yaml under the cap, then lock again")
			}
			if dryRun {
				dr := cliout.NewDryRun("lock constitution "+cfg.ConstitutionPath()).
					Transition(string(f.Status), string(model.StatusLocked))
				if strings.TrimSpace(reason) == "" {
					dr.Lacking("--reason")
				}
				dr.Require("not_already_locked", current.State != constitution.StateLocked,
					current.Detail())
				dr.Require("under_word_cap", true, fmt.Sprintf("%d of %d words", current.Words, current.WordCap))
				storesArePrecondition(dr, cfg)
				dr.Effect("constitution.yaml is rewritten with status: locked")
				dr.Effect("the lock store records the file's content hash and your --reason; every later edit is detectable and stops module work until a human re-locks")
				dr.Propose("reason", reason)
				return dryRunResult(cmd, "constitution lock", dr), nil
			}
			if err := requireReason("constitution lock", reason); err != nil {
				return cmdResult{}, err
			}
			// The same carrier guard every approval write enforces: a record
			// under a gitignored store never reaches collaborators.
			gitignoreWarnings, err := refuseIfStoresGitignored(cfg, "constitution lock")
			if err != nil {
				return cmdResult{}, err
			}
			release, err := lock.AcquireFileLock(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "constitution lock: %w", err)
			}
			defer release()
			store, err := lock.LoadStore(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "constitution lock: %w", err)
			}
			verdict := constitution.Evaluate(cfg.ConstitutionPath(), f, nil, store.Constitution)
			if verdict.State == constitution.StateLocked {
				// Locked and unchanged: a second lock would stamp a fresh
				// approval over content nobody re-approved. An EDITED roof
				// (hash moved) is not this case — re-locking it is the point.
				return cmdResult{}, cliout.Errorf(cliout.CodeAlreadyLocked,
					"constitution lock: already locked and unchanged since %s", verdict.LockedAt).
					WithDetails(verdictDetails(verdict)).
					WithHint("edit constitution.yaml first; a lock signs a change, and there is none")
			}
			relock := verdict.State == constitution.StateEdited
			// A FRESH project — no lock store yet — is crossing into the
			// ledger with this write, exactly as its first `check` used to.
			// The crossing adopts the comment threads on disk (there is
			// nothing they could have drifted from) before Save creates the
			// digest store empty; without this, every thread a new or
			// upgrading project already carries would read as unrecorded
			// the moment its roof locked. Best-effort on the claims: a
			// project whose claims do not load has nothing to adopt and is
			// refused at load by every other verb anyway.
			var adopted []string
			if !store.LedgerCovered() && !store.PreLedger() {
				if claims, loadErr := loader.LoadAll(cfg); loadErr == nil {
					adopted, _ = lock.SweepCommentDigests(store, claims, false)
				}
			}
			f.Status = model.StatusLocked
			raw, err := constitution.Marshal(f)
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "constitution lock: %w", err)
			}
			if err := os.WriteFile(f.SourcePath, raw, 0o644); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "constitution lock: %w", err)
			}
			rec := lock.LockConstitution(store, f, reason, time.Now())
			if err := store.Save(); err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteFailed, "constitution lock: %w", err)
			}
			d := constitution.NewDigest(cfg.ConstitutionPath(), f)
			data := constitutionLockData{Path: cfg.ConstitutionPath(), Relock: relock, Record: rec, Digest: d, CommentDigestsAdopted: adopted}
			return cmdResult{
				Warnings: gitignoreWarnings,
				Data:     data,
				Text: func() {
					verb := "locked"
					if relock {
						verb = "re-locked"
					}
					fmt.Fprintf(cmd.OutOrStdout(), "constitution lock: %s %s (%d of %d words, hash %s)\n", verb, d.Path, d.Words, d.WordCap, rec.Hash)
				},
			}, nil
		}),
	}
	cmd.Flags().StringVar(&reason, "reason", "", "the human's approving words (required; recorded in the lock store)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report what locking would do, and write nothing")
	return cmd
}
