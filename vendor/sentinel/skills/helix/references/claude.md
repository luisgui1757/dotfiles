# Claude Code dispatch

Check `claude --version`, `claude --help` and the configured model catalog.
Prefer a suitable native reviewer. To reach Claude from another host, run its
headless CLI with the requested model and effort, using its existing login.

For a review that only needs files, current Claude Code supports:

```sh
claude -p --model "$review_model" --effort "$effort" \
  --restricted --permission-mode dontAsk --permission-prompts none \
  --tools Read,Glob,Grep --allowedTools Read,Glob,Grep \
  --strict-mcp-config --mcp-config '{"mcpServers":{}}' \
  --settings '{"disableAllHooks":true}' --disable-slash-commands \
  --no-chrome --no-session-persistence --add-dir "$repo" \
  --output-format stream-json --verbose < "$brief"
```

Run from a dedicated, authorized scratch directory. Pass repository instructions,
full diffs and captured test output because this reviewer cannot run Git or tests.
`--restricted` ignores user/project settings and confines file tools; managed
settings still apply. Verify the advertised tool set at startup. Do not enable
Bash merely to recover a missing diff the writer can supply.

`--bare` has different authentication behavior and does not use subscription
OAuth. Do not substitute it blindly or request secret values. Check response
model and final error status. Effort may be accepted without effective effort
being exposed; keep those claims separate. Do not save full transcripts by default.

Source: [Claude Code headless mode](https://code.claude.com/docs/en/headless).
