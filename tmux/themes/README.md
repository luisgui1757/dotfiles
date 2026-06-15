# tmux / psmux status-bar themes (color discovery)

These are **experiment snippets**, not part of the managed config. They are NOT
auto-loaded and NOT chezmoi-managed — they exist so you can audition status-bar
color schemes live, then bake the winner into `tmux/tmux.conf`.

## Instant feedback (no restart)

From any psmux / tmux session:

```sh
# apply a whole theme instantly
tmux source-file ~/dotfiles/tmux/themes/warm.conf      # adjust path to your clone

# revert to the committed config
# (Ctrl-b then r — the `bind r source-file ~/.tmux.conf` binding)
```

Tweak a single element live without editing any file — `Ctrl-b` then `:` then:

```
set -g window-status-style "fg=#ebbcba,bg=#191724"
set -g status-style        "fg=#31748f,bg=#191724"
set -g status-right        "#[fg=#9ccfd8] %a %d %b #[fg=#f6c177] %H:%M "
```

…and the bar repaints immediately. Repeat until you like it, then tell me the
final values (or the theme name) and I'll merge it into `tmux/tmux.conf`.

## What each option does

The **committed default** (`tmux/tmux.conf`) is the **teal** bar: pine/teal
status *and* inactive windows (inactive is `setw -gu`, so it inherits the pine
status color), gold active window, iris session, foam date, gold time. These
files are ALTERNATIVES you can audition against it; `prefix + r` reverts to the
teal default.

| File | Inactive windows | Active window | Date / time | Mood |
|------|------------------|---------------|-------------|------|
| (default) `tmux.conf` | pine/teal `#31748f` | gold bold | foam / gold | the teal bar — your current look |
| `cool.conf`    | iris `#c4a7e7`  | gold bold | foam / gold  | like the default but iris inactive |
| `warm.conf`    | rose `#ebbcba`  | gold bold | rose / love  | warm, cosy |
| `minimal.conf` | muted `#6e6a86` | bright text bold | muted / subtle | recedes; only active pops |

## Rose Pine palette (copy-paste hexes)

```
base    #191724   surface #1f1d2e   overlay #26233a
muted   #6e6a86   subtle  #908caa   text    #e0def4
love    #eb6f92   gold    #f6c177   rose    #ebbcba
pine    #31748f   foam    #9ccfd8   iris    #c4a7e7
```

## Status-bar elements you can color

- `status-style` — the whole bar's default fg/bg.
- `status-left` — session name block (left).
- `status-right` — date + time block (right).
- `window-status-style` — inactive window cells.
- `window-status-current-format` — active window cell (color is **inlined** with
  `#[fg=...]` because psmux does not apply `window-status-current-style`).
- `message-style` — the `:` command / message line.

> Under Windows Terminal at `opacity: 95` the bar background is see-through, so
> the **text** colors are what you are tuning. For a solid bar, set WT
> `opacity: 100` (whole window) — see the opaque-bar note in `CLAUDE.md`.
