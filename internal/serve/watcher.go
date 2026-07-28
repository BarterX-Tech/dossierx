package serve

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
)

const (
	// defaultPollInterval is how often the watcher re-walks the claims tree
	// looking for a change. serve deliberately uses a zero-dependency mtime poll
	// rather than fsnotify, so go.mod stays cobra + yaml.v3 only; ~500ms is
	// imperceptible for live reload yet cheap over a claims tree of dozens of
	// small files.
	defaultPollInterval = 500 * time.Millisecond

	// defaultDebounceInterval is the trailing-debounce window that collapses a
	// burst of writes — a multi-file save, a git checkout, or a comment
	// mutation's own file rewrite — into a single "changed": the watcher waits
	// this much quiet after the last detected delta before it notifies.
	defaultDebounceInterval = 200 * time.Millisecond
)

// watcher polls a directory tree on a fixed interval, fingerprints every
// relevant claim file (path + mtime + size), and invokes onChange once per
// debounced burst of changes. It is the SINGLE change-detection path for serve:
// an external editor writing a claim and a comment mutation rewriting one are
// both observed identically, as a fingerprint delta, so create, modify, delete,
// and rename all read as "changed" with no per-event bookkeeping and no fsnotify
// dependency.
type watcher struct {
	root     string
	poll     time.Duration
	debounce time.Duration
	onChange func()
}

func newWatcher(root string, poll, debounce time.Duration, onChange func()) *watcher {
	return &watcher{root: root, poll: poll, debounce: debounce, onChange: onChange}
}

// run polls until ctx is cancelled, starting from the given baseline
// fingerprint (captured synchronously by the caller before serving, so a change
// that lands between server start and the first poll is still seen). On each
// tick it re-fingerprints; a delta (re)arms a FRESH debounce timer, and onChange
// fires only after the debounce elapses with no further delta — collapsing a
// burst into one notification. A fresh timer per delta (rather than Reset)
// sidesteps the drain-before-reset hazard entirely.
func (w *watcher) run(ctx context.Context, baseline map[string]fileStamp) {
	prev := baseline
	ticker := time.NewTicker(w.poll)
	defer ticker.Stop()

	var debounce *time.Timer
	var debounceC <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			if debounce != nil {
				debounce.Stop()
			}
			return
		case <-ticker.C:
			cur, err := scanFingerprint(w.root)
			if err != nil {
				// A transient walk error (the tree briefly gone, a file racing a
				// rename) is not fatal to a long-lived watcher: keep the prior
				// fingerprint and retry on the next tick.
				continue
			}
			if !fingerprintsEqual(prev, cur) {
				prev = cur
				if debounce != nil {
					debounce.Stop()
				}
				debounce = time.NewTimer(w.debounce)
				debounceC = debounce.C
			}
		case <-debounceC:
			// Debounce elapsed with no newer delta: one collapsed notification.
			debounce = nil
			debounceC = nil
			w.onChange()
		}
	}
}

// fileStamp is one claim file's contribution to a tree fingerprint: its
// modification time (nanoseconds) and size. Any create/modify/delete/rename of a
// relevant file changes the set of stamps, which is all "changed" needs.
//
// KNOWN LIMIT, and the price of a zero-dependency poll: an edit that changes
// neither the size nor the recorded modification time is invisible. That needs
// two writes of identical length, close enough together to land inside the
// filesystem's timestamp granularity — sub-millisecond on APFS and ext4, but
// tens of milliseconds on Windows. A person editing a claim in an editor never
// hits it; a program rewriting the same file twice in one tick can. The cost is
// one missed live-reload in the viewer, which a page refresh fixes, so it does
// not justify hashing every file on every 500ms tick. Nothing in the integrity
// gate depends on this — check re-reads the tree itself.
type fileStamp struct {
	modNano int64
	size    int64
}

// scanFingerprint walks root exactly as loader.LoadClaims does — recursively,
// matching *.yaml/*.yml — and records a stamp per file. It excludes precisely
// what the watcher must never react to: dot-directories (never claim homes) and
// the atomic writer's transient "<name>.tmp-<rand>" scratch files, which appear
// and vanish around every SaveClaim and would otherwise flap the fingerprint.
func scanFingerprint(root string) (map[string]fileStamp, error) {
	fp := make(map[string]fileStamp)
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip dot-directories wholesale, but never the root itself.
			if path != root && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if ignoredClaimFile(d.Name()) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			// The file vanished between enumeration and stat (e.g. a temp file
			// mid-rename); treat it as absent rather than failing the whole scan.
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		fp[path] = fileStamp{modNano: info.ModTime().UnixNano(), size: info.Size()}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return fp, nil
}

// ignoredClaimFile reports whether name is a file the claims fingerprint must
// skip: anything that is not a *.yaml/*.yml claim, and the atomic writer's
// "*.tmp-*" scratch files. A real claim is "<name>.yaml"; its temp sibling is
// "<name>.yaml.tmp-<rand>", so the ".tmp-" test excludes exactly those transient
// files without touching real claims.
func ignoredClaimFile(name string) bool {
	if strings.Contains(name, ".tmp-") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(name))
	return ext != ".yaml" && ext != ".yml"
}

// fingerprintsEqual reports whether two tree fingerprints are identical (the
// same files, each with the same mtime and size). A single added, removed, or
// modified relevant file makes them differ.
func fingerprintsEqual(a, b map[string]fileStamp) bool {
	if len(a) != len(b) {
		return false
	}
	for path, sa := range a {
		sb, ok := b[path]
		if !ok || sa != sb {
			return false
		}
	}
	return true
}

// isInsideDir reports whether path lies within dir (or is dir itself). It is the
// core of serve's startup guardrail: the render, catalog, and lock-store outputs
// must live OUTSIDE the watched claims tree, or one of those writes would look
// like a claim change and drive the watcher in an endless re-render loop.
func isInsideDir(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return true
}
