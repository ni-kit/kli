# kli

A command history/execution manager. Record invocations, browse and edit them in a TUI, then re-execute with modifications.

## Usage

```sh
# Record a command into kli history
kli <cmd> [args...]
kli git commit -am "fix bug"

# Open history browser (no args, imports shell history)
kli

# Open history browser filtered to commands run in the current directory
kli .

# Open the most recent kli history entry in edit mode
kli -l
kli --latest

# Show the most recent command and don't ask before opening it in the TUI
kli -ly

# Exec the most recent kli history entry immediately (no TUI)
kli -lx

# Exec the most recent kli history entry from the current directory
kli -lx .

# Show the most recent command and don't ask before executing it
kli -lxy

# Show and execute the most recent command from the current directory
kli -lxy .

# Print the most recent command without opening or executing it
kli -le

# Print the most recent command from the current directory
kli -le .

# Show current/alternative toggle commands, then ask before executing alternative
kli -t

# Execute the configured directory toggle without asking
kli -ty

# Print the current toggle command and the alternative command
kli -te

# Execute without asking, then print the updated toggle state
kli -tey

# Record and exec immediately (no TUI)
kli -x <cmd> [args...]
kli --exec <cmd> [args...]

# Import shell history (zsh/bash/fish) into kli
kli --import
```

## Help

| Command | Action |
|---------|--------|
| `kli` | Open the history browser |
| `kli .` | Open the history browser filtered to the current directory |
| `kli <cmd> [args...]` | Record a command and open it in detail/edit view |
| `kli -x <cmd> [args...]` | Record and execute a command immediately |
| `kli --exec <cmd> [args...]` | Same as `kli -x` |
| `kli -l` | Open the latest history entry in detail/edit view, with confirmation |
| `kli --latest` | Same as `kli -l` |
| `kli -ly` | Open the latest history entry without confirmation |
| `kli -lx` | Execute the latest history entry, with confirmation |
| `kli -lxy` | Execute the latest history entry without confirmation |
| `kli -le` | Print the latest history entry |
| `kli -lx .` | Execute the latest history entry from the current directory, with confirmation |
| `kli -lxy .` | Execute the latest history entry from the current directory without confirmation |
| `kli -le .` | Print the latest history entry from the current directory |
| `kli -t` | Show current/alternative toggle commands and ask before executing the alternative |
| `kli -ty` | Execute the alternative toggle command without asking |
| `kli -te` | Print the current toggle command and alternative command |
| `kli -tey` | Execute the alternative toggle command without asking, then print the updated state |
| `kli --import` | Import shell history from zsh, bash, fish, or sh history files |

### tmux pane scoping

When run inside tmux, `kli` records the `TMUX_PANE` of each command. The latest-entry commands (`kli -l`/`-lx`/`-lxy`/`-le`) then automatically resolve to the most recent command run in the **current pane**, so each pane re-runs its own last command. Outside tmux this falls back to global (or, with a trailing `.`, current-directory) scoping.

## Installation

```sh
go install github.com/ni-kit/kli/cmd/kli
```

## History browser

Navigate with `hjkl` or arrow keys.

| Key | Action |
|-----|--------|
| `enter` | Open command in detail/edit view |
| `x` | Exec selected command immediately |
| `a` | Create new command |
| `/` | Search (see search syntax below) |
| `t` | Edit tags (comma-separated list) |
| `T` | Add selected command to this directory's toggle tuple |
| `q` / `ctrl+c` | Quit |

## Search syntax

Press `/` to open the search bar. Filters are ANDed. Free text matches anywhere in the full command.

| Prefix | Filters by |
|--------|-----------|
| `C:text` | Command name |
| `F:text` | Flag name |
| `V:text` | Flag/arg value |
| `P:text` | Execution path (cwd) |
| `T:text` | Tag |
| `D:text` | Date (substring of `YYYY-MM-DD`) |
| `R:asc` / `R:desc` | Sort by run count |

`P:.` is a shortcut for the current working directory.

`enter` confirms the filter and returns to navigation. `esc` clears the filter.

In detail/edit view, value cells show up to three completion suggestions while editing. `tab` accepts the first suggestion.

## Detail / edit view

