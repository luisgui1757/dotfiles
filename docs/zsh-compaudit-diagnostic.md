# Temporary Ubuntu completion audit

This diagnostic branch starts from delivery head `044f9780705faa996914ed95de2aed2126802ba9`.
Quality job 114349412458 failed the unchanged Starship and zsh startup smoke
tests when native compinit could not read its insecure-completion prompt from a
terminal. The job did not log the offending completion paths.

The temporary workflow installs the same Ubuntu 26 zsh package, records native
fpath/compaudit paths with physical ancestry, ownership and modes, then runs both
unchanged failing tests against the generated profiles. It changes no system
permissions, production behavior or assertions. Its failed status is expected if
the original failure reproduces. This file and workflow do not enter delivery.
