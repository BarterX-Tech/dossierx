package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// MaxIsolationBytes is the hard cap on the isolation view JSON. NIT-10
// refuses an oversize emitted view rather than truncating it.
const MaxIsolationBytes = 16384

// MaxClaimSummaryRunes is the isolation claim-summary cap. Full bodies
// are opt-in and still count against MaxIsolationBytes.
const MaxClaimSummaryRunes = 80

// ConstitutionDigestStatusPending is the NIT-6 seam. This package never
// reads constitution.yaml or project-claims; NIT-6 fills Text.
const ConstitutionDigestStatusPending = "pending_nit6"

// ConstitutionDigest is always present on manifest show. NIT-6 owns the
// roof text; this PR only reserves the field.
type ConstitutionDigest struct {
	Status string `json:"status"`
	Text   string `json:"text,omitempty"`
}

// PendingConstitutionDigest is the reserved empty roof.
func PendingConstitutionDigest() ConstitutionDigest {
	return ConstitutionDigest{Status: ConstitutionDigestStatusPending}
}

// ClaimSummary is one claim card without a body dump.
type ClaimSummary struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Facet   string `json:"facet"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
	Body    string `json:"body,omitempty"`
}

// DraftHints is the NIT-7 authoring path: draft the stub from these
// contract ids and summaries, never from pasted bodies.
type DraftHints struct {
	SuggestedProvides []string `json:"suggested_provides"`
	Note              string   `json:"note"`
}

// IsolationView is constitution digest + this manifest + claim summaries.
type IsolationView struct {
	ConstitutionDigest ConstitutionDigest `json:"constitution_digest"`
	Manifest           Manifest           `json:"manifest"`
	Claims             []ClaimSummary     `json:"claims"`
	DraftHints         DraftHints         `json:"draft_hints"`
}

// Neighbor is a summaries-only catalog row for a module this one depends on.
type Neighbor struct {
	Module  string `json:"module"`
	Summary string `json:"summary"`
	Via     string `json:"via"`
}

// ModuleEdge is a declared depends_on membership, not a catalog graph walk.
type ModuleEdge struct {
	FromModule string `json:"from_module"`
	ToModule   string `json:"to_module"`
	ViaClaim   string `json:"via_claim"`
}

// IntegrationView is the neighbor catalog and the module membership graph.
type IntegrationView struct {
	Neighbors []Neighbor   `json:"neighbors"`
	Edges     []ModuleEdge `json:"edges"`
}

// CatalogEntry is one locked-or-not module blurb from manifest.yaml.
type CatalogEntry struct {
	Module       string `json:"module"`
	Summary      string `json:"summary"`
	Provides     int    `json:"provides"`
	DependsOn    int    `json:"depends_on"`
	ClaimCount   int    `json:"claim_count"`
	LockedClaims int    `json:"locked_claims"`
	Locked       bool   `json:"locked"`
	Findings     int    `json:"findings"`
}

// ShowResult is dossierx manifest show.
type ShowResult struct {
	Module             string             `json:"module"`
	Path               string             `json:"path"`
	Summary            string             `json:"summary,omitempty"`
	Provides           []string           `json:"provides"`
	DependsOn          []string           `json:"depends_on"`
	Findings           []Finding          `json:"findings"`
	ConstitutionDigest ConstitutionDigest `json:"constitution_digest"`
	Isolation          *IsolationView     `json:"isolation,omitempty"`
	Integration        *IntegrationView   `json:"integration,omitempty"`
}

// LoadModule returns the required path's bytes if present.
func LoadModule(cfg *config.Config, module string) (raw []byte, ok bool, extras []Finding) {
	if cfg == nil {
		return nil, false, nil
	}
	tree, extraRels, err := loadTree(cfg)
	if err != nil {
		return nil, false, []Finding{{Module: module, Message: err.Error()}}
	}
	rel := RequiredRelPath(module)
	raw, ok = tree[rel]
	for _, extra := range extraRels {
		extras = append(extras, Finding{
			Module:  "",
			Message: fmt.Sprintf("%s is not a module manifest; the only legal path is claims_dir/<module>/%s", extra, FileName),
		})
	}
	return raw, ok, extras
}

// Show assembles manifest show. isolation/integration are opt-in.
// bodies is isolation-only. An isolation view over MaxIsolationBytes
// returns errIsolationOversize.
func Show(claims []model.Claim, cfg *config.Config, module string, isolation, integration, bodies bool) (ShowResult, error) {
	rel := RequiredRelPath(module)
	byID := map[string]model.Claim{}
	var moduleClaims []model.Claim
	for _, c := range claims {
		byID[c.ID] = c
		if c.Module == module {
			moduleClaims = append(moduleClaims, c)
		}
	}
	sort.Slice(moduleClaims, func(i, j int) bool { return moduleClaims[i].ID < moduleClaims[j].ID })

	// The verdict is check's verdict, filtered to this module (plus the
	// project-wide findings no module owns): a missing file, its own field
	// defects, and the cross-module export rule all come from one place, so
	// manifest show can never say green where check says red.
	var findings []Finding
	for _, f := range Check(claims, cfg) {
		if f.Module == module || f.Module == "" {
			findings = append(findings, f)
		}
	}
	var m Manifest
	if raw, ok, _ := LoadModule(cfg, module); ok {
		m, _ = decodeManifest(module, rel, raw)
	}

	out := ShowResult{
		Module:             module,
		Path:               rel,
		Summary:            strings.TrimSpace(m.Summary),
		Provides:           m.Provides,
		DependsOn:          m.DependsOn,
		Findings:           findings,
		ConstitutionDigest: PendingConstitutionDigest(),
	}
	if out.Provides == nil {
		out.Provides = []string{}
	}
	if out.DependsOn == nil {
		out.DependsOn = []string{}
	}

	if isolation {
		iso := IsolationView{
			ConstitutionDigest: PendingConstitutionDigest(),
			Manifest:           m,
			Claims:             claimSummaries(moduleClaims, bodies),
			DraftHints: DraftHints{
				SuggestedProvides: suggestedProvides(moduleClaims, module),
				Note:              "Draft summary/provides/depends_on from these claim summaries plus short neighbor/product use. Do not paste claim bodies into the manifest.",
			},
		}
		if iso.Manifest.Provides == nil {
			iso.Manifest.Provides = []string{}
		}
		if iso.Manifest.DependsOn == nil {
			iso.Manifest.DependsOn = []string{}
		}
		blob, err := json.Marshal(iso)
		if err != nil {
			return out, err
		}
		if len(blob) > MaxIsolationBytes {
			return out, errIsolationOversize{n: len(blob)}
		}
		out.Isolation = &iso
	}
	if integration {
		out.Integration = integrationView(claims, cfg, module, m)
	}
	return out, nil
}

type errIsolationOversize struct{ n int }

func (e errIsolationOversize) Error() string {
	return fmt.Sprintf("isolation view is %d bytes; must be at most %d (drop --bodies or shorten claim text)", e.n, MaxIsolationBytes)
}

// IsIsolationOversize reports a NIT-10 view-size refuse.
func IsIsolationOversize(err error) bool {
	var e errIsolationOversize
	return errors.As(err, &e)
}

// List is the locked-module catalog from manifest.yaml blurbs.
func List(claims []model.Claim, cfg *config.Config) []CatalogEntry {
	if cfg == nil {
		return nil
	}
	byMod := map[string][]model.Claim{}
	for _, c := range claims {
		byMod[c.Module] = append(byMod[c.Module], c)
	}
	byID := map[string]model.Claim{}
	for _, c := range claims {
		byID[c.ID] = c
	}
	out := make([]CatalogEntry, 0, len(cfg.Modules))
	for _, module := range cfg.Modules {
		rel := RequiredRelPath(module)
		raw, ok, _ := LoadModule(cfg, module)
		entry := CatalogEntry{Module: module}
		modClaims := byMod[module]
		entry.ClaimCount = len(modClaims)
		for _, c := range modClaims {
			if c.Status == model.StatusLocked {
				entry.LockedClaims++
			}
		}
		entry.Locked = entry.ClaimCount > 0 && entry.LockedClaims == entry.ClaimCount
		if !ok {
			entry.Findings = 1
			out = append(out, entry)
			continue
		}
		m, findings := ParseBytes(module, rel, raw, byID)
		entry.Summary = strings.TrimSpace(m.Summary)
		entry.Provides = len(m.Provides)
		entry.DependsOn = len(m.DependsOn)
		entry.Findings = len(findings)
		out = append(out, entry)
	}
	return out
}

func suggestedProvides(claims []model.Claim, module string) []string {
	var ids []string
	for _, c := range claims {
		if c.Module == module && c.Facet == ContractFacet {
			ids = append(ids, c.ID)
		}
	}
	return ids
}

// claimSummaries is the summaries-only card list. Until NIT-8 lands its
// required claim `summary` field, the summary is the body's first line
// clipped to MaxClaimSummaryRunes; once that field exists it replaces this
// derivation verbatim (one definition of "summary", not two).
func claimSummaries(claims []model.Claim, bodies bool) []ClaimSummary {
	out := make([]ClaimSummary, 0, len(claims))
	for _, c := range claims {
		s := ClaimSummary{
			ID:      c.ID,
			Title:   claimTitle(c.ID),
			Facet:   c.Facet,
			Status:  string(c.Status),
			Summary: clipRunes(firstLine(c.Body), MaxClaimSummaryRunes),
		}
		if bodies {
			s.Body = c.Body
		}
		out = append(out, s)
	}
	return out
}

func integrationView(claims []model.Claim, cfg *config.Config, module string, m Manifest) *IntegrationView {
	byID := map[string]model.Claim{}
	for _, c := range claims {
		byID[c.ID] = c
	}
	var edges []ModuleEdge
	seenNeighbor := map[string]bool{}
	var neighbors []Neighbor
	for _, id := range m.DependsOn {
		c, ok := byID[id]
		if !ok || c.Module == "" || c.Module == module {
			continue
		}
		edges = append(edges, ModuleEdge{FromModule: module, ToModule: c.Module, ViaClaim: id})
		if seenNeighbor[c.Module] {
			continue
		}
		seenNeighbor[c.Module] = true
		n := Neighbor{Module: c.Module, Via: id}
		if raw, ok, _ := LoadModule(cfg, c.Module); ok {
			nm, _ := ParseBytes(c.Module, RequiredRelPath(c.Module), raw, byID)
			n.Summary = strings.TrimSpace(nm.Summary)
		}
		neighbors = append(neighbors, n)
	}
	sort.Slice(neighbors, func(i, j int) bool { return neighbors[i].Module < neighbors[j].Module })
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].ToModule != edges[j].ToModule {
			return edges[i].ToModule < edges[j].ToModule
		}
		return edges[i].ViaClaim < edges[j].ViaClaim
	})
	if neighbors == nil {
		neighbors = []Neighbor{}
	}
	if edges == nil {
		edges = []ModuleEdge{}
	}
	return &IntegrationView{Neighbors: neighbors, Edges: edges}
}

func firstLine(body string) string {
	body = strings.TrimSpace(body)
	if i := strings.IndexByte(body, '\n'); i >= 0 {
		return strings.TrimSpace(body[:i])
	}
	return body
}

func clipRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return string(runes[:limit])
}

func claimTitle(id string) string {
	segs := strings.Split(id, ".")
	if len(segs) != 3 || segs[2] == "" {
		return id
	}
	words := strings.Split(segs[2], "-")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
