// Package loader reads claim YAML files off disk (from a project's
// configured claims_dir) into model.Claim values, and writes individual
// claims back to their source file. It is the one place in the engine that
// touches the filesystem for claim content; every other package works
// purely in memory against []model.Claim.
package loader

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/BarterX-Tech/dossierx/internal/model"
)

// LoadClaims recursively reads every *.yaml/*.yml file under dir, strictly
// decoding each into a model.Claim (unknown fields are a hard error, same
// discipline as internal/config). Each claim's SourcePath is set to the
// file it was loaded from. The result is sorted by SourcePath so callers
// get deterministic ordering regardless of directory-walk order.
//
// A dir that does not exist is a hard error: claims_dir is required
// project configuration, not an optional feature.
func LoadClaims(dir string) ([]model.Claim, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("loader: claims_dir %q: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("loader: claims_dir %q is not a directory", dir)
	}

	var claims []model.Claim
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("loader: read %s: %w", path, err)
		}

		var c model.Claim
		dec := yaml.NewDecoder(strings.NewReader(string(raw)))
		dec.KnownFields(true)
		if err := dec.Decode(&c); err != nil {
			return fmt.Errorf("loader: parse %s: %w", path, err)
		}
		c.SourcePath = path
		claims = append(claims, c)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}

	sort.Slice(claims, func(i, j int) bool { return claims[i].SourcePath < claims[j].SourcePath })
	return claims, nil
}

// SaveClaim writes c back to its SourcePath as YAML. It is used by the
// lock/unlock/reaudit-apply flows, which are the only paths that mutate a
// claim's on-disk representation.
//
// The write is atomic (temp file in the same directory, then rename) rather
// than a direct os.WriteFile. os.WriteFile truncates the destination before
// writing its new bytes, leaving a window where the file is empty or
// partially written; a concurrent LoadClaims (every "docs lock" invocation
// starts by loading the *entire* claims_dir, including files other
// in-flight processes are saving) can land its read inside that window and
// see a truncated file, failing YAML decode with a bare EOF. Writing to a
// sibling temp file and renaming it into place means any concurrent reader
// only ever observes the old complete file or the new complete file, never
// a partial one — os.Rename is atomic within a single filesystem/directory.
func SaveClaim(c model.Claim) error {
	if strings.TrimSpace(c.SourcePath) == "" {
		return fmt.Errorf("loader: claim %q has no source path to save to", c.ID)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("loader: marshal claim %q: %w", c.ID, err)
	}
	if err := atomicWriteFile(c.SourcePath, data, 0o644); err != nil {
		return fmt.Errorf("loader: write %s: %w", c.SourcePath, err)
	}
	return nil
}

// atomicWriteFile writes data to path without ever leaving a reader able to
// observe a partially-written file: it writes to a temp file created in
// path's own directory (so the later rename stays on one filesystem, which
// is what makes it atomic) and then renames it over path.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

// FindByID returns the claim with the given id, if present.
func FindByID(claims []model.Claim, id string) (model.Claim, bool) {
	for _, c := range claims {
		if c.ID == id {
			return c, true
		}
	}
	return model.Claim{}, false
}
