# Temporary native bootstrap naming proof

Delivery candidate `044f9780705faa996914ed95de2aed2126802ba9` passed the
Windows race, Python, Pester and Lua tests but quality retry 114351013020 failed
PSScriptAnalyzer 1.25.0 on the plural helper `Import-BootstrapModules`.

This disposable Windows job proves that exact original analyzer failure and
zero findings after renaming the helper to `Import-BootstrapModule`. It also
executes the unchanged ZIP extraction and real PS7 → Go → PS5.1 module-binding
regressions with their updated AST lookup name. The temporary workflow and this
note must not enter the delivery commit. No analyzer suppression, builtin-module
binding change, global module-path edit or workstation mutation is made.
