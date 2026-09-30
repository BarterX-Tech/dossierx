package briefs

import (
	"io"
	"os"
	"path/filepath"
	"strings"
)

// nestedRepo reports whether git treats dir as another repository's work tree:
// the directory `git add` records as ONE gitlink (mode 160000) instead of
// descending into it and staging its files as blobs. Load hands such a
// directory to FromFiles as a non-regular File, so the working tree refuses
// exactly the path --staged refuses.
//
// The rule is git's own (is_nonbare_repository_dir in git's dir.c), ported
// rather than approximated, because any gap is a folder the two check modes
// read differently. dir is a nested repository when dir/.git is either
//
//   - a regular file (followed through a symlink) whose content starts with
//     "gitdir: " and names, relative to the file's directory unless absolute,
//     a git directory by the rule below; a .git file git cannot open or read
//     counts too, as it does in git — this is what a `git submodule add`
//     checkout and a linked worktree carry; or
//   - a git directory (followed through a symlink), which git's
//     is_git_directory defines as: HEAD is a symlink into refs/, or a file
//     starting "ref:" then optional white space then "refs/", or a file
//     starting with a 40-digit hexadecimal object name; and objects/ and refs/
//     (in the directory its commondir file names, if it has one) are both
//     searchable, git's access(2) X_OK (see searchable for how closely) —
//     what `git init` makes.
//
// Any other .git entry — an empty or junk .git file, a .git file naming a
// directory that is not a git directory, an empty .git directory, a .git
// directory with HEAD but no objects/ — is not a repository to git: `git add`
// stages the folder's files as blobs and skips the .git entry itself. Load
// then reads the folder like any other and skips the .git entry as it skips
// every dot-name, so both modes read it identically.
//
// GIT_COMMON_DIR and GIT_OBJECT_DIRECTORY are honoured as git honours them.
// One case stays outside the rule, as it is outside git's: a folder the index
// ALREADY tracks as files keeps being staged as files after a repository is
// initialised inside it (git consults the index before the .git entry), while
// the working tree alone cannot know that and refuses it.
func nestedRepo(dir string) bool {
	dotGit := filepath.Join(dir, ".git")
	info, err := os.Stat(dotGit)
	if err != nil {
		return false
	}
	if info.Mode().IsRegular() {
		return gitFileNamesRepo(dotGit, info.Size())
	}
	return isGitDirectory(dotGit)
}

// maxGitFileSize is git's read_gitfile_gently limit: a larger .git file is
// not a gitfile.
const maxGitFileSize = 1 << 20

// gitFileNamesRepo is git's read_gitfile_gently: does the .git file at p name
// a git directory? A file git cannot open or read counts as a repository, as
// it does in is_nonbare_repository_dir.
func gitFileNamesRepo(p string, size int64) bool {
	if size > maxGitFileSize {
		return false
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return true
	}
	content := string(raw)
	if !strings.HasPrefix(content, "gitdir: ") {
		return false
	}
	content = strings.TrimRight(content, "\r\n")
	if len(content) < len("gitdir: ")+1 {
		return false
	}
	target := content[len("gitdir: "):]
	if !filepath.IsAbs(target) {
		// git joins by string, not by cleaning: "a/b/../c" resolves through
		// the file system, symlinks included, exactly as git's open does.
		target = filepath.Dir(p) + string(filepath.Separator) + target
	}
	return isGitDirectory(target)
}

// isGitDirectory is git's is_git_directory (setup.c).
func isGitDirectory(suspect string) bool {
	if !validHeadRef(filepath.Join(suspect, "HEAD")) {
		return false
	}
	common := commonDir(suspect)
	objects := filepath.Join(common, "objects")
	if env := os.Getenv("GIT_OBJECT_DIRECTORY"); env != "" {
		objects = env
	}
	return searchable(objects) && searchable(filepath.Join(common, "refs"))
}

// commonDir is git's get_common_dir: GIT_COMMON_DIR when set, else the
// directory suspect/commondir names (relative to suspect unless absolute),
// else suspect itself.
func commonDir(suspect string) string {
	if env := os.Getenv("GIT_COMMON_DIR"); env != "" {
		return env
	}
	raw, err := os.ReadFile(filepath.Join(suspect, "commondir"))
	if err != nil {
		return suspect
	}
	named := strings.TrimRight(string(raw), "\r\n")
	if filepath.IsAbs(named) {
		return named
	}
	return suspect + string(filepath.Separator) + named
}

// headBufSize is the 256-byte buffer git's validate_headref reads HEAD into,
// less its NUL terminator.
const headBufSize = 255

// validHeadRef is git's validate_headref.
func validHeadRef(p string) bool {
	info, err := os.Lstat(p)
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(p)
		return err == nil && strings.HasPrefix(target, "refs/")
	}
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	raw, err := io.ReadAll(io.LimitReader(f, headBufSize))
	if err != nil {
		return false
	}
	head := string(raw)
	if ref, ok := strings.CutPrefix(head, "ref:"); ok {
		if strings.HasPrefix(strings.TrimLeft(ref, " \t\n\v\f\r"), "refs/") {
			return true
		}
	}
	return hexObjectName(head)
}

// hexObjectName is git's get_oid_hex_any on a HEAD file: does it start with a
// full object name? A SHA-256 name starts with a SHA-1-length run of hex, so
// the shorter length decides for both.
func hexObjectName(s string) bool {
	const sha1Hex = 40
	if len(s) < sha1Hex {
		return false
	}
	for i := 0; i < sha1Hex; i++ {
		c := s[i]
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F') {
			return false
		}
	}
	return true
}
