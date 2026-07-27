// migrate.go is the ONE explicit door into grandfathering: "dossierx migrate
// --adopt".
//
// It exists because adoption stopped happening by itself. Until this release the
// lock ledger grandfathered a project's already-locked artifacts as a side effect
// of whatever command happened to open the lock store for writing — usually an
// ordinary "dossierx check" — on a predicate read out of the store file itself
// ("this store says it predates the ledger"). Review established that no evidence
// inside the project directory can tell an HONEST v0.2.x store from a downgraded
// one: locked_at shipped in v0.2.0, the version field lives in the audited file,
// and an attacker who deletes the comment digest store alongside the ledger
// produces a directory byte-for-byte the shape of a legitimate pre-ledger one.
// The answer is not a cleverer predicate. The answer is that adoption FAILS
// CLOSED — the gate reports lock-ledger-adoption-required and refuses — and the
// only code path in the binary that writes a grandfathered record is one a human
// runs deliberately. See lock.AdoptProject, which is that path; this file is the
// door onto it.
//
// Everything about the command's shape follows from what adoption IS: recording
// content nobody approved as the baseline every later change is judged against.
//
//   - it refuses to run silently: --adopt is mandatory, and a bare "dossierx
//     migrate" is a missing_flag refusal, not a helpful default;
//   - it previews, like every other mutating verb: --dry-run names every artifact
//     it would grandfather and writes nothing;
//   - every record it writes carries grandfathered: true, permanently, so the
//     ledger never claims to have witnessed an approval it did not;
//   - it is one-time. A project already covered by the ledger is REFUSED
//     (already_migrated), because a migration that can be re-run is a laundering
//     command: delete one record, re-migrate, and the edit it covered is
//     "approved" again. Refusing rather than exiting 0 is the same call
//     already_locked makes for the same shape — "there is nothing to do" is a
//     refusal, so the caller learns its model of the world was wrong.
//
// WHAT IT WILL NOT DO, and this is a deliberate refusal rather than a gap: it
// does not adopt a project with NO lock store. An absent store is
// indistinguishable from a deleted one, so adopting there would make `rm
// .dossierx-lock-store.json` the first half of a two-command bypass and this
// command the second. internal/lock enforces that (ErrAdoptionRefused); this file
// reports it with the one recovery that is correct, which is version control.
package main

