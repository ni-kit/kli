# kli

A CLI command history manager. Record invocations, browse and edit them in a TUI, then re-execute with modifications.

## Usage

```sh
# Record a command into kli history
kli <cmd> [args...]
kli git commit -am "fix bug"

# Open history browser (no args, imports shell history)
kli

# Open the most recent kli history entry in edit mode
kli -l
kli --latest

# Exec the most recent kli history entry immediately (no TUI)
kli -lx

# Record and exec immediately (no TUI)
kli -x <cmd> [args...]
kli --exec <cmd> [args...]

# Import shell history (zsh/bash/fish) into kli
kli --import
```

## Installation

```sh 
ask your llm how to
```

## History browser

Navigate with `hjkl` or arrow keys.

| Key | Action |
|-----|--------|
| `enter` | Open command in detail/edit view |
| `x` | Exec selected command immediately |
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

`enter` confirms the filter and returns to navigation. `esc` clears the filter.

## Detail / edit view

Opens when you press `enter` on a history entry. Shows command, args and flags as an editable table.

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
| `y` | Copy cell to clipboard |
| `Y` | Copy full command to clipboard |
| `p` | Paste clipboard into cell |
| `u` | Undo last edit |
| `x` | Exec command |
| `esc` | Back to history list |

Env vars in values are expanded on exec and previewed inline (e.g. `$HOME` → `/Users/you`).

## Tags

Press `t` on any history entry to open the tag editor. Enter a comma-separated list of tags and press `enter` to save. Clear the field and confirm to remove all tags. Same tag name always gets the same color across all entries.

## TODO

- [ ] capital X: capture stdout/stderr to files, show exit code in history