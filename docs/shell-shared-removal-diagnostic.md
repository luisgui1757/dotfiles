# Temporary Windows shared-runtime shell lifecycle proof

This diagnostic is based on delivery `044f9780705faa996914ed95de2aed2126802ba9`.
Its core job 114349412548 failed only the selected-shell fixture: the fixture
requested empty selection without authorizing shared PowerShell removal, then
incorrectly expected PowerShell's retained shell prerequisite to disappear.
The personal suffix remained intact. The planner's retention rule was correct.

The updated fixture first proves that default deselection retains the owned
PowerShell archive and exact managed shell/personal bytes, then explicitly
removes only `tool.powershell`. Its existing exact personal/BOM restoration,
profile cleanup and pre-existing VC++ runtime checks remain unchanged. The
one disposable Windows job executes the complete original native lifecycle;
there is no planner or runtime change, assertion normalization or workstation
mutation. Temporary workflow and this note are excluded from delivery.