import (
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/BarterX-Tech/dossierx/internal/buildorder"
	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/digest"
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// The two codes this file emits live in internal/cliout/codes.go with every
// other code in the vocabulary: cliout.CodeAlreadyMigrated ("already_migrated")
// and cliout.CodeAdoptionRequired ("adoption_required"). Their WHY is written up
// there, beside the codes they are argued from — CodeAlreadyLocked for the first,
// CodeUntrackedConfig for the second. They are named here only so a reader
// arriving at this file knows the migration surface has exactly two of them.

// requireMigratedProject refuses any command that is about to write an APPROVAL
// RECORD into a lock store that predates the ledger.
//
// It is cmd/dossierx's half of the guard lock.Lock enforces for claim locking,
// and it exists because two other paths in this package reach the ledger without
// going through lock.Lock: "build-order lock" (lock.RecordBuildOrderApproval) and
// "claim reaudit --confirm" (lock.RecordApproval). Left unguarded, either one
// writes the FIRST record into a store whose version field says the ledger does
// not exist — and Store.Save stamps the current version as it writes, so the
// result is a project that is suddenly ledger-covered with exactly one record and
// every other locked claim in it now reading as lock-ledger-deleted. An ordinary
// approval would have converted an honest upgrade state into a project-wide
// accusation of tampering, which is a worse outcome than the one the fail-closed
// decision was taken to avoid.
//
// The predicate is Store.AdoptionRequired, the same one the gate and
// lock.AdoptProject use, so all four answer identically for a given project.
//
// verb is the command path, used to compose a message that reads as that
// command's own refusal rather than as a stray internal error.
func requireMigratedProject(cfg *config.Config, store *lock.Store, verb string) error {
	if store == nil || !store.AdoptionRequired(digestStorePresent(cfg)) {
		return nil
	}
	return cliout.Errorf(cliout.CodeAdoptionRequired,
		"%s: this project's lock store predates the lock ledger, so recording an approval here would write the first record into a store that says it has none — a state nothing downstream can tell apart from a tampered one. Nothing is grandfathered automatically any more", verb).
		WithHint("run: dossierx migrate --adopt --dry-run (it names every already-locked artifact it would grandfather), then dossierx migrate --adopt ONCE, commit the lock store and the comment digest store it writes, and run this command again")
}

// migratedPrecondition adds the un-migrated gate to a PREVIEW, for the three
// verbs whose write path records an approval: claim lock, claim reaudit
// --confirm, and build-order lock.
//
// It exists for the rule Task 3 restates every round: a --dry-run must agree with
// its write path. When adoption became fail-closed, those three write paths grew
// a refusal (lock.ErrAdoptionRequired, and requireMigratedProject for the two
// that do not go through lock.Lock) and their previews did not — so on every
// un-migrated project the preview said "would lock" and the run refused. That is
// the more damaging direction of disagreement: the agent takes the preview to its
// human, gets a yes, and then cannot deliver it.
//
// A store that will not LOAD is left alone rather than reported as un-migrated:
// unreadability is a different finding with a different recovery (restore from
// version control), the gate already names it, and a preview must not
// manufacture a second, wrong diagnosis out of it.
func migratedPrecondition(dr *cliout.DryRun, cfg *config.Config) {
	store, err := lock.LoadStore(storePath(cfg))
	if err != nil {
		return
	}
	migrated := !store.AdoptionRequired(digestStorePresent(cfg))
	dr.Require("project_migrated", migrated, boolDetail(migrated,
		"this project's lock store is at the current ledger schema, so an approval can be recorded",
		"this project's lock store predates the lock ledger and nothing is grandfathered automatically: run dossierx migrate --adopt ONCE first (dossierx migrate --adopt --dry-run names what it would grandfather)"))
}

// The migration modes: the stable, snake_case answer to "what kind of project is
// this?". They ride in data.mode on success AND on every refusal, because a
// caller that receives already_migrated or integrity_failed needs to know which
// state produced it — the recoveries differ, and two of them are destructive if
// applied to the wrong one.
//
// They are computed by planMigration from exactly the predicates
// lock.AdoptProject evaluates, in the order it evaluates them, which is what
// makes --dry-run's verdict and the write path's verdict the same verdict rather
// than two implementations that agree today.
const (
	// migrateModePreLedgerStore: a lock store on disk at a schema version that
	// predates the ledger, with nothing contradicting that. The ordinary v0.2.x
	// upgrade, and the one mode that adopts.
	migrateModePreLedgerStore = "pre_ledger_store"

	// migrateModeAlreadyCovered: the store is already at the ledger schema.
	// Refused: adoption is one-time.
	migrateModeAlreadyCovered = "already_covered"

	// migrateModeNoStore: there is no lock store, and locked artifacts exist.
	// This is the deleted-ledger shape, and it is refused — see this file's doc
	// comment, and lock.AdoptProject's ErrAdoptionRefused branch. It is
	// deliberately NOT migrateModeNothingToAdopt: "there is nothing to do here"
	// is the wrong thing to tell someone whose approval records have vanished.
	migrateModeNoStore = "no_lock_store"

	// migrateModeNothingToAdopt: no lock store AND nothing locked, so there is
	// no pre-ledger state at all. Refused, and specifically WITHOUT creating a
	// store: a store file conjured for a project that has never locked anything
	// would destroy the evidence lock-ledger-absent is built from, which is the
	// same reason lock.PrepareStore does not create one either.
	migrateModeNothingToAdopt = "nothing_to_adopt"

	// migrateModeDowngraded: the store claims to predate the ledger and the
	// project around it proves otherwise (lock.Store.LedgerDowngraded). Refused,
	// and the recovery is version control — never re-locking, which would record
	// whatever the claims say now as approved.
	migrateModeDowngraded = "downgraded"
)

// migrateData is "dossierx migrate"'s machine payload.
//
// Grandfathered is a constant true and is in the envelope on purpose: it is the
// machine-readable form of this command's central honesty claim — every record it
// wrote is marked an adoption, not an approval — and a consumer that has to infer
// that from prose has to trust the prose.
type migrateData struct {
	Mode string `json:"mode"`

	// Adopted is every ledger key this run wrote: claim ids, and
	// "build-order:<module>" keys, sorted. One list rather than two for the same
	// reason checkData.LedgerAdopted is one: they are the same kind of fact (a
	// locked artifact that now holds a grandfathered record), and the key's own
	// shape says which kind it is.
	Adopted []string `json:"adopted"`

	// CommentDigestsAdopted is the claims whose comment block this run took
	// digest coverage of. Kept separate from Adopted, never merged into it,
	// because the recoveries differ: a grandfathered lock can be re-locked, an
	// adopted comment digest cannot be cleared by any verb in this binary. See
	// adoptionWarnings, which renders both.
	CommentDigestsAdopted []string `json:"comment_digests_adopted"`

	Grandfathered bool `json:"grandfathered"`

	LockStorePath          string `json:"lock_store_path"`
	CommentDigestStorePath string `json:"comment_digest_store_path"`
}

// buildOrderAdoption is one module's locked build-order artifact awaiting a
// record, with the signature that record will carry. The hash is computed during
// PLANNING rather than at apply time so the dry run reports the same work the
// write path performs, down to the artifact it read.
type buildOrderAdoption struct {
	Module string
	Hash   string
}

// migrationPlan is the whole decision, computed from loaded state, writing
// nothing. Both --dry-run and the write path go through it, and that is the
// mechanism by which the preview and the real run cannot disagree: the preview
// RENDERS a plan, the write path APPLIES the same plan, and neither re-decides
// anything on its own.
type migrationPlan struct {
	Mode string

	// Claims is the locked claim ids with no ledger record — what
	// lock.AdoptProject will grandfather.
	Claims []string

	// BuildOrders is the same question asked of each module's locked
	// build-order artifact. It is answered HERE rather than inside
	// lock.AdoptProject because it cannot be answered there: internal/buildorder
	// imports internal/lock, so lock cannot read an artifact back. It used to be
	// answered inside prepareStore, on every ordinary command — which was the
	// last auto-adoption left in the binary after ledger adoption became
	// explicit, and is now this command's job alone.
	BuildOrders []buildOrderAdoption

	// Digests is the claim ids whose comment block has no digest entry yet. A
	// claim holding a standing ledger record with no digest entry is a
	// comment-digest-missing finding, so lock.AdoptProject records the comment
	// block of every claim as part of the same act; this is that list, computed
	// ahead of it so the preview can print it.
	Digests []string

	// RecordCount is how many records the store already holds — the number the
	// already_covered refusal quotes back.
	RecordCount int

	// LegacyBaselines is true for a store that also predates PER-DEPENDENT
	// dependency baselines (on-disk schema 0). See migrateLegacyBaselines for
	// why this command has to care about a migration that is otherwise none of
	// its business.
	LegacyBaselines bool

	LockStorePath          string
	CommentDigestStorePath string
}

// adopts reports whether this plan is one the write path may execute. Exactly
// one mode does.
func (p migrationPlan) adopts() bool { return p.Mode == migrateModePreLedgerStore }

// ledgerKeys is every key this plan would write, sorted — claim ids and
// "build-order:<module>" keys together, which is the form both the envelope and
// adoptionWarnings take.
func (p migrationPlan) ledgerKeys() []string {
	keys := make([]string, 0, len(p.Claims)+len(p.BuildOrders))
	keys = append(keys, p.Claims...)
	for _, bo := range p.BuildOrders {
		keys = append(keys, lock.BuildOrderLedgerKey(bo.Module))
	}
	sort.Strings(keys)
	return keys
}

// planMigration decides what a migration of this project would do, from state
// the caller has already loaded, touching nothing.
//
// THE FOUR-WAY SWITCH IS lock.AdoptProject's PRECONDITIONS, in its order. That
// is not a coincidence to be maintained by hand — it is the whole reason this
// function exists rather than the dry run asking its own questions.
// TestMigrateDryRunAgreesWithTheWritePath drives both paths over the same
// fixtures and fails if a preview ever says "would adopt" where the real run
// refuses, or the reverse.
//
// Read the order itself as the argument: "covered" is asked before "is there
// anything to adopt", so a covered project with a record DELETED out of it is
// refused as already_migrated (recovery: version control) rather than reported as
// an ordinary migration with one artifact in it. That is the difference between a
// migration and a laundering command.
func planMigration(cfg *config.Config, store *lock.Store, digests *digest.Store, claims []model.Claim) migrationPlan {
	plan := migrationPlan{
		RecordCount:            len(store.Ledger),
		LockStorePath:          storePath(cfg),
		CommentDigestStorePath: digest.StorePath(cfg),
	}

	switch {
	case !store.FileExists():
		// Two different states share "no store", and they must not share a
		// refusal: a project with locked artifacts and no ledger has LOST its
		// records (recovery: version control), while a project with neither has
		// simply never approved anything (recovery: approve something).
		plan.Mode = migrateModeNoStore
		if !anythingLocked(cfg, claims) {
			plan.Mode = migrateModeNothingToAdopt
		}
		return plan
	case store.LedgerCovered():
		plan.Mode = migrateModeAlreadyCovered
		return plan
	case store.LedgerDowngraded(digests.FileExists()):
		plan.Mode = migrateModeDowngraded
		return plan
	default:
		plan.Mode = migrateModePreLedgerStore
	}

	// A store at on-disk schema 0 predates PER-DEPENDENT dependency baselines as
	// well as the ledger. Versions are 0, 1, 2 and nothing else, so "== 0" is
	// exactly lock's own `diskVersion < nestedHashSchemaVersion`, which is not
	// exported. See migrateLegacyBaselines.
	plan.LegacyBaselines = store.OnDiskVersion() == 0

	for _, c := range claims {
		if c.Status != model.StatusLocked {
			continue
		}
		if _, exists := store.Record(c.ID); exists {
			continue
		}
		plan.Claims = append(plan.Claims, c.ID)
	}
	sort.Strings(plan.Claims)

	for _, module := range cfg.Modules {
		artifact, err := buildorder.LoadArtifact(buildorder.ArtifactPath(cfg, module))
		if err != nil || artifact == nil || !artifact.Locked {
			continue
		}
		if _, exists := store.Record(lock.BuildOrderLedgerKey(module)); exists {
			continue
		}
		hash, err := buildOrderSignature(artifact)
		if err != nil {
			// An artifact that will not marshal cannot be signed, so it cannot
			// be adopted. Skipping it leaves the gate to report it as
			// build-order-ledger-missing, which is the honest outcome for an
			// artifact nothing can describe.
			continue
		}
		plan.BuildOrders = append(plan.BuildOrders, buildOrderAdoption{Module: module, Hash: hash})
	}

	// EVERY claim without a digest entry, not only the ones being grandfathered
	// — the same set lock.AdoptProject's adoptCommentDigests records, and it has
	// to be: a claim that holds a standing approval with no digest entry is a
	// comment-digest-missing finding, so a migration that covered only the locked
	// claims would trade one gate failure for another.
	for _, c := range claims {
		if _, ok := digests.Digest(c.ID); ok {
			continue
		}
		plan.Digests = append(plan.Digests, c.ID)
	}
	sort.Strings(plan.Digests)
	return plan
}

// anythingLocked reports whether this project holds any locked artifact at all —
// a locked claim, or a module whose build-order artifact says locked. It is what
// separates "your ledger is missing" from "you have never approved anything",
// which are the same shape on disk and have opposite recoveries.
func anythingLocked(cfg *config.Config, claims []model.Claim) bool {
	for _, c := range claims {
		if c.Status == model.StatusLocked {
			return true
		}
	}
	for _, module := range cfg.Modules {
		artifact, err := buildorder.LoadArtifact(buildorder.ArtifactPath(cfg, module))
		if err == nil && artifact != nil && artifact.Locked {
			return true
		}
	}
	return false
}

// migrateDryRun renders a plan as the preview an agent shows its human before
// asking for a yes.
//
// The preconditions are the questions the write path actually asks, in the order
// it asks them, so "blocked" here means "refused there" and nothing else.
// adopt_flag_given is among them because a preview of a command that would refuse
// for want of a flag has to say so — the alternative (refusing the dry run
// itself) is the one shape a dry run may never take, since a caller cannot tell a
// refusal to preview apart from a preview of a refusal. See
// TestDryRun_MissingReasonIsReportedNotRefused, which pins the same rule for lock.
func migrateDryRun(plan migrationPlan, adopt bool) *cliout.DryRun {
	keys := plan.ledgerKeys()
	dr := cliout.NewDryRun(fmt.Sprintf(
		"adopt %d already-locked artifact(s) into the lock ledger as GRANDFATHERED — recorded as-found, never approved by anyone", len(keys)))
	dr.Transition(plan.Mode, "ledger_covered")

	if !adopt {
		dr.Lacking("--adopt")
	}
	dr.Require("adopt_flag_given", adopt, boolDetail(adopt,
		"--adopt was given, so the real run would proceed",
		"--adopt is required: this command records content nobody approved as the approved baseline, so it never runs by default"))
	dr.Require("lock_store_exists",
		plan.Mode != migrateModeNoStore,
		boolDetail(plan.Mode != migrateModeNoStore,
			"the lock store is on disk, so there is a pre-ledger state to migrate",
			"there is no lock store while locked artifacts exist: an absent ledger is never adopted, because absence is indistinguishable from deletion — restore it from version control"))
	dr.Require("something_to_adopt",
		plan.Mode != migrateModeNothingToAdopt,
		boolDetail(plan.Mode != migrateModeNothingToAdopt,
			"this project has locked artifacts with no approval record",
			"this project has no lock store and nothing locked, so there is no pre-ledger state to grandfather"))
	dr.Require("not_already_migrated",
		plan.Mode != migrateModeAlreadyCovered,
		boolDetail(plan.Mode != migrateModeAlreadyCovered,
			"this project has never been through a ledger-aware build",
			fmt.Sprintf("this project already holds %d lock-ledger record(s); adoption is one-time, and re-running it would record whatever is on disk now as approved", plan.RecordCount)))
	dr.Require("pre_ledger_claim_not_contradicted",
		plan.Mode != migrateModeDowngraded,
		boolDetail(plan.Mode != migrateModeDowngraded,
			"nothing in this project contradicts the store's pre-ledger version",
			"the store says it predates the ledger and the project around it says otherwise (its comment digest store exists, or the store still carries the ledger key) — restore the lock store from version control"))

	if plan.LegacyBaselines {
		dr.Effect("re-arms per-dependent dependency baselines from the content on disk NOW (this store predates them too): drift that happened before this run is adopted as the new baseline and will not be reported; drift after it will be")
	}
	dr.Effect("writes " + plan.LockStorePath + ": one grandfathered record per locked artifact, and the current ledger schema version")
	dr.Effect(fmt.Sprintf("writes %s: comment-block coverage for %d claim(s), adopted as-found", plan.CommentDigestStorePath, len(plan.Digests)))
	dr.Effect("from this run on, every change to a locked claim is judged against the content adopted HERE — review the list below, and re-lock anything you are not sure of (dossierx claim unlock <id> --reason \"...\" then dossierx claim lock <id> --reason \"...\")")

	dr.Propose("mode", plan.Mode)
	dr.Propose("adopted", keys)
	dr.Propose("comment_digests_adopted", plan.Digests)
	dr.Propose("grandfathered", true)
	return dr
}

// migrateRefusal turns a non-adopting plan into the refusal the write path
// returns, with the recovery for that exact state.
//
// Every hint names commands this binary actually has. That is not a style note:
// a prior round shipped a refusal pointing at a verb the CLI does not implement,
// and a reader who is wedged and following it got "unknown command" at the moment
// they most needed a way out. TestMigrateRefusalsNameOnlyRealCommands pins it.
//
// The four recoveries are deliberately different, and two of them are the
// OPPOSITE of the obvious move: for a covered project and for a downgraded one,
// the answer is to restore the lock store from version control, never to re-lock
// — re-locking records whatever the claims say NOW, which is precisely what the
// edit that produced the state was for.
//
// Two of the four share cliout.CodeAlreadyMigrated (already_covered and
// nothing_to_adopt) because they are the same answer to the caller's question —
// there is nothing here to migrate, do not loop — and data.mode is what tells
// them apart. The other two are integrity_failed: something is missing or
// contradicted, and the recovery is version control rather than a command.
func migrateRefusal(plan migrationPlan) error {
	switch plan.Mode {
	case migrateModeAlreadyCovered:
		return cliout.Errorf(cliout.CodeAlreadyMigrated,
			"migrate: this project is already covered by the lock ledger (%s holds %d record(s)); adoption is a one-time upgrade step, not a repair, and running it again would record whatever is on disk NOW as approved",
			plan.LockStorePath, plan.RecordCount).
			WithHint(fmt.Sprintf("there is nothing to migrate. Run: dossierx check --validate — a locked claim with no record in a covered project is a finding (lock-ledger-missing / lock-ledger-deleted), and its recovery is restoring %s from version control; if a claim genuinely needs a fresh approval, that is dossierx claim unlock <id> --reason \"...\" then dossierx claim lock <id> --reason \"...\"", plan.LockStorePath))
	case migrateModeNothingToAdopt:
		return cliout.Errorf(cliout.CodeAlreadyMigrated,
			"migrate: nothing to adopt: this project has no lock store and no locked claim or build order, so there is no pre-ledger state to grandfather").
			WithHint("there is nothing to migrate: the ledger is created by the first approval. Run: dossierx claim lock <id> --reason \"<the approving words>\"")
	case migrateModeNoStore:
		return cliout.Errorf(cliout.CodeIntegrityFailed,
			"migrate: %s does not exist, and an absent lock ledger is never adopted: a missing store is indistinguishable from a deleted one, so adopting here would make deleting the file the way to re-bless every locked claim as-found",
			plan.LockStorePath).
			WithHint(fmt.Sprintf("if this project has locked claims, restore %s from version control and run dossierx migrate --adopt --dry-run again; if it has none, there is nothing to migrate — the first dossierx claim lock <id> --reason \"...\" creates the store with a real approval record in it", plan.LockStorePath))
	case migrateModeDowngraded:
		return cliout.Errorf(cliout.CodeIntegrityFailed,
			"migrate: %s says it predates the lock ledger, but this project has already been through a ledger-aware build (its comment digest store is present, or the store still carries the ledger key). Nothing was adopted: a store's own version field must not be able to re-arm adoption, or editing one number would re-bless every locked claim as-found",
			plan.LockStorePath).
			WithHint(fmt.Sprintf("restore %s from version control — do NOT re-lock, which would record whatever the claims say now as approved. Then: dossierx check --validate", plan.LockStorePath))
	default:
		// Unreachable while adopts() and this switch describe the same set; kept
		// because a fifth mode added without a recovery must not silently become
		// a successful migration.
		return cliout.Errorf(cliout.CodeInternal,
			"migrate: refused in mode %q, which has no recovery defined", plan.Mode).
			WithHint("run: dossierx check --validate and report this — a mode with no recovery is a bug in dossierx, not a state of your project")
	}
}

// migrateAdoptionError classifies a refusal that came back from
// lock.AdoptProject itself rather than from the plan.
//
// It exists as the SECOND line of the same defence, not as the first: the plan
// evaluates the same preconditions and refuses before this is ever reached, so
// arriving here means the project changed between the plan and the call, or that
// the two disagree — and the second of those must produce a coded envelope rather
// than an unclassified "internal" that tells the agent to file a bug about its own
// project's state.
func migrateAdoptionError(plan migrationPlan, err error) error {
	switch {
	case errors.Is(err, lock.ErrAdoptionNotRequired):
		return cliout.Wrap(err, cliout.CodeAlreadyMigrated).
			WithHint("there is nothing to migrate; run: dossierx check --validate")
	case errors.Is(err, lock.ErrAdoptionRefused):
		return cliout.Wrap(err, cliout.CodeIntegrityFailed).
			WithHint(fmt.Sprintf("restore %s from version control — do NOT re-lock, which would record whatever the claims say now as approved. Then: dossierx check --validate", plan.LockStorePath))
	default:
		// A write failure. lock.AdoptProject undoes the half that landed, so the
		// project is exactly as it was and the command can simply be re-run.
		return cliout.Wrap(err, cliout.CodeWriteFailed).
			WithHint("nothing was adopted — the migration undoes a partial write — so fix the write failure and run: dossierx migrate --adopt")
	}
}

// newMigrateCmd builds "dossierx migrate". It is a top-level NOUN with no
// subcommands: the seventh, and the surface's twentieth leaf.
//
// It is not "claim migrate" and not "check --migrate", and both were considered.
// It is not a claim operation — it writes no claim file, and it covers build
// orders as well as claims. And a flag on check would put a one-time,
// irreversible, content-blessing write behind the one command a pre-commit hook
// and CI run on every single commit, which is the exact shape the fail-closed
// decision exists to remove.
func newMigrateCmd() *cobra.Command {
	var adopt bool
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "One-time: adopt this project's already-locked artifacts into the lock ledger as grandfathered (requires --adopt)",
		Args:  cobra.NoArgs,
		RunE: envelopeRunE(func(cmd *cobra.Command, args []string) (cmdResult, error) {
			cfg, claims, err := loadConfigAndClaims()
			if err != nil {
				return cmdResult{}, err
			}

			// --dry-run answers a question, takes no sentinel and writes
			// nothing, exactly like every other mutating verb here: it runs off
			// a plain read, before the write path below is entered at all.
			if dryRun {
				store, digests, err := migrateStores(cfg)
				if err != nil {
					return cmdResult{}, err
				}
				plan := planMigration(cfg, store, digests, claims)
				return dryRunResult(cmd, "migrate", migrateDryRun(plan, adopt)), nil
			}

			if !adopt {
				return cmdResult{}, cliout.Errorf(cliout.CodeMissingFlag,
					"migrate: --adopt is required; this command records already-locked content as the approved baseline when nobody has approved it, so it never runs by default").
					WithHint("run: dossierx migrate --adopt --dry-run first — it names every artifact it would grandfather — then dossierx migrate --adopt")
			}

			// The project's global sentinel order, unchanged: claims first, then
			// the lock store. The claims sentinel is taken even though no claim
			// file is written, because the plan is computed FROM claim files and
			// applied to a store — a lock landing between the two would leave
			// this run adopting a claim whose approved content it never read.
			//
			// The COMMENT DIGEST sentinel is deliberately NOT taken here:
			// lock.AdoptProject acquires it itself, underneath these two, which
			// is the same ordering every other digest write in that package uses.
			releaseClaims, err := lock.AcquireFileLock(claimsSentinelPath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "migrate: %w", err)
			}
			defer releaseClaims()

			claims, err = loadClaims(cfg)
			if err != nil {
				return cmdResult{}, err
			}

			releaseStore, err := lock.AcquireFileLock(storePath(cfg))
			if err != nil {
				return cmdResult{}, cliout.Errorf(cliout.CodeWriteConflict, "migrate: %w", err)
			}
			defer releaseStore()

			store, digests, err := migrateStores(cfg)
			if err != nil {
				return cmdResult{}, err
			}
			plan := planMigration(cfg, store, digests, claims)
			data := migrateData{
				Mode:                   plan.Mode,
				Adopted:                []string{},
				CommentDigestsAdopted:  []string{},
				Grandfathered:          true,
				LockStorePath:          plan.LockStorePath,
				CommentDigestStorePath: plan.CommentDigestStorePath,
			}
			if !plan.adopts() {
				// The refusal still carries data.mode: an agent branching on
				// already_migrated or integrity_failed needs to know WHICH state
				// it hit, because the recoveries differ.
				return cmdResult{Data: data}, migrateRefusal(plan)
			}

			migrateLegacyBaselines(store, claims)

			adoption, err := lock.AdoptProject(store, claims)
			if err != nil {
				return cmdResult{Data: data}, migrateAdoptionError(plan, err)
			}
			data.Adopted = append(data.Adopted, adoption.Claims...)
			data.CommentDigestsAdopted = append(data.CommentDigestsAdopted, adoption.CommentDigests...)

			// The BUILD-ORDER half of the same adoption, which lock.AdoptProject
			// structurally cannot do (internal/buildorder imports internal/lock,
			// so lock cannot read an artifact back) and which therefore has to
			// happen here — after it, never before: AdoptProject refuses a store
			// whose ledger map is already non-empty (that is one of the two
			// pieces of evidence lock.Store.LedgerDowngraded reads), so writing a
			// build-order record first would make the store look downgraded to
			// the very call about to adopt it.
			//
			// The second Save is the cost of that ordering, and its failure mode
			// is stated in the hint rather than hidden: the claim adoption has
			// landed by then, so re-running the migration correctly refuses, and
			// the recovery for a build order left without a record is to lock it
			// again — which is a real approval, not an adoption, and therefore
			// strictly better than what this would have written.
			if len(plan.BuildOrders) > 0 {
				for _, bo := range plan.BuildOrders {
					lock.AdoptBuildOrderApproval(store, bo.Module, bo.Hash)
					data.Adopted = append(data.Adopted, lock.BuildOrderLedgerKey(bo.Module))
				}
				if err := store.Save(); err != nil {
					sort.Strings(data.Adopted)
					return cmdResult{
							Data:     data,
							Warnings: adoptionWarnings(adoptions{Grandfathered: adoption.Claims, CommentDigests: adoption.CommentDigests}),
						}, cliout.Errorf(cliout.CodeWriteFailed,
							"migrate: the claims were adopted and the build-order records were not: %w", err).
							WithHint("the claim migration SUCCEEDED and this project is now covered, so dossierx migrate --adopt will correctly refuse from here on. Re-lock each affected module instead: dossierx build-order lock --module <m> --reason \"<the approving words>\" — that records a real approval rather than an adoption")
				}
			}
			sort.Strings(data.Adopted)

			return cmdResult{
				Data:     data,
				Warnings: adoptionWarnings(adoptions{Grandfathered: data.Adopted, CommentDigests: data.CommentDigestsAdopted}),
				Text:     func() { writeMigrateText(cmd, data) },
			}, nil
		}),
	}
	cmd.Flags().BoolVar(&adopt, "adopt", false, "required: adopt every already-locked artifact as GRANDFATHERED — recorded as-found, never approved")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "report exactly what would be grandfathered, and write nothing")
	return cmd
}

