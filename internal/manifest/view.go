package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/constitution"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/projectclaims"
)

// The --isolation view is capped at MaxIsolationBytes of compact JSON, split
// in two budgets (NIT-7 Q2, decided 2026-09-25):
//
//   - SharedBudgetBytes covers the text every module reads the same way: the
//     constitution text plus the project claims index. check enforces it
//     project-wide (the shared-context-budget lint) on the project claim that
//     pushes the index over, so a module view is never refused for text the
//     module does not own.
//   - ModuleBudgetBytes covers everything else in the view: this module's
//     manifest, its claim summaries, the draft hints and the JSON framing.
//     check enforces it per module (a module-manifest finding), and Show
//     refuses an overflow with errIsolationOversize, naming the module. It
//     counts bytes, not characters: summary caps count characters, so
//     multibyte summaries can overflow it with every summary under its cap.
//
// The two add up to MaxIsolationBytes, so a view inside both budgets is
// inside the whole cap.
const (
	MaxIsolationBytes = 16384
	SharedBudgetBytes = 10240
	ModuleBudgetBytes = MaxIsolationBytes - SharedBudgetBytes
)

// ConstitutionDigest is always present on manifest show: constitution.Digest
// (which constitution the view was built against and how full it is) plus
// the lock gate's state. It is the digest, not the text; --isolation carries
// the text.
type ConstitutionDigest struct {
	Path       string `json:"path"`
	Present    bool   `json:"present"`
	Status     string `json:"status,omitempty"`
	State      string `json:"state,omitempty"`
	Words      int    `json:"words"`
	WordCap    int    `json:"word_cap"`
	OverCap    bool   `json:"over_cap"`
	NearCap    bool   `json:"near_cap"`
	Hash       string `json:"hash,omitempty"`
	Invariants int    `json:"invariants"`
	Glossary   int    `json:"glossary"`
	Decisions  int    `json:"decisions"`
}

func newConstitutionDigest(d constitution.Digest, state string) ConstitutionDigest {
	return ConstitutionDigest{
		Path: d.Path, Present: d.Present, Status: d.Status, State: state,
		Words: d.Words, WordCap: d.WordCap, OverCap: d.OverCap, NearCap: d.NearCap,
		Hash: d.Hash, Invariants: d.Invariants, Glossary: d.Glossary, Decisions: d.Decisions,
	}
}

// SharedContext is the part of the isolation view every module shares, and
// the exact bytes SharedBudgetBytes measures. The lint and the view build it
// with the same function, so check and manifest show cannot disagree on its
// size.
type SharedContext struct {
	ConstitutionText string                `json:"constitution_text"`
	ProjectClaims    []projectclaims.Entry `json:"project_claims"`
}

