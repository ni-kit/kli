package domain

import (
	"os"
	"slices"
	"strings"
	"time"
)

type Run struct {
	RunAt time.Time
	Cwd   string
}

type Invocation struct {
	ID      string
	Command string
	Args    []Arg
	Runs    []Run // newest-first
	Tags    []string
}

func (inv Invocation) LastRun() Run {
	if len(inv.Runs) == 0 {
		return Run{}
	}
	return inv.Runs[0]
}

func (inv *Invocation) AddRun(r Run) {
	inv.Runs = append([]Run{r}, inv.Runs...)
}

func (inv Invocation) ArgsString() string {
	parts := make([]string, 0, len(inv.Args))
	for _, a := range inv.Args {
		parts = append(parts, a.Display())
	}
	return strings.Join(parts, " ")
}

func (inv Invocation) ArgsPreview() string {
	s := inv.ArgsString()
	if len(s) > 50 {
		s = s[:47] + "…"
	}
	return s
}

func (inv Invocation) FullCommand() string {
	return inv.Command + " " + inv.ArgsString()
}

func (inv Invocation) RawArgv() []string {
	argv := make([]string, 0, 1+len(inv.Args))
	argv = append(argv, inv.Command)
	for _, arg := range inv.Args {
		argv = append(argv, arg.RawDisplay())
	}
	return argv
}

// ExpandExecArgv returns argv for execution with runtime-only expansions
// applied. The original argv should still be used for history persistence.
func ExpandExecArgv(argv []string) []string {
	if len(argv) == 0 {
		return nil
	}

	expanded := append([]string(nil), argv...)
	expanded[0] = expandHomeTilde(expanded[0])
	for i := 1; i < len(expanded); i++ {
		expanded[i] = os.ExpandEnv(expandHomeTilde(expanded[i]))
	}
	return expanded
}

// ExpandedArgv returns argv for execution with environment variables expanded
// and leading "~/" resolved to the current HOME directory.
func (inv Invocation) ExpandedArgv() []string {
	return ExpandExecArgv(inv.RawArgv())
}

func expandHomeTilde(token string) string {
	if token != "~" && !strings.HasPrefix(token, "~/") {
		return token
	}

	home := os.Getenv("HOME")
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil || home == "" {
			return token
		}
	}

	if token == "~" {
		return home
	}
	return home + token[1:]
}

// expandShortFlag returns individual single-letter names for a short flag,
// so that -lah and -l -a -h produce the same fingerprint.
func expandShortFlag(a Arg) []string {
	if a.Kind != ArgShortFlag || len(a.Name) <= 1 {
		return []string{a.Name}
	}
	letters := make([]string, len(a.Name))
	for i, ch := range a.Name {
		letters[i] = string(ch)
	}
	return letters
}

func (inv Invocation) CommandFingerprint() string {
	var flags []string
	var positionals []string
	for _, a := range inv.Args {
		switch a.Kind {
		case ArgShortFlag:
			for _, letter := range expandShortFlag(a) {
				flags = append(flags, letter+"="+a.Value)
			}
		case ArgLongFlag:
			flags = append(flags, a.Name+"="+a.Value)
		case ArgPositional, ArgFlagValue:
			positionals = append(positionals, a.Value)
		}
	}
	slices.Sort(flags)
	return inv.Command + "|" + strings.Join(flags, ",") + "|" + strings.Join(positionals, ",")
}

func (inv Invocation) FlagSetFingerprint() string {
	var pairs []string
	for _, a := range inv.Args {
		switch a.Kind {
		case ArgShortFlag:
			for _, letter := range expandShortFlag(a) {
				pairs = append(pairs, letter+"="+a.Value)
			}
		case ArgLongFlag:
			pairs = append(pairs, a.Name+"="+a.Value)
		}
	}
	slices.Sort(pairs)
	return strings.Join(pairs, ",")
}

func (inv Invocation) HasTag(tag string) bool {
	return slices.Contains(inv.Tags, tag)
}

func (inv Invocation) LastRunInDir(cwd string) (Run, bool) {
	for _, run := range inv.Runs {
		if run.Cwd == cwd {
			return run, true
		}
	}
	return Run{}, false
}
