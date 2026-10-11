# Temporary Ubuntu completion audit

This diagnostic branch starts from delivery head `044f9780705faa996914ed95de2aed2126802ba9`.
Quality job 114349412458 failed the unchanged Starship and zsh startup smoke
tests when native compinit could not read its insecure-completion prompt from a
terminal. The job did not log the offending completion paths.

The temporary workflow installs the same Ubuntu 26 zsh package, records native
fpath/compaudit paths with physical ancestry, ownership and modes, then runs both
unchanged failing tests against the generated profiles. It changes no system
permissions, production behavior or assertions in its first run. This file and
workflow do not enter delivery.

Run 38098923912/job 114350585028 reproduced both failures and identified
`/usr/share/zsh/vendor-completions` and `/usr/share/zsh` as insecure physical,
root-owned mode 0777 directories; `/usr/share` was also mode 0777. The follow-up
checks those three exact directories are physical and root-owned before setting
only their own modes to 0755. No recursive chmod or completion bypass is used.
Strict compaudit must then produce no insecure paths, and both original tests
must pass. This changes only the explicitly disposable test host.

The final diagnostic uses the exact proposed fixed helper, with namespace-isolated
foreign-owner, symlink, invalid-host and invalid-recipe refusal tests. The private
mount changes only the namespace's fixture directories, then exits before the
normal hosted completion directories are prepared and original tests run.
