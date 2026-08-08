## The surface: install-scripts

scripts/install-git-hook.sh and scripts/install-git-hook.ps1 are the pre-commit
hook installers. They are curl'd at the pinned tag by README and by the router
skill — which also tells the agent to SHOW THE CLIENT THE CONTENTS before running
them. So these are read aloud on the way into somebody else's repository.

Check, specifically:

- Every version pin names this release; a stale pin fetches the wrong script.
- The two scripts do the same thing. A divergence between the shell and
  PowerShell forms means half the clients get different behaviour.
- Every statement a script prints about what it is doing is true of what it does.
- Every `dossierx` command the installed hook will run exists with the flags
  shown and the exit codes the script branches on.
- Anything the script writes outside the repository it is run in is a finding.
