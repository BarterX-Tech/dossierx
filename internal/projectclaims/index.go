// Package projectclaims is the system-generated index over the project-claims
// store (NIT-25): one line per project.<slug> claim, id plus a one-line
// summary, and nothing anybody authors.
//
// It is the tier-1 read for project claims in `manifest show --isolation`
// and `--integration` (NIT-10 / NIT-9, PR #103): every module may rest on
// every project claim, so the store has no export boundary, no manifest and
// no cap — the index IS the curated view, and `claim show project.<slug>` is
// the body on demand. It lives in its own package so the manifest code can
// import it without pulling in the loader.
package projectclaims

import (
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// Entry is one project claim's line in the index.
type Entry struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	Status  string `json:"status"`
}

// Index returns one Entry per project claim in claims, sorted by id. Module
// claims are skipped: they have their own tier-1 read (the module
// manifest). It is total over its input — a project claim with an empty body
// still gets a line, with an empty summary, so a consumer can see that the
// claim exists and has nothing to say yet.
func Index(claims []model.Claim) []Entry {
	var out []Entry
	for _, c := range claims {
		if !c.IsProjectClaim() {
			continue
		}
		out = append(out, Entry{ID: c.ID, Summary: Summary(c), Status: string(c.Status)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Summary is the one line the index carries for a claim.
//
// NIT-8 (PR #105) adds a required `summary` field to every claim; until that
// lands on the release branch this reads the FIRST NON-BLANK LINE of the
// body instead, trimmed, which is the same thing an agent skimming the file
// would take as the gist. When `summary` exists this function is where the
// switch happens, and nothing else in the index changes shape.
func Summary(c model.Claim) string {
	for _, line := range strings.Split(c.Body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		return line
	}
	return ""
}
