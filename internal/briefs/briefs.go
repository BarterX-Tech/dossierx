// Package briefs is the READ SIDE of briefs (NIT-204, the first half of
// NIT-192): discovering them, parsing them, holding them to their shape and
// their caps, and handing them to the commands and the viewer.
//
// WHAT A BRIEF IS. A brief is a markdown document beside the claims —
// briefs/<folder>/<slug>.md next to project.config.yaml — for the prose a
// project needs that is not a claim: why a feature exists, how a flow reads end
// to end, what a design is trying to be. Not everything is a claim (NIT-178).
// A claim is one reviewable fact; a brief is a document a human reads in one
// sitting and that may rest on claims.
//
// THE COUPLING RULES, which are why this package is a leaf beside the claim
// pipeline rather than a stage of it:
//
//   - A claim never learns about briefs. Nothing in internal/model, the claims
//     loader, the lint registry, the catalog or the lock store imports this
//     package, and no brief field reaches a claim hash.
//   - Briefs never enter `manifest show --isolation` or `--integration`, and
//     never the claims graph.
//   - Brief state never affects claim readiness, locking or review. A brief's
//     findings ride in `check`'s lint_findings so an agent branches on them the
//     way it branches on every other rule, but they are this package's rule set
//     (Rules), not lint.Registry's, and `claim lock` never sees them.
//
// THE LOCK LIFECYCLE (NIT-205) is lockstate.go: a brief's state against its
// record in the lock store, its review-pending state against the rests_on
// baselines that record keeps, and the four findings they raise. write.go is
// the one place a brief file is rewritten — its status line and its comments
// block, never a byte of the body. Discovery itself still writes nothing.
//
// THE SHAPE, in full. briefs_dir (default briefs) is ONE folder level deep:
// briefs/<folder>/ holds <slug>.md files and the images those briefs reference,
// and nothing else. Folder names, brief file names and image file names are all
// drawn from [a-z0-9-] (markdown.DocumentNameStem), which is what gives a brief
// a slash-free id, <folder>.<slug>: the Go 1.22 mux binds one path segment per
// {id} wildcard, so an id that could hold a slash could not be a route, and a
// store keyed by one would need an escaping rule of its own. Refused, each as a
// brief-shape finding: a file directly under briefs/, anything deeper than one
// folder, any other file, a name outside the character set, a non-regular file,
// and an image no brief in its folder references.
//
// ONE EXCEPTION, deliberately: a name beginning with "." is not read at all.
// .DS_Store, an editor's swap file and a .gitkeep are litter the operating
// system and the tools put there, not content anybody authored, and refusing
// them would make `check` fail on a Mac the moment Finder opens the folder.
// serve's watcher skips dot-directories for the same reason.
//
// An ABSENT briefs_dir is a project with no briefs, and that project sees no
// change anywhere: no finding, no payload, no field.
package briefs

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/constitution"
	"github.com/BarterX-Tech/dossierx/internal/lint"
	"github.com/BarterX-Tech/dossierx/internal/model"
	"github.com/BarterX-Tech/dossierx/internal/render/markdown"
)

// Status is a brief's lifecycle state as its frontmatter declares it. Only the
// two values exist; a "locked" brief is approved only when the lock store holds
// a standing record for it (lockstate.go).
type Status string

const (
	StatusDraft  Status = "draft"
	StatusLocked Status = "locked"
)

// briefExt is the one extension a brief file carries, lowercase.
const briefExt = ".md"

// Image is one image a brief references, with the file's size. Present is
// false for a reference to a file its folder does not hold — a brief-shape
// finding, carried here so a reader of the payload sees the same fact.
type Image struct {
	Name    string `json:"name"`
	Bytes   int64  `json:"bytes"`
	Present bool   `json:"present"`
	// Digest is the sha256 of the image file's bytes, hex, when present:
	// what `brief lock` records and brief-content-drift compares (NIT-205).
	// An .svg is text, so its bytes are hashed with CRLF normalized to LF, as
	// a brief's markdown is: under core.autocrlf the working tree holds CRLF
	// and the index LF, and the two must sign the same image. The binary
	// formats are hashed as they are. Not in the payload; the viewer has no
	// use for it.
	Digest string `json:"-"`
	// ContentBytes is the length of the bytes Digest is taken over — Bytes
	// for a binary image, the normalized length for an .svg — and is what
	// brief-image-cap counts, so --validate and --staged reach one verdict on
	// one image. Bytes stays the file's own size: it is what a static build
	// copies and charges to the viewer's bound.
	ContentBytes int64 `json:"-"`
}

