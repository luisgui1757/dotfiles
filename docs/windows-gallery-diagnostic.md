# Temporary Windows Gallery dependency diagnostic

This branch is based on delivery candidate `044f9780705faa996914ed95de2aed2126802ba9`.
It is diagnostic only and must not enter PR #87. The delivery quality job
114349412700 failed before its test suite because all three unchanged
`Install-Module PSScriptAnalyzer` attempts reported no package match.
Its raw log does not establish missing repository registration or a feed outage.

The single Windows 2025 job records native module/provider versions, selected
repository metadata and the official feed HTTP/entry result, then runs the
original required provisioning script with additional verbose output. Its
failure remains a job failure. A separate `always()` step compares explicit
Gallery discovery and registers only the official default if actually absent;
it never masks the original result or replaces the required checker. This is a
disposable hosted-runner experiment, with no workstation changes, new PR,
canonical workflow changes or delivery-code changes.
