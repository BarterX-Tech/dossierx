// Package layout owns the ONE fact about where the engine writes that every
// command has to agree on before it reads anything: the project's runtime
// artifacts live under the build directory (internal/config/paths.go), and a
// project that still keeps them at the project root — the layout every release
// before this one wrote — is refused, on every verb, with the exact commands
// that move each file.
//
// It holds exactly four things: LegacyFiles (the scan), Refuse (the refusal,
// built here so the verbatim text and the tracked/untracked classification have
// one home instead of one per caller; RefuseMoves is the same refusal over a
// scan the caller already ran), EnsureBuildGitignore (the build
// directory's own .gitignore, written once) and RecommendedGitignore (the
// replacement block the store-gitignored finding, the README and
// EnsureBuildGitignore's doc all print, so the three cannot drift). The
// gitignore VERDICT — whether a store the engine is about to write would be
// ignored — is not here: it needs git's exit status through the runner
// internal/check already owns, and internal/check imports this package, so it
// lives beside that runner as check.Gitignored. This package imports
// internal/config and internal/cliout and nothing else under internal/.
package layout

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/cliout"
	"github.com/BarterX-Tech/dossierx/internal/config"
)

// The seven legacy kinds, by the root names every release before this one
// wrote. Two are per-module (a prefix and a suffix around the module name),
// five are fixed.
const (
	legacyBuildOrderPrefix = ".build-order."
	legacyCodeLinksPrefix  = ".implementation."
	legacyLockStore        = ".dossierx-lock-store.json"
	legacyCommentDigest    = ".dossierx-comment-digest.json"
	legacyFlagStore        = ".dossierx-flag-store.json"
	legacyCatalog          = ".catalog.json"
	legacyViewer           = "viewer/index.html"
)

// LegacyFileNames is every fixed legacy base name plus the two per-module
// prefixes, exported so internal/check's index scan can recognise a legacy
// store staged half-way through a migration by the same names this package
// scans for.
var (
	LegacyLockStoreName     = legacyLockStore
	LegacyCommentDigestName = legacyCommentDigest
	LegacyFlagStoreName     = legacyFlagStore
	LegacyCatalogName       = legacyCatalog
	LegacyViewerPath        = legacyViewer
	LegacyBuildOrderPrefix  = legacyBuildOrderPrefix
	LegacyCodeLinksPrefix   = legacyCodeLinksPrefix
)

// Kind names one of the seven legacy kinds a Move is about — or, on an
// IgnoredPath only, the build directory's own .gitignore, which never lived at
// the root and so is never a Move.
type Kind string

// The kinds, in the order the printed block lists them.
const (
	KindLockStore     Kind = "lock-store"
	KindCommentDigest Kind = "comment-digest"
	KindFlagStore     Kind = "flag-store"
	KindBuildOrder    Kind = "build-order"
	KindCodeLinks     Kind = "code-links"
	KindCatalog       Kind = "catalog"
	KindViewer        Kind = "viewer"
	// KindBuildGitignore is <build_dir>/.gitignore, the one engine-written
	// path the gitignore guard checks that is not a store or an artifact.
	KindBuildGitignore Kind = "build-gitignore"
)

// Move is one legacy file found at the project root and where it goes.
//
// From and To are project-relative, slash-separated paths — the form a reader
// pastes into a shell in the project directory. Tracked is decided per file
// from the git index (LegacyFiles): it chooses `git mv` over `mv` on the printed
// line, and it is in the JSON details so a consumer can tell the two apart
// without parsing the prose. Regenerated marks the two kinds (catalog, viewer)
// whose recovery is a removal rather than a move, because every `check` rewrites
// them under the build directory.
type Move struct {
	From        string `json:"from"`
	To          string `json:"to"`
	Tracked     bool   `json:"tracked"`
	Kind        Kind   `json:"kind"`
	Module      string `json:"module,omitempty"`
	Regenerated bool   `json:"regenerated,omitempty"`
	// NewExists reports that the destination ALSO exists — a half-done move.
	NewExists bool `json:"new_exists,omitempty"`
	// InWorkTree is false when no .git was found above the project (or git is
	// not on PATH): the printed block then carries no git command at all.
	InWorkTree bool `json:"-"`
}