// migrateLegacyBaselines runs the OTHER on-load store migration — the
// per-dependent dependency-baseline re-arm — inside the migration, before
// adoption, and it has to.
//
// lock.MigrateLegacyStore refuses a store whose on-disk schema is already 1 or
// later, deliberately: an empty baselines map in a NEWER store means something
// else entirely (a claim hand-flipped to locked), and re-arming there would bless
// whatever its dependencies say now. lock.AdoptProject stamps the current schema
// version as part of adopting. So for a project on the OLDEST store format —
// schema 0, no "version" field at all — the order is forced: if the migration
// runs first and the re-arm second, the re-arm looks at a store that now says
// version 2 and correctly declines, and that project's dependency-drift detection
// stays down permanently, silently, with no command left that can restore it.
//
// Before adoption became explicit this could not happen: one `dossierx check`
// ran both, in this order, through lock.PrepareStore. Splitting adoption out of
// the ordinary commands split the pair, and this is where they are put back
// together.
//
// It is called unconditionally and guards itself (a store with baselines, or at
// schema 1+, is a no-op), and its Save is lock.AdoptProject's — the re-armed
// baselines ride out on the same write, so a failed adoption leaves neither.
func migrateLegacyBaselines(store *lock.Store, claims []model.Claim) {
	lock.MigrateLegacyStore(store, claims)
}

