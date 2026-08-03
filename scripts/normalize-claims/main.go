// Command normalize-claims rewrites the committed claim fixtures through the
// engine's own writer, so the authored corpus and internal/loader's emitter
// agree byte-for-byte.
//
// It exists because internal/loader's MUTATE path re-emits a claim file's whole
// document (block-scalar indent width normalised to claimYAMLIndent, keys the
// struct no longer emits dropped, keys it emits appended). A fixture authored in
// a style the emitter does not reproduce therefore takes a whole-file diff on
// the first ordinary write — lock, unlock, reaudit, a single new comment — which
// is exactly the noise the minimal-diff work removed. Normalising the corpus
// once, in its own announced commit, means every later diff on these files is
// the change itself.
//
// Walk mode:
//
//	go run ./scripts/normalize-claims testdata
//
// walks every *.yaml under a claims/ directory beneath the given root, and for
// each one calls loader.LoadClaims on its directory and loader.SaveClaim for the
// claim whose SourcePath is that file. project.config.yaml is excluded by the
// pattern; testdata/markdown-cases/ and testdata/markdown-claim-body-cases/ are
// not under a claims/ directory and are deliberately left alone (they are
// renderer fixtures, not claims). Nothing else is written and there are no
// other flags.
//
// Compare mode:
//
//	go run ./scripts/normalize-claims -compare <git-rev> <file>
//
// is the semantic half of the gate: it decodes <git-rev>:<file> and the working
// copy of <file> through loader.LoadClaims into model.Claim and reflect.DeepEqual
// them, printing "equal" or "DIFFERENT". Both sides go through the real decoder
// on purpose. A raw byte diff would flag pure style, and a yaml.Unmarshal into
// `any` is worse than useless here: fixture-coverage/lint/comments-unresolved
// carries an unquoted `created: 2026-07-25T00:00:00Z`, which normalisation
// rewrites as a quoted string; decoded into `any` those two are time.Time vs
// string and compare unequal, a false positive on a change that is not a value
// change. Decoded into model.Claim (where the field is a string) they are the
// same value, which is the question actually being asked.
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/BarterX-Tech/dossierx/internal/loader"
	"github.com/BarterX-Tech/dossierx/internal/model"
)

// excluded lists claim files the walk must skip, keyed by slash-separated path
// suffix. Each entry is a fixture whose whole purpose is a shape the emitter
// cannot reproduce, so normalising it would be a genuine VALUE change rather
// than a formatting one.
var excluded = map[string]string{
	// This fixture omits governed_by entirely, which is the condition the
	// governed-required lint fires on. model.Claim.Governed has no omitempty,
	// so a normalising save APPENDS `governed_by:\n  type: ""` and the lint
	// stops seeing the case it was written to exercise. That is a value
	// change, not a reformat, so the file stays authored as-is.
	"testdata/fixture-coverage/lint/governed-required/claims/missing-type.yaml": "omits governed_by on purpose; normalising would append an empty governed_by and defeat the governed-required lint fixture",
}

func main() {
	args := os.Args[1:]
	if len(args) == 3 && args[0] == "-compare" {
		if err := compare(args[1], args[2]); err != nil {
			fmt.Fprintf(os.Stderr, "normalize-claims: %v\n", err)
			os.Exit(1)
		}
		return
	}
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "usage: normalize-claims <root>\n       normalize-claims -compare <git-rev> <file>\n")
		os.Exit(2)
	}
	if err := normalize(args[0]); err != nil {
		fmt.Fprintf(os.Stderr, "normalize-claims: %v\n", err)
		os.Exit(1)
	}
}

// isExcluded reports whether path is on the skip list, and why.
func isExcluded(path string) (string, bool) {
	slash := filepath.ToSlash(filepath.Clean(path))
	for suffix, why := range excluded {
		if slash == suffix || strings.HasSuffix(slash, "/"+suffix) {
			return why, true
		}
	}
	return "", false
}