// Brief is one parsed brief. Everything on it is derived from the file's own
// bytes and its folder; nothing is read from a store.
type Brief struct {
	// ID is <folder>.<slug>. Slash-free by construction (see the package doc).
	ID string
	// Path is the file's path relative to the config directory, slash-separated
	// — "briefs/checkout/flow.md" — which is what a human types and what every
	// finding about the brief names.
	Path   string
	Folder string
	Slug   string

	// Title is the first level-1 heading RenderDocument would emit, and
	// otherwise the slug title-cased ("checkout-flow" → "Checkout Flow").
	Title   string
	Summary string
	Status  Status
	RestsOn []string

	// Content is the whole file with line endings normalized to LF; Body is
	// the markdown after the frontmatter block. Digest is the SHA-256 of
	// Content, hex-encoded: normalizing first is what makes the digest of one
	// brief the same on a Windows checkout under core.autocrlf as everywhere
	// else.
	Content string
	Body    string
	Digest  string

	// Comments is the brief's review threads (NIT-205), engine-managed and in
	// the claim's comment shape. LockHash is what `brief lock` signs — summary,
	// rests_on and body (see LockHash) — so neither status nor a comment moves
	// it, the way a claim's status and comments never move its hashes.
	Comments []model.Comment
	LockHash string

	// Words is constitution.CountWords — the meter the roof's cap uses — over
	// the text the rendered Body puts on the page (markdown.DocumentText), so
	// image references and link targets are not words. Images is every
	// distinct image Body references, in first-use order.
	Words  int
	Images []Image
}

// Folder is one brief folder and how many briefs it holds. Images are not
// counted: the folder cap counts documents.
type Folder struct {
	Name  string `json:"folder"`
	Count int    `json:"briefs"`
}

// Set is every brief a project holds, plus the findings their discovery
// produced. It is the value every consumer shares — check's rules, the two
// commands, the viewer payload — so that none of them discovers the tree a
// second time with a second idea of what is in it.
type Set struct {
	// DisplayDir is briefs_dir relative to the config directory,
	// slash-separated ("briefs"); findings about the tree itself name it.
	DisplayDir string
	Briefs     []Brief
	Folders    []Folder
	Caps       config.BriefCaps

	findings []lint.Finding
}

// Empty reports whether the project holds no brief. An empty set renders no
// payload and adds no field anywhere.
func (s *Set) Empty() bool { return s == nil || len(s.Briefs) == 0 }

// Lookup resolves the argument `brief show` was given: a brief's id
// (<folder>.<slug>) or its path as `brief list` prints it. Nothing else is
// guessed at — an argument that is neither is not found.
func (s *Set) Lookup(arg string) (Brief, bool) {
	if s == nil {
		return Brief{}, false
	}
	want := strings.TrimPrefix(filepath.ToSlash(arg), "./")
	for _, b := range s.Briefs {
		if b.ID == want || b.Path == want {
			return b, true
		}
	}
	return Brief{}, false
}