// migrateStores loads the two stores the migration reads, and classifies their
// load failures.
//
// A lock store that EXISTS and does not decode is refused, never migrated: the
// one thing a migration must not do is adopt over approval records it could not
// read, because then corrupting the file would be worth exactly as much as
// deleting it. integrity_failed is the code the ledger gate already reports the
// same file's unreadability under (cliout.CodeIntegrityFailed names it), so the
// two surfaces answer in one vocabulary.
func migrateStores(cfg *config.Config) (*lock.Store, *digest.Store, error) {
	store, err := lock.LoadStore(storePath(cfg))
	if err != nil {
		return nil, nil, cliout.Errorf(cliout.CodeIntegrityFailed, "migrate: %w", err).
			WithHint("restore " + storePath(cfg) + " from version control — a migration cannot adopt over records it cannot read, and re-locking would record whatever the claims say now as approved. Then: dossierx check --validate")
	}
	digests, err := digest.LoadStore(digest.StorePath(cfg))
	if err != nil {
		return nil, nil, cliout.Errorf(cliout.CodeCommentDigestUnavailable, "migrate: %w", err).
			WithHint("restore " + digest.StorePath(cfg) + " from version control, then run: dossierx migrate --adopt --dry-run")
	}
	return store, digests, nil
}

// writeMigrateText is the terminal rendering. The envelope is the contract and
// this is the courtesy — but it is the form a human sees when an agent pastes
// what it did into a chat, so it NAMES the artifacts rather than counting them,
// and it repeats what an adoption does and does not establish.
func writeMigrateText(cmd *cobra.Command, data migrateData) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "migrate --adopt: %d artifact(s) adopted into the lock ledger as GRANDFATHERED (mode: %s)\n", len(data.Adopted), data.Mode)
	for _, id := range data.Adopted {
		fmt.Fprintf(out, "  %s\n", id)
	}
	fmt.Fprintf(out, "  wrote %s\n", data.LockStorePath)
	fmt.Fprintf(out, "  wrote %s (%d claim(s) now covered)\n", data.CommentDigestStorePath, len(data.CommentDigestsAdopted))
	fmt.Fprintln(out, "  their recorded content is what was on disk just now, NOT content anyone approved")
}