// LegacyFiles scans cfg.Dir() for the seven legacy kinds and returns one Move
// per file found, in the order the printed block lists them: the lock store,
// the comment digest, the flag store, every build order (by module), every
// code-links artifact (by module), the catalog, the viewer. It returns nil when
// the project is on the current layout.
//
// Tracked is decided by asking git ONCE — `git ls-files -z -- <the found files>`
// from the project directory — when a .git directory or file is found walking
// up from cfg.Dir() and git is on PATH; otherwise every file is Tracked: false
// and InWorkTree: false. The distinction is what makes the printed block run:
// `git mv` exits 128 on an untracked source and outside a work tree, and three
// of the seven kinds are untracked in the ordinary case (a proposed-but-unlocked
// build order, the flag store, a code-links artifact).
func LegacyFiles(cfg *config.Config) ([]Move, error) {
	root := cfg.Dir()
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("layout: read project directory %s: %w", root, err)
	}
	buildRel := buildDirRelative(cfg)

	var lockStore, commentDigest, flagStore, catalog bool
	var buildOrders, codeLinks []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		switch {
		case name == legacyLockStore:
			lockStore = true
		case name == legacyCommentDigest:
			commentDigest = true
		case name == legacyFlagStore:
			flagStore = true
		case name == legacyCatalog:
			catalog = true
		case strings.HasPrefix(name, legacyBuildOrderPrefix) && strings.HasSuffix(name, ".json"):
			if m := strings.TrimSuffix(strings.TrimPrefix(name, legacyBuildOrderPrefix), ".json"); m != "" {
				buildOrders = append(buildOrders, m)
			}
		case strings.HasPrefix(name, legacyCodeLinksPrefix) && strings.HasSuffix(name, ".json"):
			if m := strings.TrimSuffix(strings.TrimPrefix(name, legacyCodeLinksPrefix), ".json"); m != "" {
				codeLinks = append(codeLinks, m)
			}
		}
	}
	sort.Strings(buildOrders)
	sort.Strings(codeLinks)
	viewerInfo, viewerErr := os.Stat(filepath.Join(root, filepath.FromSlash(legacyViewer)))
	viewer := viewerErr == nil && !viewerInfo.IsDir()

	var moves []Move
	add := func(kind Kind, module, from, to string, regenerated bool) {
		moves = append(moves, Move{From: from, To: to, Kind: kind, Module: module, Regenerated: regenerated})
	}
	if lockStore {
		add(KindLockStore, "", legacyLockStore, path.Join(buildRel, config.LedgerDirName, config.LockStoreFileName), false)
	}
	if commentDigest {
		add(KindCommentDigest, "", legacyCommentDigest, path.Join(buildRel, config.LedgerDirName, config.CommentDigestFileName), false)
	}
	if flagStore {
		add(KindFlagStore, "", legacyFlagStore, path.Join(buildRel, config.LedgerDirName, config.FlagStoreFileName), false)
	}
	for _, m := range buildOrders {
		add(KindBuildOrder, m, legacyBuildOrderPrefix+m+".json", path.Join(buildRel, config.BuildOrderDirName, m+".json"), false)
	}
	for _, m := range codeLinks {
		add(KindCodeLinks, m, legacyCodeLinksPrefix+m+".json", path.Join(buildRel, config.CodeLinksDirName, m+".json"), false)
	}
	if catalog {
		add(KindCatalog, "", legacyCatalog, path.Join(buildRel, config.CatalogDirName, "catalog.json"), true)
	}
	if viewer {
		add(KindViewer, "", legacyViewer, path.Join(buildRel, config.ViewerDirName, "index.html"), true)
	}
	if len(moves) == 0 {
		return nil, nil
	}

	for i := range moves {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(moves[i].To))); err == nil {
			moves[i].NewExists = true
		}
	}

	inWorkTree := InWorkTree(root)
	if inWorkTree {
		if _, err := exec.LookPath("git"); err != nil {
			inWorkTree = false
		}
	}
	for i := range moves {
		moves[i].InWorkTree = inWorkTree
	}
	if inWorkTree {
		tracked, err := trackedFiles(root, moves)
		if err != nil {
			return nil, err
		}
		for i := range moves {
			moves[i].Tracked = tracked[moves[i].From]
		}
	}
	return moves, nil
}