// File is one file of a briefs tree, from either source a Set is built from:
// the working tree (Load) or the git index (FromFiles, for check --staged).
// Rel is slash-separated and relative to briefs_dir. Data is the file's bytes
// for a .md and may be nil for anything else; Size is always the file's size.
//
// Regular is false for anything that is not a plain file: a symlink, a device,
// or — in the index only — a gitlink (160000, a submodule or embedded
// repository). Such an entry is still handed to FromFiles, so that it is
// REFUSED under the brief-shape rule rather than silently absent. A submodule
// checkout in the working tree is a directory, not a File: Load reads it as an
// ordinary folder (see Load). Rel "." names briefs_dir itself, for the one case
// where the tree's root is not a directory: a symlink, a gitlink or a file
// where the folder should be.
type File struct {
	Rel     string
	Size    int64
	Regular bool
	Data    []byte
	// Digest is the sha256 of a regular image's bytes, hex — an .svg's with
	// CRLF normalized (see Image.Digest) — when the reader hashed it without
	// keeping the bytes (Load streams images), and ContentSize is the length
	// it was taken over; the index reader hands the bytes in Data instead,
	// and FromFiles hashes them.
	Digest      string
	ContentSize int64
}

// Load discovers the briefs tree on disk at cfg.BriefsDirPath(). A directory
// that does not exist is an empty Set. A tree that cannot be read is reported
// as a brief-shape finding naming the error rather than returned: a caller's
// verdict must not be "no briefs" when the truth is "could not look".
//
// The root is read with Lstat, not Stat: a briefs_dir that is a SYMLINK is
// refused (brief-shape on the tree), for the reason a symlinked brief is. Stat
// followed the link and WalkDir then declined to descend a symlinked root, so a
// linked tree read as a project with no briefs at all.
//
// A directory holding a .git entry — a submodule checkout, an embedded
// repository, briefs_dir itself being one, or a stray .git file or directory —
// is read like any other directory: its .git entry is a dot-name and is not
// read, and everything else in it is judged as usual. Load does not ask
// whether git would stage that directory as a gitlink. `check --staged` is the
// stricter mode here: it refuses a gitlink (160000) entry under briefs_dir, so
// a submodule or embedded repository can never be committed as a brief folder
// (see stagedBriefs in internal/check).
func Load(cfg *config.Config) *Set { return load(cfg, true) }

// LoadListing is Load without reading a byte of any image: the same walk and
// the same refusals, every brief parsed, every image Present or not, but no
// Digest, and an image's ContentBytes is its file size. It is for a caller
// that needs only which images the briefs reference — serve's brief-asset
// route, which answers one image per request and must not hash every image
// in the tree to do it. Its findings are not a verdict (an .svg's cap is
// counted on the file's size, and an unreadable image is not noticed), so
// nothing that judges or locks a brief reads it.
func LoadListing(cfg *config.Config) *Set { return load(cfg, false) }

func load(cfg *config.Config, digests bool) *Set {
	dir := cfg.BriefsDirPath()
	info, err := os.Lstat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return newSet(cfg)
	}
	if err != nil {
		s := newSet(cfg)
		s.add(RuleShape, dirPath(s.DisplayDir), "briefs_dir could not be read: %v", readErrText(err))
		return s
	}
	if !info.IsDir() {
		return FromFiles(cfg, []File{{Rel: ".", Regular: info.Mode().IsRegular(), Size: info.Size()}})
	}
	// An entry that cannot be read below the root is ONE finding on that
	// entry, and the walk goes on: an unreadable folder must not drop every
	// other folder's briefs (which made the whole project read as holding
	// none). Only an unreadable root is the whole-tree finding below.
	var files []File
	type unreadable struct {
		display string
		err     error
	}
	var unread []unreadable
	skip := func(p string, d fs.DirEntry, err error) error {
		rel, relErr := filepath.Rel(dir, p)
		if relErr != nil {
			return relErr
		}
		display := path.Join(displayDir(cfg), filepath.ToSlash(rel))
		if d != nil && d.IsDir() {
			display += "/"
		}
		unread = append(unread, unreadable{display, err})
		if d != nil && d.IsDir() {
			return filepath.SkipDir
		}
		return nil
	}
	walkErr := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if p == dir {
				return err
			}
			if strings.HasPrefix(filepath.Base(p), ".") {
				return nil
			}
			return skip(p, d, err)
		}
		if p == dir {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(dir, p)
		if relErr != nil {
			return relErr
		}
		f := File{Rel: filepath.ToSlash(rel), Regular: d.Type().IsRegular()}
		if !f.Regular {
			files = append(files, f)
			return nil
		}
		fi, statErr := d.Info()
		if statErr != nil {
			return skip(p, d, statErr)
		}
		f.Size = fi.Size()
		if path.Ext(f.Rel) == briefExt {
			raw, readErr := os.ReadFile(p)
			if readErr != nil {
				return skip(p, d, readErr)
			}
			f.Data = raw
		} else if digests && markdown.IsDocumentImageExt(path.Ext(f.Rel)) {
			// An image is signed by a brief's lock (its sha256 on the
			// record), so it is hashed here — streamed, never held.
			digest, n, hashErr := fileDigest(p, textImage(f.Rel))
			if hashErr != nil {
				return skip(p, d, hashErr)
			}
			f.Digest, f.ContentSize = digest, n
		}
		files = append(files, f)
		return nil
	})
	if walkErr != nil {
		s := newSet(cfg)
		s.add(RuleShape, dirPath(s.DisplayDir), "briefs_dir could not be read: %v", readErrText(walkErr))
		return s
	}
	s := FromFiles(cfg, files)
	for _, u := range unread {
		s.add(RuleShape, u.display, "could not be read (%v); a brief the engine cannot read is not judged, so this is refused rather than skipped", readErrText(u.err))
	}
	return s
}

