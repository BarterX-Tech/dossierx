package buildorder

// mermaid.go is the ONE generator behind every build-order diagram: the
// viewer's Build order tab (internal/render/build_order_view.go) and the
// "dossierx build-order show --format mermaid" export both call Views and
// Mermaid here, so the text a reader pastes into a pull request and the text
// the page renders are the same bytes apart from the classDef lines (see
// PaletteMode). It derives everything from a LOCKED artifact's stored bytes
// plus the catalog's current claims: levels are recomputed from the stored
// rests_on through layeredTopoSort — the same derivation that produced the
// artifact's order at propose time — never read from a field the artifact
// does not carry, and never from a second Kahn pass with different tie rules.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// PaletteMode selects where a diagram's colours come from.
type PaletteMode int

const (
	// PaletteLiteral emits classDef lines carrying the fixed light-theme
	// fill/stroke/colour literals below: the CLI export, because a terminal
	// or a pull request has no theme to read.
	PaletteLiteral PaletteMode = iota
	// PaletteCSS emits classDef lines carrying ONLY stroke-dasharray for the
	// draft_* and ghost classes and no colour at all: the viewer, whose
	// style.css paints the node classes from its theme tokens so the OS
	// colour scheme and a project's viewer.theme both apply. locked_con and
	// locked_int get no classDef under this mode — an empty style list is a
	// flowchart syntax error, and ":::locked_con" lands the class on the
	// g.node whether or not a classDef exists for it.
	PaletteCSS
)

// MermaidOptions parameterises Mermaid.
type MermaidOptions struct {
	Palette PaletteMode
}

// ExcludedPhaseName is the name the sixth, non-sequence block carries in a
// PhaseView (and in the payload / export): the artifact's Excluded list,
// which is never a phase of the build.
const ExcludedPhaseName = "excluded"

// Ghost is an earlier-phase claim drawn once in a later phase's block as a
// dashed stadium node, because a claim in that block rests on it.
type Ghost struct {
	ID    string `json:"id"`
	Phase string `json:"phase"`
}

// PhaseView is one block of a module's Build order tab: the phase's fixed
// number (1-5; 0 for the excluded block), its name and definition, the claims
// the artifact placed in it (artifact order), those claims as Kahn layers
// (Levels), the earlier-phase claims drawn as ghosts, the cross-module
// dependencies listed (never drawn) per module, the excluded claims the
// block's claims rest on, and how many of the block's claims the catalog
// currently reports as locked.
type PhaseView struct {
	Number      int
	Name        string
	Definition  string
	Claims      []ClaimEntry
	Levels      [][]string
	Ghosts      []Ghost
	CrossModule map[string][]string
	// ExcludedDeps are rests_on targets that are this module's out-of-scope
	// claims: not drawn, listed under the diagram.
	ExcludedDeps []string
	Locked       int

	// classes maps each in-block claim id to its node class, decided from the
	// catalog claim at render time (facet and CURRENT status), not from the
	// artifact. Unexported: the payload carries facet/status per claim itself.
	// A PhaseView built by hand rather than by Views has no classes, and
	// Mermaid draws every one of its nodes draft_int (defaultNodeClass).
	classes map[string]string

	// nodes maps each claim id the block draws (in-block claims and ghosts)
	// to the node id Views allocated for it — NodeID(id) unless that id was
	// already taken by a different claim of the module, in which case the
	// disambiguated form (see nodeAllocator). Unexported for the same reason
	// as classes; a hand-built view has none and Mermaid falls back to
	// NodeID, which is correct for any set of ids that does not collide.
	nodes map[string]string
}

// nodeID is the node id Mermaid writes for claim id in this view.
func (v PhaseView) nodeID(id string) string {
	if n, ok := v.nodes[id]; ok {
		return n
	}
	return NodeID(id)
}

// defaultNodeClass is the class Mermaid gives a node whose PhaseView carries
// no class for it — only a hand-built view, since Views classifies every
// claim it places.
const defaultNodeClass = "draft_int"