// ClaimSummary is one claim card: the claim's authored summary, never its
// body (bodies are read with claim show <id>).
type ClaimSummary struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Facet   string `json:"facet"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

// DraftHints is the NIT-7 authoring path: draft the stub from these
// contract ids and summaries, never from pasted bodies.
type DraftHints struct {
	SuggestedProvides []string `json:"suggested_provides"`
	Note              string   `json:"note"`
}

// IsolationView is the bounded context an agent works a module from: the
// shared context, this manifest, this module's claim summaries and the
// draft hints.
type IsolationView struct {
	Shared     SharedContext  `json:"shared"`
	Manifest   Manifest       `json:"manifest"`
	Claims     []ClaimSummary `json:"claims"`
	DraftHints DraftHints     `json:"draft_hints"`
}

// IsolationBudget reports how the isolation view spends its two budgets. It
// sits beside the view, not inside it, so reporting the size never changes
// the size.
type IsolationBudget struct {
	SharedBytes  int `json:"shared_bytes"`
	SharedBudget int `json:"shared_budget"`
	ModuleBytes  int `json:"module_bytes"`
	ModuleBudget int `json:"module_budget"`
}

// ProvidedClaim is one id a neighbor exports, with that claim's authored
// summary: what an agent reads to pick a rests_on target without opening
// the neighbor's own view. Only contract claims are ever listed.
type ProvidedClaim struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
}

// Neighbor is a summaries-only catalog row for a module this one depends on:
// its manifest summary and everything it provides, never its internals and
// never a body.
type Neighbor struct {
	Module   string          `json:"module"`
	Summary  string          `json:"summary"`
	Via      string          `json:"via"`
	Provides []ProvidedClaim `json:"provides"`
}

// ModuleEdge is a declared depends_on membership, not a catalog graph walk.
type ModuleEdge struct {
	FromModule string `json:"from_module"`
	ToModule   string `json:"to_module"`
	ViaClaim   string `json:"via_claim"`
}

// IntegrationView is the neighbor catalog, the module membership graph and
// the project claims index (the same index --isolation carries). It has no
// byte cap in v0.7.21 (NIT-7 Q4): depends_on bounds the fan-out.
type IntegrationView struct {
	Neighbors     []Neighbor            `json:"neighbors"`
	Edges         []ModuleEdge          `json:"edges"`
	ProjectClaims []projectclaims.Entry `json:"project_claims"`
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
	IsolationBudget    *IsolationBudget   `json:"isolation_budget,omitempty"`
	Integration        *IntegrationView   `json:"integration,omitempty"`
}

// ShowOptions selects manifest show's opt-in views. ConstitutionState is the
// constitution lock gate's state, which needs the lock store this package
// never reads; the caller passes it through.
type ShowOptions struct {
	Isolation         bool
	Integration       bool
	ConstitutionState string
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

// loadConstitution reads the configured constitution.yaml (the index copy
// when cfg.ConstitutionIndex is set), or nil when it is absent or unreadable (the constitution gate reports those; the views just
// carry no text).
func loadConstitution(cfg *config.Config) (string, *constitution.File) {
	if cfg == nil {
		return "", nil
	}
	path := cfg.ConstitutionPath()
	if idx := cfg.ConstitutionIndex; idx != nil {
		// check --staged: the roof the commit carries, not the working tree.
		if !idx.Tracked {
			return path, nil
		}
		f, err := constitution.Parse(idx.Raw, path)
		if err != nil {
			return path, nil
		}
		return path, f
	}
	f, err := constitution.LoadOptional(path)
	if err != nil {
		return path, nil
	}
	return path, f
}

// BuildSharedContext is the shared half of every isolation view: the
// constitution text and the project claims index.
func BuildSharedContext(claims []model.Claim, cfg *config.Config) SharedContext {
	_, f := loadConstitution(cfg)
	idx := projectclaims.Index(claims)
	if idx == nil {
		idx = []projectclaims.Entry{}
	}
	return SharedContext{ConstitutionText: constitution.Text(f), ProjectClaims: idx}
}

// SharedOverflow is where the shared context first crosses SharedBudgetBytes.
type SharedOverflow struct {
	// ClaimID is the project claim whose index line pushes the shared
	// context over the budget, in index (id) order. It is "" when the
	// constitution text alone is over.
	ClaimID string
	// Bytes is the whole shared context's size.
	Bytes int
}

// CheckSharedBudget measures sc the way the isolation view serializes it and
// reports the first index line that crosses SharedBudgetBytes, or nil when
// the shared context fits. It marshals each entry once: linear in the index.
func CheckSharedBudget(sc SharedContext) (*SharedOverflow, error) {
	base, err := json.Marshal(SharedContext{ConstitutionText: sc.ConstitutionText, ProjectClaims: []projectclaims.Entry{}})
	if err != nil {
		return nil, err
	}
	// "[]" becomes "[e1,e2,...]": each entry adds its own bytes, and every
	// entry after the first adds a comma.
	size := len(base)
	var over *SharedOverflow
	if size > SharedBudgetBytes {
		over = &SharedOverflow{}
	}
	for i, e := range sc.ProjectClaims {
		line, err := json.Marshal(e)
		if err != nil {
			return nil, err
		}
		size += len(line)
		if i > 0 {
			size++
		}
		if over == nil && size > SharedBudgetBytes {
			over = &SharedOverflow{ClaimID: e.ID}
		}
	}
	if over != nil {
		over.Bytes = size
	}
	return over, nil
}

// Show assembles manifest show. The isolation and integration views are
// opt-in. An isolation view whose module-owned part is over
// ModuleBudgetBytes returns errIsolationOversize.
func Show(claims []model.Claim, cfg *config.Config, module string, opts ShowOptions) (ShowResult, error) {
	rel := RequiredRelPath(module)
	var moduleClaims []model.Claim
	for _, c := range claims {
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

	path, f := loadConstitution(cfg)
	out := ShowResult{
		Module:             module,
		Path:               rel,
		Summary:            strings.TrimSpace(m.Summary),
		Provides:           m.Provides,
		DependsOn:          m.DependsOn,
		Findings:           findings,
		ConstitutionDigest: newConstitutionDigest(constitution.NewDigest(path, f), opts.ConstitutionState),
	}
	if out.Provides == nil {
		out.Provides = []string{}
	}
	if out.DependsOn == nil {
		out.DependsOn = []string{}
	}

	if opts.Isolation {
		iso := newIsolationView(BuildSharedContext(claims, cfg), m, moduleClaims, module)
		budget, over, err := measureModuleBudget(iso, module)
		if err != nil {
			return out, err
		}
		out.IsolationBudget = &budget
		if over != nil {
			return out, *over
		}
		out.Isolation = &iso
	}
	if opts.Integration {
		out.Integration = integrationView(claims, cfg, module, m)
	}
	return out, nil
}

// newIsolationView assembles one module's isolation view from its manifest
// and its claims (sorted by id). Show and check's module-budget finding both
// build the view here, so they measure the same bytes.
func newIsolationView(shared SharedContext, m Manifest, moduleClaims []model.Claim, module string) IsolationView {
	iso := IsolationView{
		Shared:   shared,
		Manifest: m,
		Claims:   claimSummaries(moduleClaims),
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
	return iso
}

// measureModuleBudget measures iso and returns the refusal when its
// module-owned part is over ModuleBudgetBytes. The module part does not
// depend on the shared text (it is the whole minus the shared object), so a
// caller that only needs the verdict may pass an empty SharedContext.
func measureModuleBudget(iso IsolationView, module string) (IsolationBudget, *errIsolationOversize, error) {
	budget, err := measureIsolation(iso)
	if err != nil {
		return IsolationBudget{}, nil, err
	}
	if budget.ModuleBytes <= ModuleBudgetBytes {
		return budget, nil, nil
	}
	return budget, &errIsolationOversize{
		module:        module,
		budget:        budget,
		manifestBytes: jsonLen(iso.Manifest),
		claims:        len(iso.Claims),
	}, nil
}

// moduleBudgetFindings is the module half of the isolation cap, judged by
// check (module-manifest) rather than discovered only when someone runs
// manifest show --isolation. One finding per module over budget; work is one
// grouping pass over claims plus one marshal per module.
func moduleBudgetFindings(claims []model.Claim, modules []string, parsed map[string]Manifest) []Finding {
	byModule := make(map[string][]model.Claim, len(modules))
	for _, c := range claims {
		if c.Module != "" {
			byModule[c.Module] = append(byModule[c.Module], c)
		}
	}
	empty := SharedContext{ProjectClaims: []projectclaims.Entry{}}
	var findings []Finding
	for _, module := range modules {
		moduleClaims := byModule[module]
		sort.Slice(moduleClaims, func(i, j int) bool { return moduleClaims[i].ID < moduleClaims[j].ID })
		_, over, err := measureModuleBudget(newIsolationView(empty, parsed[module], moduleClaims, module), module)
		if err != nil {
			findings = append(findings, Finding{Module: module, Message: fmt.Sprintf("module %q isolation context could not be measured: %v", module, err)})
			continue
		}
		if over != nil {
			findings = append(findings, Finding{Module: module, Message: over.Error()})
		}
	}
	return findings
}

// measureIsolation splits the view's compact JSON into the shared context and
// everything else. Everything else is the module's: its manifest, claim
// summaries, hints, and the framing keys.
func measureIsolation(iso IsolationView) (IsolationBudget, error) {
	whole, err := json.Marshal(iso)
	if err != nil {
		return IsolationBudget{}, err
	}
	shared, err := json.Marshal(iso.Shared)
	if err != nil {
		return IsolationBudget{}, err
	}
	return IsolationBudget{
		SharedBytes:  len(shared),
		SharedBudget: SharedBudgetBytes,
		ModuleBytes:  len(whole) - len(shared),
		ModuleBudget: ModuleBudgetBytes,
	}, nil
}

func jsonLen(v any) int {
	b, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	return len(b)
}

type errIsolationOversize struct {
	module        string
	budget        IsolationBudget
	manifestBytes int
	claims        int
}

func (e errIsolationOversize) Error() string {
	return fmt.Sprintf("module %q isolation context is %d bytes, over its %d-byte module budget "+
		"(%d claim summaries; the manifest itself is %d bytes). The budget counts UTF-8 bytes while "+
		"max_claim_summary_chars counts characters, so multibyte summaries (CJK is 3 bytes a character) "+
		"can overflow it with every summary inside its cap; trim this module's claim summaries or manifest, or split the module",
		e.module, e.budget.ModuleBytes, e.budget.ModuleBudget, e.claims, e.manifestBytes)
}

// IsIsolationOversize reports a module-budget refuse from Show.
func IsIsolationOversize(err error) bool {
	var e errIsolationOversize
	return errors.As(err, &e)
}

// IsolationOversizeDetails is the refusal's machine-readable half, for the
// CLI envelope's error.details.
func IsolationOversizeDetails(err error) map[string]any {
	var e errIsolationOversize
	if !errors.As(err, &e) {
		return nil
	}
	return map[string]any{
		"module":         e.module,
		"module_bytes":   e.budget.ModuleBytes,
		"module_budget":  e.budget.ModuleBudget,
		"shared_bytes":   e.budget.SharedBytes,
		"shared_budget":  e.budget.SharedBudget,
		"claims":         e.claims,
		"manifest_bytes": e.manifestBytes,
	}
}

// List is the locked-module catalog from manifest.yaml blurbs. Each row's
// Findings is the count manifest show reports for that module: check's
// findings filtered to the module plus the project-wide ones no module owns.
// It comes from the same Check pass, so list can never read 0 where show and
// check refuse (the cross-module export rule is only judged there).
// The manifest tree is walked once for the whole list.
func List(claims []model.Claim, cfg *config.Config) []CatalogEntry {
	if cfg == nil {
		return nil
	}
	byMod := map[string][]model.Claim{}
	for _, c := range claims {
		byMod[c.Module] = append(byMod[c.Module], c)
	}
	tree, extras, walkErr := loadTree(cfg)
	var findings []Finding
	if walkErr != nil {
		findings = []Finding{{Module: "", Message: walkErr.Error()}}
	} else {
		findings = checkTree(claims, cfg, tree, extras)
	}
	perModule := map[string]int{}
	projectWide := 0
	for _, f := range findings {
		if f.Module == "" {
			projectWide++
		} else {
			perModule[f.Module]++
		}
	}
	out := make([]CatalogEntry, 0, len(cfg.Modules))
	for _, module := range cfg.Modules {
		entry := CatalogEntry{Module: module, Findings: perModule[module] + projectWide}
		modClaims := byMod[module]
		entry.ClaimCount = len(modClaims)
		for _, c := range modClaims {
			if c.Status == model.StatusLocked {
				entry.LockedClaims++
			}
		}
		entry.Locked = entry.ClaimCount > 0 && entry.LockedClaims == entry.ClaimCount
		if raw, ok := tree[RequiredRelPath(module)]; ok {
			m, _ := decodeManifest(module, RequiredRelPath(module), raw)
			entry.Summary = strings.TrimSpace(m.Summary)
			entry.Provides = len(m.Provides)
			entry.DependsOn = len(m.DependsOn)
		}
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

// claimSummaries is the card list: each claim's authored summary exactly as
// written, the same field the project claims index reads. One definition of
// "summary", never a derivation from the body.
func claimSummaries(claims []model.Claim) []ClaimSummary {
	out := make([]ClaimSummary, 0, len(claims))
	for _, c := range claims {
		out = append(out, ClaimSummary{
			ID:      c.ID,
			Title:   claimTitle(c.ID),
			Facet:   c.Facet,
			Status:  string(c.Status),
			Summary: projectclaims.Summary(c),
		})
	}
	return out
}

// integrationView reads one hop: the modules this module's depends_on names,
// and each of those modules' own manifest. It never follows a neighbor's
// depends_on, so its work is linear in this module's depends_on plus the
// neighbors' provides lists.
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
		n := Neighbor{Module: c.Module, Via: id, Provides: []ProvidedClaim{}}
		if raw, ok, _ := LoadModule(cfg, c.Module); ok {
			nm, _ := ParseBytes(c.Module, RequiredRelPath(c.Module), raw, byID)
			n.Summary = strings.TrimSpace(nm.Summary)
			n.Provides = providedClaims(nm.Provides, c.Module, byID)
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
	idx := projectclaims.Index(claims)
	if idx == nil {
		idx = []projectclaims.Entry{}
	}
	return &IntegrationView{Neighbors: neighbors, Edges: edges, ProjectClaims: idx}
}

// providedClaims resolves a neighbor's provides list to its own contract
// claims and their authored summaries, in the manifest's order. An id that is
// not one of the neighbor's contract claims is skipped: module-manifest
// already reports it, and internals never cross a module line.
func providedClaims(ids []string, module string, byID map[string]model.Claim) []ProvidedClaim {
	out := make([]ProvidedClaim, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		c, ok := byID[id]
		if !ok || seen[id] || c.Module != module || c.Facet != ContractFacet {
			continue
		}
		seen[id] = true
		out = append(out, ProvidedClaim{ID: id, Summary: projectclaims.Summary(c)})
	}
	return out
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