// fileDigest is the sha256 of a file's bytes, hex, streamed, and the number
// of bytes hashed. text normalizes CRLF to LF on the way through, exactly as
// imageContent normalizes the index's copy.
func fileDigest(p string, text bool) (string, int64, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	if !text {
		n, err := io.Copy(h, f)
		if err != nil {
			return "", 0, err
		}
		return hex.EncodeToString(h.Sum(nil)), n, nil
	}
	w := &crlfWriter{w: h}
	if _, err := io.Copy(w, f); err != nil {
		return "", 0, err
	}
	if err := w.flush(); err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), w.n, nil
}

// textImage reports whether an image file is text, and so hashed with its
// line endings normalized: .svg, the one text format of the six.
func textImage(name string) bool { return path.Ext(name) == ".svg" }

// imageContent is an image file's Digest and ContentBytes, from whichever
// form the reader handed over: Load's streamed digest, or the index's bytes,
// hashed here under the same normalization. A File with neither (a test's
// size-only fixture) has no digest, and its size is its content.
func imageContent(f File) (string, int64) {
	if f.Digest != "" {
		return f.Digest, f.ContentSize
	}
	if f.Data == nil {
		return "", f.Size
	}
	data := f.Data
	if textImage(f.Rel) {
		data = normalizeLineEndings(data)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), int64(len(data))
}

// crlfWriter is normalizeLineEndings as a stream: it writes what it is given
// with each CRLF as LF, holding a CR that ends one chunk until the next shows
// whether an LF follows. n counts the bytes written through.
type crlfWriter struct {
	w  io.Writer
	cr bool
	n  int64
}

func (c *crlfWriter) Write(p []byte) (int, error) {
	out := make([]byte, 0, len(p)+1)
	for _, b := range p {
		if c.cr && b != '\n' {
			out = append(out, '\r')
		}
		c.cr = b == '\r'
		if !c.cr {
			out = append(out, b)
		}
	}
	c.n += int64(len(out))
	if _, err := c.w.Write(out); err != nil {
		return 0, err
	}
	return len(p), nil
}

// flush writes a CR the input ended on.
func (c *crlfWriter) flush() error {
	if !c.cr {
		return nil
	}
	c.cr = false
	c.n++
	_, err := c.w.Write([]byte{'\r'})
	return err
}

// ImageDigests is the sha256 of every present image b references, by name —
// what `brief lock` records beside the lock hash.
func (b Brief) ImageDigests() map[string]string {
	out := map[string]string{}
	for _, img := range b.Images {
		if img.Present {
			out[img.Name] = img.Digest
		}
	}
	return out
}

// readErrText is an I/O error without the absolute path a *fs.PathError
// carries: the finding already names the path, relative to the config
// directory, as every brief finding does.
func readErrText(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}

