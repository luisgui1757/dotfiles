from pathlib import Path
p=Path("installer/windows_buildtools_query_windows.go")
s=p.read_text()
a=s.index("func completeBuildToolsQueryMembers(")
b=s.index("\nfunc terminateBuildToolsQueryProcesses(",a)
s=s[:a]+'''func completeBuildToolsQueryMembers(deadline time.Time, query func(*buildToolsJobMembers) error) (buildToolsJobMembers, error) {
    var members buildToolsJobMembers
    if err := query(&members); err != nil { return members, err }
    if members.Listed > uint32(len(members.PIDs)) || members.Listed != members.Assigned {
        return members, errors.New("Build Tools inspection membership changed during capture")
    }
    return members, nil
}
'''+s[b:]
p.write_text(s)