// InWorkTree reports whether a .git directory or file (the submodule and
// worktree form) exists at dir or any directory above it. It consults no git
// binary, which is the point: the runner in internal/check answers the same
// ErrNoIndex for "no git on PATH" and "not a work tree", and the two call for
// different answers here.
func InWorkTree(dir string) bool {
	dir = filepath.Clean(dir)
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
}

// trackedFiles runs one `git ls-files` from the project directory over the
// found files and reports which of them the index holds.
func trackedFiles(root string, moves []Move) (map[string]bool, error) {
	args := []string{"-c", "core.quotepath=false", "ls-files", "-z", "--"}
	for _, m := range moves {
		args = append(args, m.From)
	}
	cmd := exec.Command("git", args...) //nolint:gosec // fixed binary, fixed argv; no shell involved
	cmd.Dir = root
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 128 {
			// A .git that git itself cannot read as a work tree (a bare
			// repository, a corrupt one): nothing here is tracked as far as
			// a `git mv` is concerned, so the block prints plain moves.
			return map[string]bool{}, nil
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("layout: git ls-files: %s", msg)
	}
	tracked := map[string]bool{}
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			tracked[p] = true
		}
	}
	return tracked, nil
}

// buildDirRelative is the build directory as a project-relative, slash form
// ("build" for the default), which is what the printed lines name.
func buildDirRelative(cfg *config.Config) string {
	rel, err := filepath.Rel(cfg.Dir(), cfg.BuildDirPath())
	if err != nil {
		return filepath.ToSlash(cfg.BuildDirPath())
	}
	return filepath.ToSlash(rel)
}

// Refuse returns the layout_legacy error when cfg's project keeps any of the
// seven legacy kinds at the root, and nil otherwise. buildDirIgnored appends
// the "build/ is ignored" hint — a verdict this package cannot compute (it
// needs the runner in internal/check), so each caller supplies it from
// check.Gitignored.
//
// The error is a *cliout.CodedError carrying cliout.CodeLayoutLegacy, the
// verbatim recovery text, and Details{"moves": []Move} so `--format json`
// carries the same classification the prose prints.
func Refuse(cfg *config.Config, buildDirIgnored bool) error {
	moves, err := LegacyFiles(cfg)
	if err != nil {
		return cliout.Errorf(cliout.CodeInternal, "%w", err)
	}
	return RefuseMoves(cfg, moves, buildDirIgnored)
}

// RefuseMoves is Refuse over a LegacyFiles result the caller already has, so a
// caller that needs the scan's emptiness BEFORE deciding whether to pay for the
// git verdict behind buildDirIgnored (cmd/dossierx's loadConfig, on every verb)
// does not walk the project directory twice. nil when moves is empty.
func RefuseMoves(cfg *config.Config, moves []Move, buildDirIgnored bool) error {
	if len(moves) == 0 {
		return nil
	}
	return refusal(cfg, moves, buildDirIgnored)
}

