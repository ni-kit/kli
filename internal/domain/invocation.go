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
	Env     []EnvVar
	Args    []Arg
	Runs    []Run // newest-first
	Tags    []string
	Stdout  StreamRedirect
	Stderr  StreamRedirect
}

type EnvVar struct {
	Key      string
	Value    string
	Redacted bool
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
	parts := make([]string, 0, len(inv.Env)+1+len(inv.Args)+2)
	for _, e := range inv.Env {
		parts = append(parts, e.Display())
	}
	parts = append(parts, inv.Command)
	for _, a := range inv.Args {
		parts = append(parts, a.Display())
	}
	if s := inv.Stdout.StdoutShell(); s != "" {
		parts = append(parts, s)
	}
	if s := inv.Stderr.StderrShell(); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, " ")
}

func (inv Invocation) RawArgv() []string {
	argv := make([]string, 0, 1+len(inv.Args))
	argv = append(argv, inv.Command)
	for _, arg := range inv.Args {
		argv = append(argv, arg.RawDisplay())
	}
	return argv
}

func (inv Invocation) RawEnv() []string {
	env := make([]string, 0, len(inv.Env))
	for _, e := range inv.Env {
		if e.Key == "" {
			continue
		}
		env = append(env, e.RawDisplay())
	}
	return env
}

func (inv Invocation) RawCommandTokens() []string {
	tokens := make([]string, 0, len(inv.Env)+1+len(inv.Args))
	tokens = append(tokens, inv.RawEnv()...)
	tokens = append(tokens, inv.RawArgv()...)
	return tokens
}

// ExpandExecArgv returns argv for execution with runtime-only expansions
// applied. The original argv should still be used for history persistence.
func ExpandExecArgv(argv []string) []string {
	return ExpandExecArgvWithEnv(argv, nil)
}

// ExpandExecArgvWithEnv returns argv for execution with runtime-only expansions
// applied, using env assignments as overrides when expanding argument values.
func ExpandExecArgvWithEnv(argv []string, env []string) []string {
	if len(argv) == 0 {
		return nil
	}

	expanded := append([]string(nil), argv...)
	lookup := envLookup(env)
	expanded[0] = expandHomeTilde(expanded[0])
	for i := 1; i < len(expanded); i++ {
		expanded[i] = os.Expand(expandHomeTilde(expanded[i]), lookup)
	}
	return expanded
}

// ExpandedArgv returns argv for execution with environment variables expanded
// and leading "~/" resolved to the current HOME directory.
func (inv Invocation) ExpandedArgv() []string {
	return ExpandExecArgvWithEnv(inv.RawArgv(), inv.ExpandedEnv())
}

func (inv Invocation) ExpandedEnv() []string {
	return ExpandEnvAssignments(inv.RawEnv())
}

func ExpandEnvAssignments(env []string) []string {
	expanded := make([]string, 0, len(env))
	lookup := envLookup(env)
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		expandedValue := os.Expand(expandHomeTilde(value), lookup)
		expanded = append(expanded, key+"="+expandedValue)
	}
	return expanded
}

func envLookup(env []string) func(string) string {
	overrides := map[string]string{}
	for _, item := range env {
		key, value, ok := strings.Cut(item, "=")
		if ok {
			overrides[key] = value
		}
	}
	return func(key string) string {
		if value, ok := overrides[key]; ok {
			return value
		}
		return os.Getenv(key)
	}
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
	var env []string
	var positionals []string
	for _, e := range inv.Env {
		if e.Key != "" {
			env = append(env, e.RawDisplay())
		}
	}
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
	slices.Sort(env)
	slices.Sort(flags)
	return strings.Join(env, ",") + "|" + inv.Command + "|" + strings.Join(flags, ",") + "|" + strings.Join(positionals, ",") + "|" + inv.Stdout.StdoutShell() + "|" + inv.Stderr.StderrShell()
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
