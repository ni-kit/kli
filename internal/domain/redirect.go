package domain

type RedirectTarget int

const (
	RedirectDefault RedirectTarget = iota // no redirect
	RedirectToOther                       // stdout→stderr (>&2) or stderr→stdout (2>&1)
	RedirectNull                          // /dev/null
	RedirectFile                          // specific file
)

type StreamRedirect struct {
	Target RedirectTarget
	File   string
	Append bool // >> vs >
}

func (r StreamRedirect) IsZero() bool { return r.Target == RedirectDefault }

// StdoutShell returns the shell token for redirecting stdout with this config.
func (r StreamRedirect) StdoutShell() string {
	switch r.Target {
	case RedirectToOther:
		return ">&2"
	case RedirectNull:
		return ">/dev/null"
	case RedirectFile:
		if r.Append {
			return ">>" + r.File
		}
		return ">" + r.File
	}
	return ""
}

// StderrShell returns the shell token for redirecting stderr with this config.
func (r StreamRedirect) StderrShell() string {
	switch r.Target {
	case RedirectToOther:
		return "2>&1"
	case RedirectNull:
		return "2>/dev/null"
	case RedirectFile:
		if r.Append {
			return "2>>" + r.File
		}
		return "2>" + r.File
	}
	return ""
}

// CarouselLabel returns the display label for this target in the TUI carousel.
// isStdout distinguishes which row the label is for (default label differs).
func (r StreamRedirect) CarouselLabel(isStdout bool) string {
	switch r.Target {
	case RedirectDefault:
		if isStdout {
			return "out"
		}
		return "err"
	case RedirectToOther:
		if isStdout {
			return "err"
		}
		return "out"
	case RedirectNull:
		return "null"
	case RedirectFile:
		return "file"
	}
	return ""
}

// Next cycles to the next carousel option.
func (r StreamRedirect) Next() StreamRedirect {
	switch r.Target {
	case RedirectDefault:
		return StreamRedirect{Target: RedirectToOther}
	case RedirectToOther:
		return StreamRedirect{Target: RedirectNull}
	case RedirectNull:
		return StreamRedirect{Target: RedirectFile, File: r.File, Append: r.Append}
	case RedirectFile:
		return StreamRedirect{Target: RedirectDefault}
	}
	return r
}