// RefuseTracked is Refuse for a set of legacy files the GIT INDEX holds —
// "check --staged"'s half of the refusal, which judges the commit rather than
// the working tree. names are project-relative legacy paths as found in the
// index; every one of them is tracked by definition.
func RefuseTracked(cfg *config.Config, names []string) error {
	if len(names) == 0 {
		return nil
	}
	buildRel := buildDirRelative(cfg)
	var moves []Move
	byKind := map[Kind][]Move{}
	for _, n := range names {
		base := filepath.ToSlash(n)
		var m Move
		switch {
		case base == legacyLockStore:
			m = Move{From: base, To: path.Join(buildRel, config.LedgerDirName, config.LockStoreFileName), Kind: KindLockStore}
		case base == legacyCommentDigest:
			m = Move{From: base, To: path.Join(buildRel, config.LedgerDirName, config.CommentDigestFileName), Kind: KindCommentDigest}
		case base == legacyFlagStore:
			m = Move{From: base, To: path.Join(buildRel, config.LedgerDirName, config.FlagStoreFileName), Kind: KindFlagStore}
		case base == legacyCatalog:
			m = Move{From: base, To: path.Join(buildRel, config.CatalogDirName, "catalog.json"), Kind: KindCatalog, Regenerated: true}
		case base == legacyViewer:
			m = Move{From: base, To: path.Join(buildRel, config.ViewerDirName, "index.html"), Kind: KindViewer, Regenerated: true}
		case strings.HasPrefix(base, legacyBuildOrderPrefix) && strings.HasSuffix(base, ".json"):
			mod := strings.TrimSuffix(strings.TrimPrefix(base, legacyBuildOrderPrefix), ".json")
			m = Move{From: base, To: path.Join(buildRel, config.BuildOrderDirName, mod+".json"), Kind: KindBuildOrder, Module: mod}
		case strings.HasPrefix(base, legacyCodeLinksPrefix) && strings.HasSuffix(base, ".json"):
			mod := strings.TrimSuffix(strings.TrimPrefix(base, legacyCodeLinksPrefix), ".json")
			m = Move{From: base, To: path.Join(buildRel, config.CodeLinksDirName, mod+".json"), Kind: KindCodeLinks, Module: mod}
		default:
			continue
		}
		m.Tracked = true
		m.InWorkTree = true
		byKind[m.Kind] = append(byKind[m.Kind], m)
	}
	for _, k := range []Kind{KindLockStore, KindCommentDigest, KindFlagStore, KindBuildOrder, KindCodeLinks, KindCatalog, KindViewer} {
		ms := byKind[k]
		sort.Slice(ms, func(i, j int) bool { return ms[i].Module < ms[j].Module })
		moves = append(moves, ms...)
	}
	if len(moves) == 0 {
		return nil
	}
	return refusal(cfg, moves, false)
}

// IsLegacyName reports whether a project-relative path names one of the seven
// legacy kinds at the project root. It is the predicate "check --staged" runs
// over the index listing of the project's own subtree.
func IsLegacyName(rel string) bool {
	rel = filepath.ToSlash(rel)
	switch {
	case rel == legacyLockStore, rel == legacyCommentDigest, rel == legacyFlagStore, rel == legacyCatalog, rel == legacyViewer:
		return true
	case strings.Contains(rel, "/"):
		return false
	case strings.HasPrefix(rel, legacyBuildOrderPrefix) && strings.HasSuffix(rel, ".json"):
		return len(rel) > len(legacyBuildOrderPrefix)+len(".json")
	case strings.HasPrefix(rel, legacyCodeLinksPrefix) && strings.HasSuffix(rel, ".json"):
		return len(rel) > len(legacyCodeLinksPrefix)+len(".json")
	}
	return false
}

func refusal(cfg *config.Config, moves []Move, buildDirIgnored bool) error {
	for _, m := range moves {
		if m.NewExists && !m.Regenerated {
			return cliout.Errorf(cliout.CodeLayoutLegacy,
				"both %s and %s exist. Two ledgers cannot both be the record. Keep the one under %s/ if it is the newer copy, delete the root copy with git rm, and run: dossierx check --validate. If the root copy is the newer one, move it over the %s/ copy with git mv -f.",
				m.To, m.From, buildDirRelative(cfg), buildDirRelative(cfg)).
				WithDetails(map[string]any{"moves": moves})
		}
	}
	return cliout.Errorf(cliout.CodeLayoutLegacy, "%s", RecoveryText(cfg, moves, buildDirIgnored)).
		WithDetails(map[string]any{"moves": moves}).
		WithHint("run the printed block in the project directory, then: dossierx check --validate")
}

