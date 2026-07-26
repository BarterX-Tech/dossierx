// Package comments is the single, shared implementation of the "comments on
// claims" mutation ops used by BOTH the `dossierx comment` CLI verbs and (in a
// later phase) the `dossierx serve` HTTP handlers, so there is exactly one
// code path — and one locking discipline — for every claim-comment change.
//
// Every mutating op takes the project-wide claims sentinel
// (lock.AcquireClaimsLock), re-reads the claims fresh INSIDE that lock,
// resolves its target thread/reply BY ID (an unknown id is a structured
// not-found error, never a positional fallback onto a neighbour), applies the
// advisory-rights rule, mints any new ids inside the critical section, writes
// exactly one claim back with loader.SaveClaim, and releases. No state is
// carried across calls.
package comments

import (
	cryptorand "crypto/rand"
	"encoding/hex"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// Id prefixes: threads are "c-", replies "r-", each followed by 6 lowercase
// hex characters (3 random bytes). This is the engine's only RNG.
const (
	threadIDPrefix = "c-"
	replyIDPrefix  = "r-"
	idRandomBytes  = 3 // 3 bytes -> 6 lowercase hex chars
)

// randRead is crypto/rand.Read, indirected through a package var so tests can
// force a deterministic byte sequence (and, in particular, an id collision to
// prove regeneration). Production always uses crypto/rand.
var randRead = cryptorand.Read

// randomHex6 returns 6 lowercase hex characters from 3 crypto-random bytes.
func randomHex6() (string, error) {
	var b [idRandomBytes]byte
	if _, err := randRead(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

// mintUniqueID returns a fresh id (prefix + 6 hex) that is not already present
// in used, recording it in used before returning so successive mints within
// one claim file never collide with each other either. On a within-file
// collision it simply regenerates — the whole reason ids are minted inside the
// claims-lock critical section, against the just-re-read claim's live id set.
func mintUniqueID(prefix string, used map[string]bool) (string, error) {
	for {
		h, err := randomHex6()
		if err != nil {
			return "", err
		}
		id := prefix + h
		if !used[id] {
			used[id] = true
			return id, nil
		}
	}
}

// collectIDs returns the set of every thread and reply id currently present on
// c (skipping empties, which backfill will fill), so new ids and backfilled
// ids are minted unique against the whole claim file.
func collectIDs(c model.Claim) map[string]bool {
	used := map[string]bool{}
	for _, cm := range c.Comments {
		if cm.ID != "" {
			used[cm.ID] = true
		}
		for _, r := range cm.Replies {
			if r.ID != "" {
				used[r.ID] = true
			}
		}
	}
	return used
}

// backfillIDs assigns an id to every id-less comment/reply on c (hand-authored
// or legacy entries that predate the id generator), mutating c in place, and
// returns the full id set now in use so a caller can mint further-unique ids.
// This is what keeps KnownFields(true) strict decode happy without forcing
// humans to hand-write ids: a load that finds an id-less entry assigns one on
// the next save.
func backfillIDs(c *model.Claim) (used map[string]bool, err error) {
	used = collectIDs(*c)
	for i := range c.Comments {
		if c.Comments[i].ID == "" {
			id, err := mintUniqueID(threadIDPrefix, used)
			if err != nil {
				return nil, err
			}
			c.Comments[i].ID = id
		}
		for j := range c.Comments[i].Replies {
			if c.Comments[i].Replies[j].ID == "" {
				id, err := mintUniqueID(replyIDPrefix, used)
				if err != nil {
					return nil, err
				}
				c.Comments[i].Replies[j].ID = id
			}
		}
	}
	return used, nil
}