// Count is the number of claims in the block.
func (v PhaseView) Count() int { return len(v.Claims) }

// Counts is the "N claims · N levels · N locked" line the block header and
// the export's third %% line both print.
func (v PhaseView) Counts() string {
	return fmt.Sprintf("%s · %s · %d locked",
		plural(len(v.Claims), "claim"), plural(len(v.Levels), "level"), v.Locked)
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// PhaseDefinition is the one name this package's callers use for a phase's
// prose definition; the data lives in model.BuildRoleDefinition beside the
// consts whose doc comments it mirrors.
func PhaseDefinition(r model.BuildRole) string {
	return model.BuildRoleDefinition(r)
}

// PhaseNumber is a phase's 1-based position in Phases, or 0 for anything
// that is not one of them (the excluded block included).
func PhaseNumber(r model.BuildRole) int {
	if i, ok := phaseIndex[r]; ok {
		return i + 1
	}
	return 0
}

// NodeID is the mermaid node id for a claim id: every character outside
// [A-Za-z0-9_] replaced by "_". It is not invertible — "a.b-c" and "a.b_c"
// both give "a_b_c" — which is why Views allocates node ids through a
// nodeAllocator (a second claim landing on a taken id gets a suffixed one)
// and returns the node id -> claim id index the page carries. NodeID alone
// is the right call only for a single id, or for a set known not to collide.
func NodeID(claimID string) string {
	var b strings.Builder
	for _, r := range claimID {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}

// nodeAllocator hands out one node id per claim id, module-wide, so a
// module's diagrams (and the index the page navigates by) never draw two
// claims as one node. The first claim to reach a sanitised id keeps it; a
// later, different claim whose NodeID is already taken gets NodeID plus "_"
// and the first nodeIDSuffixLen hex characters of the sha256 of ITS OWN id,
// which is stable across runs and machines (no counter, so the id a
// pull-request diff shows does not depend on iteration order) and, being
// [0-9a-f], stays inside the node-id alphabet. Only a claim that collides
// carries a suffix; every other claim's node id is exactly NodeID(id), so
// the export a reader has seen so far does not move.
type nodeAllocator struct {
	byClaim map[string]string // claim id -> node id
	byNode  map[string]string // node id -> claim id (the index Views returns)
}

const nodeIDSuffixLen = 6

func newNodeAllocator() *nodeAllocator {
	return &nodeAllocator{byClaim: map[string]string{}, byNode: map[string]string{}}
}

func (na *nodeAllocator) id(claimID string) string {
	if n, ok := na.byClaim[claimID]; ok {
		return n
	}
	n := NodeID(claimID)
	if prev, taken := na.byNode[n]; taken && prev != claimID {
		sum := sha256.Sum256([]byte(claimID))
		suffix := hex.EncodeToString(sum[:])
		// The full digest is 64 hex characters; the loop only matters if a
		// suffixed id itself collides, which needs a claim id that already
		// ends in the same hex run — extend rather than fail.
		for l := nodeIDSuffixLen; ; l++ {
			candidate := n + "_" + suffix[:l]
			if prev, taken := na.byNode[candidate]; !taken || prev == claimID {
				n = candidate
				break
			}
		}
	}
	na.byClaim[claimID] = n
	na.byNode[n] = claimID
	return n
}

// Views fills the fixed six blocks — the five Phases, then the excluded block
// — for artifact a, BY NAME: the artifact stores only the phases that had
// claims (computePhases skips an empty bucket), so a phase the artifact lacks
// is returned as a zero-count view rather than left out, and the sequence a
// reader sees is always the whole sequence. claims is the CURRENT catalog
// (every module), used for each node's facet, label and status and for
// attributing a cross-module target to its module; a claim the catalog no
// longer holds is drawn from its id alone and counted as not locked.
//
// The second return is the node id -> claim id index (see NodeID) over every
// node the module's diagrams draw, ghosts included.
//
// It returns an error, rather than a partial view, for an artifact whose
// stored rests_on names a LATER phase of the same module (which computePhases
// refuses at propose time, so it only appears in a hand-edited file), for a
// same-phase cycle (likewise), for a phase block whose name is not one of
// the five in Phases (or a name stored twice — either would drop that block's claims from every
// diagram and every count with nothing on the page saying so), and for a
// same-module rests_on target the artifact neither places nor excludes
// (which Propose cannot produce, and which the cross-module list would
// otherwise attribute to the module itself) — each of which would draw a
// diagram that lies about the order.
func Views(a *Artifact, claims []model.Claim) ([]PhaseView, map[string]string, error) {
	if a == nil {
		return nil, nil, fmt.Errorf("buildorder: Views of a nil artifact")
	}

	byID := make(map[string]model.Claim, len(claims))
	for _, c := range claims {
		byID[c.ID] = c
	}
	phaseOf := make(map[string]string) // in-sequence claim id -> stored phase name
	for _, p := range a.Phases {
		for _, c := range p.Claims {
			phaseOf[c.ID] = p.Phase
		}
	}
	excluded := make(map[string]bool, len(a.Excluded))
	for _, id := range a.Excluded {
		excluded[id] = true
	}
	stored := make(map[string]PhaseBlock, len(a.Phases))
	var badPhases []string
	for _, p := range a.Phases {
		if _, known := phaseIndex[model.BuildRole(p.Phase)]; !known {
			badPhases = append(badPhases, fmt.Sprintf("%q is not a phase", p.Phase))
		} else if _, dup := stored[p.Phase]; dup {
			badPhases = append(badPhases, fmt.Sprintf("%q is stored twice", p.Phase))
		}
		stored[p.Phase] = p
	}
	if len(badPhases) > 0 {
		return nil, nil, fmt.Errorf("buildorder: module %q's artifact holds %d claim(s) no diagram would draw: %s", a.Module, orphanedClaimCount(a, stored), strings.Join(badPhases, "; "))
	}

	// Two claim ids that sanitise to one node id (a module or facet name
	// differing only in "-" vs "_" is lint-clean and reaches here) are NOT an
	// error: the allocator gives the second a suffixed id, so both are drawn
	// and both are indexed. The alternative, refusing, cost a whole project
	// its viewer over one module's names.
	alloc := newNodeAllocator()

	views := make([]PhaseView, 0, len(Phases)+1)
	for i, phase := range Phases {
		v := PhaseView{
			Number:       i + 1,
			Name:         string(phase),
			Definition:   PhaseDefinition(phase),
			Claims:       []ClaimEntry{},
			Levels:       [][]string{},
			Ghosts:       []Ghost{},
			CrossModule:  map[string][]string{},
			ExcludedDeps: []string{},
			classes:      map[string]string{},
			nodes:        map[string]string{},
		}
		block, ok := stored[string(phase)]
		if ok {
			v.Claims = append(v.Claims, block.Claims...)
		}

		// Levels: the stored entries, in artifact order, through the one
		// derivation. The artifact's own order IS a flattening of these
		// layers (computePhases), so this re-reads the boundary it dropped.
		sortInput := make([]model.Claim, 0, len(v.Claims))
		for _, c := range v.Claims {
			sortInput = append(sortInput, model.Claim{ID: c.ID, RestsOn: c.RestsOn})
		}
		layers, cyclic := layeredTopoSort(sortInput)
		if len(cyclic) > 0 {
			return nil, nil, fmt.Errorf("buildorder: phase %q of module %q: rests_on cycle among %s", phase, a.Module, strings.Join(cyclic, ", "))
		}
		for _, layer := range layers {
			ids := make([]string, 0, len(layer))
			for _, c := range layer {
				ids = append(ids, c.ID)
			}
			v.Levels = append(v.Levels, ids)
		}

		seenGhost := map[string]bool{}
		seenExcluded := map[string]bool{}
		seenCross := map[string]bool{}
		for _, c := range v.Claims {
			v.nodes[c.ID] = alloc.id(c.ID)
			cc, known := byID[c.ID]
			if known && cc.Status == model.StatusLocked {
				v.Locked++
			}
			v.classes[c.ID] = nodeClass(c.ID, cc, known)

			for _, dep := range c.RestsOn {
				switch {
				case phaseOf[dep] == string(phase):
					// same phase: the solid edge Mermaid draws.
				case phaseOf[dep] != "":
					if phaseIndex[model.BuildRole(phaseOf[dep])] > i {
						return nil, nil, fmt.Errorf("buildorder: phase %q of module %q: %q rests on %q, which the artifact places in the later phase %q", phase, a.Module, c.ID, dep, phaseOf[dep])
					}
					if !seenGhost[dep] {
						seenGhost[dep] = true
						v.Ghosts = append(v.Ghosts, Ghost{ID: dep, Phase: phaseOf[dep]})
						v.nodes[dep] = alloc.id(dep)
					}
				case excluded[dep]:
					if !seenExcluded[dep] {
						seenExcluded[dep] = true
						v.ExcludedDeps = append(v.ExcludedDeps, dep)
					}
				default:
					mod := targetModule(dep, byID)
					if mod == a.Module {
						return nil, nil, fmt.Errorf("buildorder: phase %q of module %q: %q rests on %q, a claim of the same module that the artifact neither places in a phase nor excludes", phase, a.Module, c.ID, dep)
					}
					if !seenCross[dep] {
						seenCross[dep] = true
						v.CrossModule[mod] = append(v.CrossModule[mod], dep)
					}
				}
			}
		}
		views = append(views, v)
	}

	ex := PhaseView{
		Number:       0,
		Name:         ExcludedPhaseName,
		Definition:   PhaseDefinition(model.BuildRoleOutOfScope),
		Claims:       []ClaimEntry{},
		Levels:       [][]string{},
		Ghosts:       []Ghost{},
		CrossModule:  map[string][]string{},
		ExcludedDeps: []string{},
		classes:      map[string]string{},
		nodes:        map[string]string{},
	}
	// The excluded block names its claims and carries NOTHING derived from a
	// diagram: no levels, no ghosts, no cross-module list and a locked count
	// of 0, because the block is not an order — it is the list of claims
	// deliberately kept out of one, and "N locked" is a statement about a
	// sequence a reader is about to follow. The plan's envelope in (f) pins
	// the excluded entry as locked: 0 whatever the catalog says about them.
	for _, id := range a.Excluded {
		ex.Claims = append(ex.Claims, ClaimEntry{ID: id})
	}
	views = append(views, ex)
	return views, alloc.byNode, nil
}

// orphanedClaimCount is the number of stored claim entries the five
// in-sequence blocks would not carry: everything under an unrecognised
// phase name plus every block a same-named later block shadowed in stored.
func orphanedClaimCount(a *Artifact, stored map[string]PhaseBlock) int {
	placed := 0
	for _, phase := range Phases {
		if block, ok := stored[string(phase)]; ok {
			placed += len(block.Claims)
		}
	}
	total := 0
	for _, p := range a.Phases {
		total += len(p.Claims)
	}
	return total - placed
}

// targetModule attributes a cross-module (or catalog-unknown) rests_on
// target to a module for the .bo-cross list: the catalog claim's Module when
// the catalog has it, else the id's first segment when the id has the linted
// three-segment shape, else "unknown".
func targetModule(id string, byID map[string]model.Claim) string {
	if c, ok := byID[id]; ok && c.Module != "" {
		return c.Module
	}
	segs := strings.Split(id, ".")
	if len(segs) == 3 && segs[0] != "" && segs[1] != "" && segs[2] != "" {
		return segs[0]
	}
	return "unknown"
}

// nodeClass is the node's class: contract and doctrine facets use the _con
// pair, everything else _int; a claim the catalog holds as locked is
// locked_*, anything else (draft, or gone from the catalog) draft_*. The
// facet comes from the catalog claim when known, else from the id.
func nodeClass(id string, c model.Claim, known bool) string {
	facet := c.Facet
	if !known || facet == "" {
		if segs := strings.Split(id, "."); len(segs) == 3 {
			facet = segs[1]
		}
	}
	kind := "int"
	if facet == "contract" || facet == "doctrine" {
		kind = "con"
	}
	status := "draft"
	if known && c.Status == model.StatusLocked {
		status = "locked"
	}
	return status + "_" + kind
}

// Class reports the node class Views decided for id in this block ("" when
// id is not one of the block's claims).
func (v PhaseView) Class(id string) string { return v.classes[id] }

// labelWrap is the character width labels are word-wrapped at, with <br/>.
const labelWrap = 22

// claimLabel and displayCase duplicate components.ClaimLabel/DisplayCase
// (internal/render/components/components.go) on purpose: internal/render is
// top-of-stack (.golangci.yml's render-is-top-of-stack rule, which lists
// this package) and imports internal/buildorder, so this package cannot
// import components in return. internal/graph/build.go:300 is the precedent
// and says the same; internal/render/build_order_view_test.go pins the two
// implementations to agree, so a diagram node and a claim card never label
// one claim differently. The rule: an id that is not exactly three non-empty
// segments is the id VERBATIM; otherwise DisplayCase of the slug.
func claimLabel(id string) string {
	segs := strings.Split(id, ".")
	if len(segs) != 3 || segs[0] == "" || segs[1] == "" || segs[2] == "" {
		return id
	}
	return displayCase(segs[2])
}

func displayCase(s string) string {
	words := strings.FieldsFunc(s, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for i, w := range words {
		if w == "" {
			continue
		}
		r := []rune(w)
		r[0] = []rune(strings.ToUpper(string(r[0])))[0]
		words[i] = string(r)
	}
	return strings.Join(words, " ")
}

// mermaidEscape makes text safe inside a mermaid label: the characters that
// are syntax to mermaid's parser or markup to its HTML labels become
// mermaid's own entity forms (mermaid's encodeEntities turns "#name;" into
// "&name;" and "#digits;" into "&#digits;" before the label reaches the
// DOM), so the emitted text carries none of `"`, `#`, `;`, `<`, `>` or `&`
// raw. It is applied per line, BEFORE the <br/> joins, so those tags stay
// tags.
func mermaidEscape(text string) string {
	var b strings.Builder
	for _, r := range text {
		switch r {
		case '"':
			b.WriteString("#quot;")
		case '#':
			b.WriteString("#35;")
		case ';':
			b.WriteString("#59;")
		case '<':
			b.WriteString("#lt;")
		case '>':
			b.WriteString("#gt;")
		case '&':
			b.WriteString("#amp;")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// wrapLabel word-wraps text at labelWrap characters (a single word longer
// than that stands alone on its line) and escapes each line.
func wrapLabel(text string) string {
	var lines []string
	var cur []rune
	for _, w := range strings.Fields(text) {
		wr := []rune(w)
		if len(cur) == 0 {
			cur = wr
			continue
		}
		if len(cur)+1+len(wr) > labelWrap {
			lines = append(lines, string(cur))
			cur = wr
			continue
		}
		cur = append(cur, ' ')
		cur = append(cur, wr...)
	}
	if len(cur) > 0 {
		lines = append(lines, string(cur))
	}
	for i, l := range lines {
		lines[i] = mermaidEscape(l)
	}
	return strings.Join(lines, "<br/>")
}

// NodeLabel is the wrapped, escaped mermaid label for a claim id, under the
// same rule the claim cards use (claimLabel).
func NodeLabel(id string) string {
	return wrapLabel(claimLabel(id))
}

// HeaderLine is the first %% line of a phase's export: "phase N of 5: name".
func (v PhaseView) HeaderLine() string {
	return fmt.Sprintf("phase %d of %d: %s", v.Number, len(Phases), v.Name)
}

// legendLine is the fourth %% line, the same on every diagram.
const legendLine = "solid arrow: rests on, same phase. dotted arrow: rests on an earlier phase (ghost node)."

var literalClassDefs = []string{
	"classDef locked_con fill:#dfeee6,stroke:#287052,color:#14231b",
	"classDef draft_con fill:#f4f6f3,stroke:#287052,stroke-dasharray:4 3,color:#14231b",
	"classDef locked_int fill:#e3eaf0,stroke:#205b78,color:#14231b",
	"classDef draft_int fill:#f2f5f7,stroke:#205b78,stroke-dasharray:4 3,color:#14231b",
	"classDef ghost fill:#ffffff,stroke:#b9c2bc,stroke-dasharray:2 3,color:#6b776f",
}

var cssClassDefs = []string{
	"classDef draft_con stroke-dasharray:4 3",
	"classDef draft_int stroke-dasharray:4 3",
	"classDef ghost stroke-dasharray:2 3",
}

// Mermaid renders one phase view as one flowchart: four leading %% comment
// lines (phase number and name; the definition VERBATIM — a definition holds
// no newline, and mermaid discards comments; the counts; the legend), then
// "flowchart TD", one node per claim in artifact order, then for each claim
// in that order its rests_on edges in their stored order — an earlier-phase
// target declared as a ghost stadium node on first reference and joined by a
// dotted edge, a same-phase target joined by a solid edge, everything else
// omitted (Views listed it) — then the classDef lines opts.Palette selects.
// It returns "" for a phase with no claims and for the excluded block: they
// have no diagram, and the export prints nothing for them so every chunk of
// a "--format mermaid" export contains a flowchart.
func Mermaid(v PhaseView, opts MermaidOptions) string {
	if v.Number == 0 || len(v.Claims) == 0 {
		return ""
	}
	phaseOfGhost := make(map[string]string, len(v.Ghosts))
	for _, g := range v.Ghosts {
		phaseOfGhost[g.ID] = g.Phase
	}
	inBlock := make(map[string]bool, len(v.Claims))
	for _, c := range v.Claims {
		inBlock[c.ID] = true
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%%%% %s\n", v.HeaderLine())
	fmt.Fprintf(&b, "%%%% %s\n", v.Definition)
	fmt.Fprintf(&b, "%%%% %s\n", v.Counts())
	fmt.Fprintf(&b, "%%%% %s\n", legendLine)
	b.WriteString("flowchart TD\n")
	for _, c := range v.Claims {
		cls := v.classes[c.ID]
		if cls == "" {
			cls = defaultNodeClass
		}
		fmt.Fprintf(&b, "  %s[\"%s\"]:::%s\n", v.nodeID(c.ID), NodeLabel(c.ID), cls)
	}
	declared := map[string]bool{}
	for _, c := range v.Claims {
		for _, dep := range c.RestsOn {
			switch {
			case inBlock[dep]:
				fmt.Fprintf(&b, "  %s --> %s\n", v.nodeID(dep), v.nodeID(c.ID))
			case phaseOfGhost[dep] != "":
				if !declared[dep] {
					declared[dep] = true
					fmt.Fprintf(&b, "  %s([\"%s<br/><i>%s</i>\"]):::ghost\n", v.nodeID(dep), NodeLabel(dep), mermaidEscape(phaseOfGhost[dep]))
				}
				fmt.Fprintf(&b, "  %s -.-> %s\n", v.nodeID(dep), v.nodeID(c.ID))
			}
		}
	}
	defs := literalClassDefs
	if opts.Palette == PaletteCSS {
		defs = cssClassDefs
	}
	for _, d := range defs {
		b.WriteString("  " + d + "\n")
	}
	return b.String()
}

// CrossModuleNames returns the CrossModule keys sorted, for deterministic
// rendering.
func (v PhaseView) CrossModuleNames() []string {
	names := make([]string, 0, len(v.CrossModule))
	for k := range v.CrossModule {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