Opens when you press `enter` on a history entry. Shows leading env vars, the command, args and flags as editable fields. The header shows the last run time, the current working directory (not the directory of the last run), the command's tags, and the parent command when the entry was forked from another one.

| Key | Action |
|-----|--------|
| `hjkl` / arrows | Navigate cells |
| `i` | Edit cell (keeps current value) |
| `ci` | Change cell (clears then edits) |
| `enter` | Edit cell / run (on exec button) |
| `a` | Add row below |
| `dd` | Delete row |
| `space` | Disable/enable row (struck-through rows are excluded from exec) |
| `f` | On a flag/name cell, rotate `--flag` / `-flag` / `flag` |
| `s` | Toggle secret (value hidden in copy) |
| `S` | Save changes without executing |
| `y` | Copy cell to clipboard |
| `Y` | Copy full command to clipboard |
| `p` | Paste clipboard into cell |
| `m` | Merge flag into previous row |
| `M` | Push flag/value to next row |
| `u` | Undo last edit |
| `E` | Toggle env var rows |
| `r` | Toggle stdout/stderr redirects |
| `t` | Edit tags (comma-separated list) |
| `T` | Toggle all tags on/off |
| `&` / `\|` | Append a chained command segment (`&&` / pipe) |
| `?` | Show/hide the extended help bar |
| `x` | Exec command |
| `esc` | Back to history list |

Leading `KEY=value` env vars are stored with the command and set when executing it. Env vars and leading `~` in values are expanded on exec and previewed inline (e.g. `$HOME` or `~/src`). Env rows are hidden by default when empty; press `E` to show them.

When editing a flag/name cell, bare text is treated as a long flag (`verbose` becomes `--verbose`). Press `f` on that cell to rotate between long flag, short flag, and no prefix. Press `space` on any flag or value row to strike it through and exclude it from execution.

`m` and `M` reorganize how flags and values are split across rows — useful for clustering short flags (e.g. merging `-v` and `-x` into `-vx`) or splitting them apart.

## Tags

Press `t` on any history entry, or in the detail view, to open the tag editor. Enter a comma-separated list of tags and press `enter` to save. Clear the field and confirm to remove all tags. Same tag name always gets the same color across all entries.

Press `T` in the detail view to toggle all tags at once: it strips every tag from the command, and pressing it again puts them back.

Editing the command itself forks it into a new history entry. A fork does not inherit the original's tags, and records the parent command's fingerprint so the origin stays visible in the header. Press `T` on a fork to keep the parent's tags anyway (`T` again drops them); typing tags by hand also keeps them.

## Directory toggles

Press `T` on a history entry to add that command to the current directory's toggle tuple. The first command added is state `1`; the second command is state `0`.

`kli -t` prints the current and alternative commands, asks for confirmation, then executes the alternative command for the current directory and flips the stored state only if the command exits successfully. Use `kli -ty` to skip the prompt. `kli -te` prints the current state command in color and shows the alternative command below it. `kli -tey` skips the prompt, executes the alternative, and prints the updated state after a successful toggle.

## Redirects

Press `r` in the detail view to show the stdout and stderr redirect controls. Each stream has a carousel of targets — press `space` or `enter` to cycle:

| Value | Meaning |
|-------|---------|
| `out` | Normal terminal stdout (default) |
| `err` | Normal terminal stderr (default), or redirect this stream to the other |
| `null` | Discard (`/dev/null`) |
| `file` | Redirect to a file |

When `file` is selected, press `i` or `enter` to type the path. Prefix with `>>` to append instead of overwrite (e.g. `>>app.log`). Exec is blocked until the path is filled in.

Redirect operators in shell history (`>file`, `>>file`, `2>&1`, `2>/dev/null`, etc.) are recognised and parsed automatically on import.

## TODO

- [ ] make toggle not binary, when hitting {BINDING?} on toggle circle element uncheck it (remove from the array) and reorder circle
- [ ] think about forcing toggle through non-zero exit code (to 'reset' state)
- [ ] cleanup main (extract logic to pkgs, libs and services where possible)
- [ ] add step result preview (one liner builder)
- [ ] add tldr support
- [ ] subcmd preview (like env vars)
- [ ] capture stdout & stderr merged into internal file into history; show exit code in history
