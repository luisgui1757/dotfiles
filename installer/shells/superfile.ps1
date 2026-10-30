# ---- superfile: Shift+Q quits into the folder you browsed --------------------
# superfile writes a cd line with the folder in POSIX single quotes. Parse only
# that form and change to the folder as a literal path; never run the file. spf.exe
# names the application, so the function does not call itself. The cmdlets
# after spf.exe keep its $LASTEXITCODE.
function global:spf {
    $lastDirectory = spf.exe path-list --lastdir-file
    spf.exe @args
    if ($lastDirectory -and (Test-Path -LiteralPath $lastDirectory -PathType Leaf)) {
        $text = [IO.File]::ReadAllText($lastDirectory)
        Remove-Item -LiteralPath $lastDirectory -Force
        if ($text.Length -ge 5 -and $text.StartsWith("cd '") -and $text.EndsWith("'")) {
            $folder = $text.Substring(4, $text.Length - 5).Replace("'\''", "'")
            if (Test-Path -LiteralPath $folder -PathType Container) {
                Set-Location -LiteralPath $folder
            }
        }
    }
}