// FromFiles builds a Set from an already-enumerated tree. It is Load's second
// half, and it is exported for check --staged, which reads the tree out of the
// git index and must judge it under exactly the rules the working tree gets.
func FromFiles(cfg *config.Config, files []File) *Set {
	s := newSet(cfg)
	sort.Slice(files, func(i, j int) bool { return files[i].Rel < files[j].Rel })

	type folderFiles struct {
		briefs []File
		images map[string]File
	}
	folders := map[string]*folderFiles{}
	var folderNames []string
	folderOf := func(name string) *folderFiles {
		f, ok := folders[name]
		if !ok {
			f = &folderFiles{images: map[string]File{}}
			folders[name] = f
			folderNames = append(folderNames, name)
		}
		return f
	}

	for _, f := range files {
		if f.Rel == "." {
			// briefs_dir itself is not a directory (see File).
			if f.Regular {
				s.add(RuleShape, dirPath(s.DisplayDir), "briefs_dir is a file, not a directory; briefs live in %s/<folder>/<slug>.md", s.DisplayDir)
			} else {
				s.add(RuleShape, dirPath(s.DisplayDir), "briefs_dir is a symlink or a submodule, not a directory; briefs are read from a plain directory only, so a linked tree is refused rather than read as no briefs")
			}
			continue
		}
		segs := strings.Split(f.Rel, "/")
		if hiddenPath(segs) {
			continue
		}
		display := path.Join(s.DisplayDir, f.Rel)
		switch {
		case len(segs) == 1 && !f.Regular:
			// It stands where only a folder may, so the finding is on that
			// folder and spelled as every folder-level finding is.
			display = dirPath(display)
			s.add(RuleShape, display, "%s is a symlink or a submodule; directly under %s/ the briefs tree holds plain folders only, and a brief folder is a plain directory holding plain files", display, s.DisplayDir)
			continue
		case len(segs) == 1:
			s.add(RuleShape, display, "a file directly under %s/ is not a brief; briefs live one folder down, as %s/<folder>/<slug>.md", s.DisplayDir, s.DisplayDir)
			continue
		case len(segs) > 2:
			s.add(RuleShape, display, "%s is deeper than one folder; a brief folder holds its briefs and their images directly, with no subdirectory", display)
			continue
		}
		folder, name := segs[0], segs[1]
		if !markdown.DocumentNameStem(folder) {
			s.add(RuleShape, display, "folder name %q is outside [a-z0-9-]; a brief's id is <folder>.<slug>, so both names are held to that set", folder)
			continue
		}
		if !f.Regular {
			s.add(RuleShape, display, "%s is not a regular file (a symlink, a submodule or a device); a brief folder holds plain files only", display)
			continue
		}
		ext := path.Ext(name)
		stem := strings.TrimSuffix(name, ext)
		switch {
		case ext == briefExt:
			if !markdown.DocumentNameStem(stem) {
				s.add(RuleShape, display, "brief file name %q is outside [a-z0-9-]; a brief's id is <folder>.<slug>, so the slug is held to that set", name)
				continue
			}
			ff := folderOf(folder)
			ff.briefs = append(ff.briefs, f)
		case markdown.IsDocumentImageExt(ext):
			if !markdown.DocumentNameStem(stem) {
				s.add(RuleShape, display, "image file name %q is outside [a-z0-9-]; a brief references an image by that name, and the name rule is the reference rule", name)
				continue
			}
			folderOf(folder).images[name] = f
		default:
			s.add(RuleShape, display, "%s is not a brief or an image; a brief folder holds <slug>.md files and the .png/.jpg/.jpeg/.gif/.webp/.svg images they reference, and nothing else", display)
		}
	}

	sort.Strings(folderNames)
	referenced := map[string]bool{}
	for _, folder := range folderNames {
		ff := folders[folder]
		for _, f := range ff.briefs {
			b := s.parse(folder, f)
			for i, img := range b.Images {
				file, ok := ff.images[img.Name]
				b.Images[i].Present = ok
				b.Images[i].Bytes = file.Size
				if ok {
					b.Images[i].Digest, b.Images[i].ContentBytes = imageContent(file)
				}
				if !ok {
					s.add(RuleShape, b.Path, "references image %q, which is not in %s/%s/; a brief's images sit beside it in its own folder", img.Name, s.DisplayDir, folder)
				}
				referenced[path.Join(folder, img.Name)] = true
			}
			s.Briefs = append(s.Briefs, b)
		}
		if len(ff.briefs) > 0 {
			s.Folders = append(s.Folders, Folder{Name: folder, Count: len(ff.briefs)})
		}
		imageNames := make([]string, 0, len(ff.images))
		for name := range ff.images {
			imageNames = append(imageNames, name)
		}
		sort.Strings(imageNames)
		for _, name := range imageNames {
			if !referenced[path.Join(folder, name)] {
				s.add(RuleShape, path.Join(s.DisplayDir, folder, name), "no brief in %s/%s/ references this image; a brief folder holds only the images its briefs use, so reference it with ![alt](%s) or delete it", s.DisplayDir, folder, name)
			}
		}
	}
	sort.Slice(s.Briefs, func(i, j int) bool { return s.Briefs[i].ID < s.Briefs[j].ID })
	s.checkCaps()
	return s
}

