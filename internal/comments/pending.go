package comments

import (
	"github.com/BarterX-Tech/dossierx/internal/lock"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/reaudit"
)

// PendingTriggers reports the three independent reasons a claim's
// review_pending could be set, evaluated against the live claim set and the
// two stores. It is the single authority the comment ops, reaudit, and the
// check/serve reconciler all consult so the three triggers can never diverge.
//
//   - drift: a dependency (lock.BaselineDependencyIDs — c.Mirrors ∪ c.RestsOn ∪
//     a claim-valued c.Governed.Type) whose recorded per-dependent baseline no
//     longer matches its current content hash. This is computed EXACTLY like
//     lock.DetectStale — via ls.Baseline(c.ID, dep) compared to
//     lock.ContentHash(depClaim) — and deliberately NOT via ls.Hashes[dep],
//     which does not exist as a flat map (Store.Hashes is per-dependent).
//   - flag: a pending `dossierx claim flag` entry parked in fs.Flags[c.ID].
//   - openThreads: the count of c's comment threads still status: open.
//
// review_pending is meaningful only for a LOCKED claim; callers apply the
// result solely to locked claims (a draft never carries review_pending), so
// PendingTriggers itself stays a pure function of the three triggers.
func PendingTriggers(c model.Claim, claims []model.Claim, ls *lock.Store, fs *reaudit.FlagStore) (drift, flag bool, openThreads int) {
	if ls != nil {
		for _, dep := range lock.BaselineDependencyIDs(c) {
			depClaim, ok := findClaim(claims, dep)
			if !ok {
				continue
			}
			if stored, known := ls.Baseline(c.ID, dep); known && stored != lock.ContentHash(depClaim) {
				drift = true
				break
			}
		}
	}
	if fs != nil {
		_, flag = fs.Flags[c.ID]
	}
	openThreads = len(c.OpenThreadIDs())
	return drift, flag, openThreads
}

// Recompute is the whole-claim review_pending verdict: drift || flag ||
// openThreads > 0. Resolve/Delete of the last open thread therefore clears
// review_pending only when neither a drifted dependency nor a pending flag
// still stands.
func Recompute(c model.Claim, claims []model.Claim, ls *lock.Store, fs *reaudit.FlagStore) bool {
	drift, flag, open := PendingTriggers(c, claims, ls, fs)
	return drift || flag || open > 0
}

// findClaim returns the claim with id, if present.
func findClaim(claims []model.Claim, id string) (model.Claim, bool) {
	for _, c := range claims {
		if c.ID == id {
			return c, true
		}
	}
	return model.Claim{}, false
}