// RecoveryText is the verbatim layout_legacy message for moves: the sentence,
// the pasteable block (one `mkdir -p` naming only the destination directories
// the found files need, then one line per file in kind order), and the
// closing sentence. Every line exits 0 in the no-build/ state the refusal is
// printed from: `git mv` neither creates directories nor accepts an untracked
// source, and `git rm --cached` is fatal on an untracked path, which is why the
// block has the shape it has.
func RecoveryText(cfg *config.Config, moves []Move, buildDirIgnored bool) string {
	inWorkTree := len(moves) > 0 && moves[0].InWorkTree
	var b strings.Builder
	b.WriteString("this project keeps dossierx artifacts at the project root, which this release no longer reads. ")
	if inWorkTree {
		b.WriteString("Move each one so its history and its approvals travel with it (git mv for the files git tracks, mv for the rest), then stage the moves in the same commit as any config change:\n\n")
	} else {
		b.WriteString("Move each one so its approvals stay beside its claims:\n\n")
	}
	for _, line := range RecoveryLines(moves) {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	if runtime.GOOS == "windows" {
		b.WriteString(WindowsShellHint)
		b.WriteString(" ")
	}
	b.WriteString("Only the lines for files that exist are printed, one per file, in this order. Signatures hash bytes, not paths, so nothing needs re-locking. Then run: dossierx check --validate")
	if buildDirIgnored {
		b.WriteString("\n\n")
		b.WriteString(BuildDirIgnoredHint(buildDirRelative(cfg)))
		b.WriteString("\n\n")
		b.WriteString(indent(RecommendedGitignore))
	}
	return b.String()
}

// WindowsShellHint is the sentence RecoveryText adds on windows. The block's
// mv, rm -f and mkdir -p exist in Git Bash (the shell Git for Windows installs)
// and not in cmd.exe, where mv and rm are not commands and `mkdir -p` creates a
// directory named -p, or in PowerShell, which rejects -p as ambiguous; pasted
// there the block does not run, and because Refuse runs from loadConfig the
// reader is refused on every verb until they find out why.
const WindowsShellHint = "Paste the block into Git Bash, the shell Git for Windows installs: cmd.exe and PowerShell have no mv, rm -f or mkdir -p."

// RecoveryLines is the pasteable block without its surrounding prose: the
// `mkdir -p` line first, then one line per move.
func RecoveryLines(moves []Move) []string {
	inWorkTree := len(moves) > 0 && moves[0].InWorkTree
	var dirs []string
	seen := map[string]bool{}
	for _, m := range moves {
		if m.Regenerated {
			continue
		}
		d := path.Dir(m.To)
		if !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	var lines []string
	if len(dirs) > 0 {
		lines = append(lines, "mkdir -p "+strings.Join(dirs, " "))
	}
	for _, m := range moves {
		switch {
		case m.Regenerated && inWorkTree:
			lines = append(lines, fmt.Sprintf("git rm --cached --ignore-unmatch %s && rm -f %s", m.From, m.From))
		case m.Regenerated:
			lines = append(lines, "rm -f "+m.From)
		case m.Tracked:
			lines = append(lines, fmt.Sprintf("git mv %s %s", m.From, m.To))
		default:
			lines = append(lines, fmt.Sprintf("mv %s %s", m.From, m.To))
		}
	}
	return lines
}

// BuildDirIgnoredHint is the sentence appended to the layout_legacy refusal
// when the build directory is ALSO ignored: moving the files first would put
// every approval under a pattern git never re-enters.
func BuildDirIgnoredHint(buildRel string) string {
	return buildRel + "/ is ignored by .gitignore; replace that pattern with the block under store-gitignored below, or set build_dir to a directory the pattern does not match, before committing the move."
}

// RecommendedGitignore is the block a project's .gitignore must carry in
// place of a bare `build/` pattern. It is the ONE copy: the store-gitignored
// finding, EnsureBuildGitignore's doc comment and README's "Where DossierX
// writes" subsection all print it, so the three cannot drift.
//
// Its shape is dictated by two git rules. Git never re-enters an excluded
// directory, so `!build/ledger/` under a `build/` pattern is inert — the
// pattern has to be `build/*` so the directory itself is not excluded. And a
// trailing-slash negation is directory-only and can only match once the
// directory EXISTS on disk, so `!build/code-links/` does nothing for a project
// that has no code links yet; each tracked kind therefore gets a slash-less
// negation for the directory and a `/*` re-include for its files.
const RecommendedGitignore = `build/*
!build/.gitignore
!build/ledger
!build/ledger/*
!build/build-order
!build/build-order/*
!build/code-links
!build/code-links/*`

// BuildGitignoreContent is what EnsureBuildGitignore writes into
// <build_dir>/.gitignore: the generated kinds (catalog, viewer) and the
// transient files (sentinels, temp and probe files) are ignored; the tracked
// kinds under ledger/, build-order/ and code-links/ are not.
const BuildGitignoreContent = `# Written by dossierx check. Generated kinds are ignored; tracked kinds are not.
catalog/
viewer/
*.lock
*.tmp-*
*.probe-*
`

// EnsureBuildGitignore writes <build_dir>/.gitignore with BuildGitignoreContent
// when the file is absent, creating the build directory if needed. An existing
// file is never rewritten: it is the project's to edit. The tracked kinds it
// leaves un-ignored are exactly the negations in RecommendedGitignore.
func EnsureBuildGitignore(cfg *config.Config) error {
	path := cfg.BuildGitignorePath()
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("layout: stat %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("layout: create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(BuildGitignoreContent), 0o644); err != nil {
		return fmt.Errorf("layout: write %s: %w", path, err)
	}
	return nil
}

// ---------------------------------------------------------------------
// the store-gitignored texts, so the finding, the refusal and the warning
// have one home beside the block they print
// ---------------------------------------------------------------------

// IgnoredPath describes one engine-written path git reports as ignored: which
// kind it is (the harm sentence is per kind), the path and the .gitignore
// source as a reader sees them FROM THE PROJECT DIRECTORY, the pattern and line
// `git check-ignore -v` reported, and where that source sits — in the project,
// above it (the monorepo case, whose recovery is a per-project negation rather
// than the block), or outside the repository altogether (a core.excludesFile,
// which no negation pasted beside the pattern can scope to one project).
type IgnoredPath struct {
	Path         string
	Kind         Kind
	Module       string // the module, for KindBuildOrder and KindCodeLinks
	What         string // "the lock ledger", "module widget's build order", ...
	Source       string
	Line         int
	Pattern      string
	AboveProject bool
	// OutsideRepository: Source is not a file in this repository at all (git
	// reported it by absolute path: core.excludesFile). Source is then that
	// absolute path, verbatim, and ProjectNegation is the negation to add to
	// the PROJECT's own .gitignore, which outranks the machine-wide file.
	OutsideRepository bool
	// ProjectNegation is the one-line negation for THIS project's build
	// directory, printed instead of the block when AboveProject (relative to
	// Source, e.g. "!docs/dossierx/build/") or OutsideRepository (relative to
	// the project's own .gitignore, e.g. "!build/") is true.
	ProjectNegation string
	BuildRel        string
}

// StoreGitignoredMessage is the store-gitignored finding's text for one
// ignored, untracked path, and the body of the store_gitignored refusal.
//
// The harm clause is per Kind. The sentence that is true of the lock ledger —
// a clone has no approval record, check fails there with lock-ledger-absent —
// is false of a build order, of the flag store and of build/.gitignore, and a
// stated harm that can be refuted on the evidence is the report that teaches
// people to bypass the gate (internal/check/ledger.go's rule on false reports).
func StoreGitignoredMessage(p IgnoredPath) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s is ignored by .gitignore (pattern %q at %s:%d), so %s never reaches the repository: %s ",
		p.Path, p.Pattern, p.Source, p.Line, p.What, storeGitignoredHarm(p))
	// The negation caveat names the directory holding the reported path, so
	// the obvious "!build/build-order/" a reader would try for a build order
	// is explained before they try it. A file directly under the build
	// directory (build/.gitignore) has no directory of its own to negate.
	if dir := path.Dir(p.Path); dir == p.BuildRel || dir == "." {
		fmt.Fprintf(&b, "Git never re-enters an excluded directory, so \"!%s\" alone does nothing. ", p.Path)
	} else {
		fmt.Fprintf(&b, "Git never re-enters an excluded directory, so \"!%s/\" alone does nothing, and a directory-only negation (\"!%s/\") cannot match a directory that does not exist yet. ", dir, dir)
	}
	switch {
	case p.OutsideRepository:
		fmt.Fprintf(&b, "That pattern is in %s, which is not a .gitignore in this repository but a machine-wide excludes file (core.excludesFile): a replacement block pasted there would un-ignore every project's build output on this machine and reach no collaborator. Set build_dir in project.config.yaml to a directory the pattern does not match (for example build_dir: dossierx-build) and move the files there, or add to this project's own .gitignore (created if absent) one negation for its build directory:\n\n  %s\n\nwhich git honours because a pattern in the repository outranks the machine-wide file and it negates the excluded directory itself.",
			p.Source, p.ProjectNegation)
	case p.AboveProject:
		fmt.Fprintf(&b, "That pattern is in %s, above this project, so pasting a replacement block there would un-ignore every other project's build output. Set build_dir in project.config.yaml to a directory the pattern does not match (for example build_dir: dossierx-build) and move the files there, or append after that pattern, in %s, one negation for this project's build directory:\n\n  %s\n\nwhich git honours because it negates the excluded directory itself.",
			p.Source, p.Source, p.ProjectNegation)
	default:
		b.WriteString("Replace the pattern with:\n\n")
		b.WriteString(indent(RecommendedGitignore))
		b.WriteString("\n\nor, if that pattern belongs to another toolchain, set build_dir in project.config.yaml to a directory it does not match (for example build_dir: dossierx-build) and move the files there.")
	}
	return b.String()
}