// dirPath is a directory's display path: slash-joined, with the trailing slash
// that tells a reader a finding is about a folder or the tree and not a file.
func dirPath(parts ...string) string {
	return fmt.Sprintf("%s/", path.Join(parts...))
}

// hiddenPath reports whether any segment of a tree-relative path begins with
// ".", the one exclusion the package doc names.
func hiddenPath(segs []string) bool {
	for _, seg := range segs {
		if strings.HasPrefix(seg, ".") {
			return true
		}
	}
	return false
}

func newSet(cfg *config.Config) *Set {
	return &Set{DisplayDir: displayDir(cfg), Caps: cfg.BriefCapLimits()}
}

// displayDir is briefs_dir as a reader sees it from the config directory. A
// briefs_dir outside that directory keeps its "../" form: it is still the
// honest relative path.
func displayDir(cfg *config.Config) string {
	dir := cfg.BriefsDirPath()
	if dir == "" {
		return config.DefaultBriefsDir
	}
	if rel, err := filepath.Rel(cfg.Dir(), dir); err == nil && cfg.Dir() != "" {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(dir)
}

// parse builds one Brief from its file. A frontmatter defect is a finding, not
// a missing brief: the brief is still listed, shown and counted against its
// folder, with whatever the frontmatter did yield, so the author sees the file
// they wrote and the rule it broke side by side.
func (s *Set) parse(folder string, f File) Brief {
	slug := strings.TrimSuffix(path.Base(f.Rel), briefExt)
	content := string(normalizeLineEndings(f.Data))
	sum := sha256.Sum256([]byte(content))
	b := Brief{
		ID:      folder + "." + slug,
		Path:    path.Join(s.DisplayDir, f.Rel),
		Folder:  folder,
		Slug:    slug,
		Status:  StatusDraft,
		Content: content,
		Digest:  hex.EncodeToString(sum[:]),
	}
	fm, body, problems := parseFrontmatter(content)
	for _, p := range problems {
		s.add(RuleFrontmatter, b.Path, "%s", p)
	}
	b.Summary, b.RestsOn, b.Body, b.Comments = fm.summary, fm.restsOn, body, fm.comments
	b.LockHash = LockHash(b.Summary, b.RestsOn, b.Body)
	if fm.status != "" {
		b.Status = Status(fm.status)
	}
	if title, ok := markdown.DocumentTitle(body); ok {
		b.Title = title
	} else {
		b.Title = titleCase(slug)
	}
	b.Words = constitution.CountWords(markdown.DocumentText(body))
	seen := map[string]bool{}
	for _, name := range markdown.DocumentImages(body) {
		if seen[name] {
			continue
		}
		seen[name] = true
		b.Images = append(b.Images, Image{Name: name})
	}
	refused := map[string]bool{}
	for _, src := range markdown.DocumentRefusedImages(body) {
		if refused[src] {
			continue
		}
		refused[src] = true
		s.add(RuleShape, b.Path, "image %q is not one a brief can show, so it renders as literal text; a brief references an image by its bare file name in its own folder — [a-z0-9-] and a lowercase .png/.jpg/.jpeg/.gif/.webp/.svg extension, as ![alt](flow-diagram.svg)", src)
	}
	return b
}

// LockHash is the hash `brief lock` records and brief-content-drift compares:
// the summary, the rests_on set and the body — everything a reader of the
// brief reads. Status and comments are left out on purpose, as a claim's are:
// locking flips status, and a review thread is about the brief, not part of
// it. rests_on is hashed as a set (sorted), because its order carries no
// meaning. Each field is length-prefixed so no two different briefs can
// concatenate to the same input.
//
// Images a brief references are not in this hash: it covers the markdown file.
// `brief lock` records each image's Digest beside it, and brief-content-drift
// compares those separately.
func LockHash(summary string, restsOn []string, body string) string {
	ids := append([]string(nil), restsOn...)
	sort.Strings(ids)
	h := sha256.New()
	fmt.Fprintf(h, "dossierx-brief-lock/v1\nsummary=%d:%s\nrests_on=%d\n", len(summary), summary, len(ids))
	for _, id := range ids {
		fmt.Fprintf(h, "id=%d:%s\n", len(id), id)
	}
	fmt.Fprintf(h, "body=%d:%s\n", len(body), body)
	return hex.EncodeToString(h.Sum(nil))
}

// FilePath is the brief's file on disk: briefs_dir/<folder>/<slug>.md.
func FilePath(cfg *config.Config, b Brief) string {
	return filepath.Join(cfg.BriefsDirPath(), b.Folder, b.Slug+briefExt)
}

// OpenThreads is how many of the brief's comment threads are unresolved.
func (b Brief) OpenThreads() int {
	n := 0
	for _, c := range b.Comments {
		if c.Status == model.CommentStatusOpen {
			n++
		}
	}
	return n
}

// titleCase turns a slug into the fallback title: each hyphen-separated word
// capitalized, joined by spaces.
func titleCase(slug string) string {
	words := strings.FieldsFunc(slug, func(r rune) bool { return r == '-' })
	for i, w := range words {
		r := []rune(w)
		r[0] = unicode.ToUpper(r[0])
		words[i] = string(r)
	}
	if len(words) == 0 {
		return slug
	}
	return strings.Join(words, " ")
}

// normalizeLineEndings turns CRLF into LF and touches nothing else — the one
// transformation core.autocrlf performs on checkout, and so the one difference
// between two copies of a brief that is not a difference in the brief. A lone
// CR is left alone, as internal/check's identical helper leaves it.
func normalizeLineEndings(b []byte) []byte {
	return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n"))
}

// add records one finding. claimID is the path a reader opens — see Findings
// for why a brief finding's claim_id is a path.
func (s *Set) add(rule, claimID, format string, args ...any) {
	s.findings = append(s.findings, lint.Finding{
		LintName: rule,
		ClaimID:  claimID,
		Severity: severityOf(rule),
		Message:  fmt.Sprintf(format, args...),
	})
}

// init gives source-internal-drift its brief pin (NIT-198): an internal
// source citing a brief pins the brief's LockHash — summary, rests_on and
// body — so a status flip or a comment thread written into the brief's
// frontmatter never reads as drift under the claim that cites it. The file
// is parsed exactly as discovery parses it (FromFiles), so the value is the
// content_hash `brief show` prints and `brief lock` signs.
func init() {
	lint.BriefContentHash = func(cfg *config.Config, rel string, data []byte) (string, bool) {
		set := FromFiles(cfg, []File{{Rel: rel, Size: int64(len(data)), Regular: true, Data: data}})
		if len(set.Briefs) != 1 {
			return "", false
		}
		return set.Briefs[0].LockHash, true
	}
}
