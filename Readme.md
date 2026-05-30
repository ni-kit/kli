# kli

A CLI command history manager. Record invocations, browse and edit them in a TUI, then re-execute with modifications.

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

# Record and exec immediately (no TUI)
kli -x <cmd> [args...]
kli --exec <cmd> [args...]

# Import shell history (zsh/bash/fish) into kli
kli --import
```

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

## Detail / edit view

Opens when you press `enter` on a history entry. Shows leading env vars, the command, args and flags as editable fields.

| Key | Action |
|-----|--------|
| `hjkl` / arrows | Navigate cells |
| `i` | Edit cell (keeps current value) |
| `ci` | Change cell (clears then edits) |
| `enter` | Edit cell / run (on exec button) |
| `a` | Add row below |
| `dd` | Delete row |
| `space` | Toggle row (excluded from exec/copy) |
| `s` | Toggle secret (value hidden in copy) |
| `S` | Save changes without executing |
| `y` | Copy cell to clipboard |
| `Y` | Copy full command to clipboard |
| `p` | Paste clipboard into cell |
| `m` | Merge flag into previous row |
| `M` | Push flag/value to next row |
| `u` | Undo last edit |
| `r` | Toggle stdout/stderr redirects |
| `x` | Exec command |
| `esc` | Back to history list |

Leading `KEY=value` env vars are stored with the command and set when executing it. Env vars and leading `~` in values are expanded on exec and previewed inline (e.g. `$HOME` or `~/src`).

`m` and `M` reorganize how flags and values are split across rows — useful for clustering short flags (e.g. merging `-v` and `-x` into `-vx`) or splitting them apart.

## Tags

Press `t` on any history entry to open the tag editor. Enter a comma-separated list of tags and press `enter` to save. Clear the field and confirm to remove all tags. Same tag name always gets the same color across all entries.

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

- [ ] support && and |
- [ ] add step result preview (one liner builder)
- [ ] add tldr support
- [ ] capture stdout & stderr merged into internal file into history; show exit code in history