// storeGitignoredHarm is the per-kind clause after "never reaches the
// repository:" — who does what on the clone, and what goes wrong for them.
// Each names the finding a collaborator's check actually reports in that
// state, or says that nothing reports it, which is the worse case.
func storeGitignoredHarm(p IgnoredPath) string {
	switch p.Kind {
	case KindCommentDigest:
		return "a collaborator or CI cloning this project has no record of which comment threads each approval covered, so check fails there with comment-digest-absent (or with lock-ledger-absent when the ledger beside it is ignored too), and no thread digested here can be compared against its approval there."
	case KindFlagStore:
		return "a collaborator or CI cloning this project has no record that any claim was flagged: claim reaudit there finds no pending flag to confirm, and every flag written here vanishes on the next clone with nothing to say so."
	case KindBuildOrder:
		return fmt.Sprintf("a collaborator or CI cloning this project has no approved build order for module %q: an agent there builds in whatever order the claims imply now, and check reports build-order-ledger-abandoned for that module there when its ledger record did travel — and nothing at all when it did not.", p.Module)
	case KindCodeLinks:
		return fmt.Sprintf("a collaborator or CI cloning this project has no code links for module %q: check there prints no code-link status for it at all, drift between its claims and the source files linked here goes unreported, and the links vanish on the next clone with nothing to say so.", p.Module)
	case KindBuildGitignore:
		return "every clone starts without the build directory's own ignore rules until its first check rewrites the file from the default, so any edit made to it here (it is the project's to edit) is lost on every clone with nothing to say so."
	default:
		// The lock ledger — the plan's verbatim body — and any kind this
		// package does not know, which the ledger's sentence is the safest
		// description of.
		return "a collaborator or CI cloning this project has no approval record to compare against, check fails there with lock-ledger-absent, and every flag written here vanishes on the next clone with nothing to say so."
	}
}

