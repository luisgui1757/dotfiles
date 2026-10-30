# ---- superfile: Shift+Q quits into the folder you browsed --------------------
# superfile writes "cd '<folder>'" with POSIX quote escaping. Parse only that
# form and change to the folder as a literal path; never source the file.
spf() {
    local spf_last spf_folder spf_status
    spf_last=$(command spf path-list --lastdir-file) || return
    command spf "$@"
    spf_status=$?
    if [ -f "$spf_last" ]; then
        spf_folder=$(<"$spf_last")
        rm -f -- "$spf_last"
        case $spf_folder in
            "cd '"*"'")
                spf_folder=${spf_folder#"cd '"}
                spf_folder=${spf_folder%"'"}
                spf_folder=${spf_folder//"'\\''"/"'"}
                if [ -d "$spf_folder" ]; then cd -- "$spf_folder" || :; fi
                ;;
        esac
    fi
    return "$spf_status"
}
