package lock

import (
	"github.com/BarterX-Tech/dossierx/internal/config"
)

// ClaimsSentinelPath returns the base path of the single project-wide
// claim-file write sentinel (AcquireFileLock appends ".lock", so the file on
// disk is build/ledger/claims.lock by default). It is config.ClaimsSentinelPath:
// under the build directory — which config guarantees is outside claims_dir —
// so it is never itself decoded as a claim (LoadClaims only reads
// *.yaml/*.yml, and this file is outside claims_dir besides) and, in a later
// serve phase, never trips a claims_dir file watcher.
//
// Unlike the per-store lock-store / flag-store sentinels, each of which
// guards its own single JSON file, this sentinel guards EVERY claim file in
// the project: loader.SaveClaim rewrites a claim's whole file, so two writers
// that each loaded the same pre-mutation snapshot would have whichever saved
// last silently erase the other's change. Every claim-file writer therefore
// takes THIS sentinel first — before any lock-store or flag-store sentinel —
// then re-reads claims inside it, keeping the global acquisition order
// claims -> lock-store -> flag-store so no AB-BA deadlock is possible.
//
// cmd/dossierx computes the identical path for its own retrofitted writers
// (lock/unlock/check/flag/reaudit); this exported form is what packages that
// cannot import package main — chiefly internal/comments — use to take the
// same sentinel, so CLI comment ops serialize against every other writer.
func ClaimsSentinelPath(cfg *config.Config) string {
	return cfg.ClaimsSentinelPath()
}

// AcquireClaimsLock takes the project-wide claim-file write sentinel for cfg,
// returning a release func the caller must invoke (typically via defer) to
// release it. It is the one lock a claim-file mutation must hold across its
// load -> mutate -> SaveClaim critical section; the returned error is
// AcquireFileLock's (e.g. a timeout when another process holds the sentinel).
func AcquireClaimsLock(cfg *config.Config) (release func(), err error) {
	return AcquireFileLock(ClaimsSentinelPath(cfg))
}
