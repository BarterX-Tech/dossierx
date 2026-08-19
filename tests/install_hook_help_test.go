// install_hook_help_test.go pins the two things the hook installer's `--help`
// has to get right, both of which it got wrong until the v0.5.2 gate read it
// and neither of which any test could see.
//
// The installer is not run from this repository by the reader it is written
// for. README and the router skill hand them a pinned raw URL, they curl one
// file into their own project, and `--help` is the whole of the documentation
// they get. So a sentence that stops halfway, or a command line that comes
// back mangled, is not a cosmetic defect: it is the only instruction that
// reader has.
//
//	THE HELP STOPPED MID-SENTENCE. `sed -n '/^# USAGE/,/^# Exit status/p'`
//	    ends AT its closing match, so the two-line exit-status paragraph
//	    printed its first line and dropped the second. The last thing a
//	    reader saw was "1 declined, refused," — a sentence with no end.
//	A WINDOWS PATH CAME BACK MANGLED. The invocation was substituted into
//	    the text with sed, whose replacement treats a backslash as an escape.
//	    On the platform install-git-hook.ps1 exists for, the line whose
//	    entire job is to name a command the reader can type is the line that
//	    lost characters.
package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// installHookHelp runs the installer's --help with an optional invocation
// override and returns what a reader would see. The shell is the same one the
// hostile-path corpus resolves (sh, or Git for Windows' bash on Windows),
// because --help is read on every platform the installer runs on.
func installHookHelp(t *testing.T, invocation string) string {
	t.Helper()
	script := filepath.Join(repoRoot(t), "scripts", "install-git-hook.sh")
	cmd := exec.Command(hostileShell(t), script, "--help")
	cmd.Env = os.Environ()
	if invocation != "" {
		cmd.Env = append(cmd.Env, "DOSSIERX_HOOK_INVOCATION="+invocation)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("install-git-hook.sh --help: %v\n%s", err, out)
	}
	return string(out)
}

func TestInstallHookHelpPrintsItsWholeExitStatusSentence(t *testing.T) {
	help := installHookHelp(t, "")

	if !strings.Contains(help, "Exit status:") {
		t.Fatalf("--help prints no exit-status paragraph at all:\n%s", help)
	}
	// The clause that used to be cut. Checking the last WORD rather than the
	// paragraph's presence is the point: the defect was a paragraph that
	// started.
	if !strings.Contains(help, "or failed.") {
		t.Errorf("--help stops before the end of its exit-status sentence. A reader is told what exit 0 means and half of what exit 1 means:\n%s", help)
	}

	trimmed := strings.TrimRight(help, "\n \t")
	if strings.HasSuffix(trimmed, ",") {
		t.Errorf("--help ends on a comma, which is what a truncated range looks like from the outside:\n%s", help)
	}

	// The sentinel itself must not leak into the output: it closes the range,
	// it is not part of the documentation.
	if strings.Contains(help, "END USAGE") {
		t.Errorf("--help prints the END USAGE sentinel, which is range plumbing, not documentation:\n%s", help)
	}
}

func TestInstallHookHelpNamesTheInvocationLiterally(t *testing.T) {
	// A path with backslashes and a drive letter — the shape Git for Windows
	// hands the wrapper, and the shape sed's replacement text destroys.
	const windows = `sh "C:\Users\me\dev\install-git-hook.sh"`

	help := installHookHelp(t, windows)
	if !strings.Contains(help, windows) {
		t.Errorf("--help does not print the invocation it was given, character for character.\nwanted: %s\ngot:\n%s\n\n"+
			"Every backslash that does not survive is a command the reader cannot type, on the one platform where the path always has them.", windows, help)
	}

	// And the repository-relative default must not survive beside it: the
	// reader being told two different ways to run the same file is the defect
	// the invocation override exists to fix.
	if strings.Contains(help, "  scripts/install-git-hook.sh [options]") {
		t.Error("--help still offers the repository-relative path as the usage line, which is a file the reader who curled this script does not have")
	}
}