// IgnoredButTrackedWarning is the envelope warning for a path that IS in the
// index although a pattern matches it (force-added, or committed before the
// pattern was written). It is not the finding's harm — that ledger does reach
// every collaborator — but nothing will stage the next NEW artifact.
func IgnoredButTrackedWarning(p IgnoredPath) string {
	return fmt.Sprintf("%s is in the repository but matched by .gitignore pattern %q (%s:%d): it was force-added, so nothing will stage the next NEW artifact under %s/ (a new module's %s/build-order/<m>.json, or a first flag store) and git add -A will never pick one up; replace the pattern with the block under store-gitignored, or set build_dir to a directory the pattern does not match",
		p.Path, p.Pattern, p.Source, p.Line, p.BuildRel, p.BuildRel)
}

// GitUnavailableMessage is the store_gitignored refusal's text when the
// project is inside a work tree and git could not answer: the approval verbs
// refuse on it; the read-only check modes report it as data.gitignore_check.
func GitUnavailableMessage(lockStoreDisplay, cause string) string {
	return fmt.Sprintf("cannot tell whether %s is ignored: this directory is inside a git work tree but git could not answer (%s). An approval written here has no way to reach a collaborator except through the repository, so this is a refusal, not a skip. Install git, or fix the repository, and run again.",
		lockStoreDisplay, cause)
}

// OutsideWorkTreeWarning is the warnings[] line every verb carries when the
// build directory resolves above the repository's top level: a ledger there
// reaches no collaborator by any route.
func OutsideWorkTreeWarning(buildDir string) string {
	return fmt.Sprintf("build_dir resolves outside the repository (%s): no approval written under it is carried by the repository", buildDir)
}

func indent(block string) string {
	lines := strings.Split(block, "\n")
	for i, l := range lines {
		lines[i] = "  " + l
	}
	return strings.Join(lines, "\n")
}
