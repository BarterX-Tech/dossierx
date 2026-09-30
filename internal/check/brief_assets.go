package check

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/BarterX-Tech/dossierx/internal/atomicfile"
	"github.com/BarterX-Tech/dossierx/internal/briefs"
	"github.com/BarterX-Tech/dossierx/internal/config"
	"github.com/BarterX-Tech/dossierx/internal/render"
)

// writeBriefAssets puts every image a brief references beside the static
// viewer, at viewerDir/brief-assets/<folder>/<name> — the relative src the
// page carries (render.BriefAssetDir). `dossierx serve` answers that same path
// from the brief's own folder; a static build has no server, so the files
// have to be there. The set copied is render.BriefAssets, the list the render
// charged against the viewer's byte budget, so what is copied is what was
// counted.
//
// The directory is engine output and is rebuilt whole on every write: an
// image a brief stopped referencing, or a brief that was deleted, leaves
// nothing behind. A project with no brief image removes the directory and
// writes nothing.
//
// Every source is read as a plain file and nothing else. discovery (briefs.Load)
// already refuses a symlinked or otherwise non-regular entry, but the tree can
// change between discovery and this copy, so each file is checked again here:
// Lstat must say regular, the opened handle must be that same file, and it
// must hold exactly the bytes that were charged. Anything else fails the
// write with the path named, rather than copying through a link to a file
// outside the project or writing a viewer whose size was never counted.
func writeBriefAssets(cfg *config.Config, set *briefs.Set, viewerDir string) error {
	dest := filepath.Join(viewerDir, render.BriefAssetDir)
	if err := os.RemoveAll(dest); err != nil {
		return fmt.Errorf("clear %s: %w", dest, err)
	}
	for _, a := range render.BriefAssets(cfg, set) {
		data, err := readBriefAsset(a)
		if err != nil {
			return err
		}
		out := filepath.Join(viewerDir, filepath.FromSlash(a.Rel))
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return fmt.Errorf("create %s: %w", filepath.Dir(out), err)
		}
		if err := atomicfile.Write(out, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", out, err)
		}
	}
	return nil
}

// errBriefAssetChanged is the refusal for a source that is no longer the
// plain file of the size discovery read.
var errBriefAssetChanged = errors.New("is not the plain file of the size check read; re-run dossierx check")

func readBriefAsset(a render.BriefAsset) ([]byte, error) {
	// The folder and briefs_dir itself are held to the same rule discovery
	// holds them to — a plain directory, never a link — so a folder swapped
	// for a symlink after discovery cannot route the copy outside the tree.
	folder := filepath.Dir(a.Src)
	for _, dir := range []string{filepath.Dir(folder), folder} {
		info, err := os.Lstat(dir)
		if err != nil {
			return nil, fmt.Errorf("brief image %s: %w", a.Display, unwrapPathError(err))
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("brief image %s %w", a.Display, errBriefAssetChanged)
		}
	}
	before, err := os.Lstat(a.Src)
	if err != nil {
		return nil, fmt.Errorf("brief image %s: %w", a.Display, unwrapPathError(err))
	}
	if !before.Mode().IsRegular() || before.Size() != a.Bytes {
		return nil, fmt.Errorf("brief image %s %w", a.Display, errBriefAssetChanged)
	}
	f, err := os.Open(a.Src)
	if err != nil {
		return nil, fmt.Errorf("brief image %s: %w", a.Display, unwrapPathError(err))
	}
	defer f.Close() //nolint:errcheck // read-only handle
	after, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("brief image %s: %w", a.Display, unwrapPathError(err))
	}
	if !os.SameFile(before, after) {
		return nil, fmt.Errorf("brief image %s %w", a.Display, errBriefAssetChanged)
	}
	data, err := io.ReadAll(io.LimitReader(f, a.Bytes+1))
	if err != nil {
		return nil, fmt.Errorf("brief image %s: %w", a.Display, unwrapPathError(err))
	}
	if int64(len(data)) != a.Bytes {
		return nil, fmt.Errorf("brief image %s %w", a.Display, errBriefAssetChanged)
	}
	return data, nil
}

// unwrapPathError drops the *fs.PathError wrapper, whose path the message
// already names.
func unwrapPathError(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}