// claimFiles returns every *.yaml under a claims/ directory beneath root, in
// sorted order, alongside the ones the skip list removed.
func claimFiles(root string) (files, skipped []string, err error) {
	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".yaml" {
			return nil
		}
		underClaims := false
		for _, part := range strings.Split(filepath.ToSlash(filepath.Dir(path)), "/") {
			if part == "claims" {
				underClaims = true
				break
			}
		}
		if !underClaims {
			return nil
		}
		if why, yes := isExcluded(path); yes {
			skipped = append(skipped, fmt.Sprintf("%s (%s)", filepath.ToSlash(path), why))
			return nil
		}
		files = append(files, filepath.Clean(path))
		return nil
	})
	sort.Strings(files)
	sort.Strings(skipped)
	return files, skipped, err
}

// normalize walks root and rewrites each claim file through the engine's writer,
// reporting the files whose bytes actually changed.
func normalize(root string) error {
	files, skipped, err := claimFiles(root)
	if err != nil {
		return err
	}

	// LoadClaims works on a directory, so load each claims dir once and reuse
	// it for every file in it.
	loaded := map[string][]model.Claim{}
	var rewritten []string

	for _, file := range files {
		dir := filepath.Dir(file)
		claims, ok := loaded[dir]
		if !ok {
			claims, err = loader.LoadClaims(dir)
			if err != nil {
				return fmt.Errorf("load %s: %w", dir, err)
			}
			loaded[dir] = claims
		}

		idx := -1
		for i := range claims {
			if filepath.Clean(claims[i].SourcePath) == file {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("%s: loaded %s but no claim reported it as its source path", file, dir)
		}

		before, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if err := loader.SaveClaim(claims[idx]); err != nil {
			return fmt.Errorf("save %s: %w", file, err)
		}
		after, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if !bytes.Equal(before, after) {
			rewritten = append(rewritten, filepath.ToSlash(file))
		}
	}

	for _, f := range rewritten {
		fmt.Println("rewrote", f)
	}
	for _, s := range skipped {
		fmt.Println("excluded", s)
	}
	fmt.Printf("%d of %d claim files rewritten (%d excluded)\n", len(rewritten), len(files), len(skipped))
	return nil
}

// compare decodes rev:path and the working copy of path into model.Claim and
// reports whether they carry the same value.
func compare(rev, path string) error {
	old, err := exec.Command("git", "show", rev+":"+filepath.ToSlash(filepath.Clean(path))).Output()
	if err != nil {
		return fmt.Errorf("git show %s:%s: %w", rev, path, err)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	before, err := decodeClaim(old)
	if err != nil {
		return fmt.Errorf("%s at %s: %w", path, rev, err)
	}
	after, err := decodeClaim(current)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	if reflect.DeepEqual(before, after) {
		fmt.Printf("equal %s\n", filepath.ToSlash(path))
		return nil
	}
	fmt.Printf("DIFFERENT %s\n", filepath.ToSlash(path))
	os.Exit(1)
	return nil
}

// decodeClaim runs raw through the real LoadClaims (the decoder every command
// uses) by dropping it into a scratch directory of its own, and clears
// SourcePath so the two sides of a comparison are not distinguished by the
// temp path they were read from.
func decodeClaim(raw []byte) (model.Claim, error) {
	dir, err := os.MkdirTemp("", "normalize-claims-compare")
	if err != nil {
		return model.Claim{}, err
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "claim.yaml"), raw, 0o644); err != nil {
		return model.Claim{}, err
	}
	claims, err := loader.LoadClaims(dir)
	if err != nil {
		return model.Claim{}, err
	}
	if len(claims) != 1 {
		return model.Claim{}, fmt.Errorf("expected exactly one claim, decoded %d", len(claims))
	}
	claims[0].SourcePath = ""
	return claims[0], nil
}
