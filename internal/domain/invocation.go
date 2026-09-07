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
	// Metadata holds optional contextual key/values captured at record time,
	// e.g. "TMUX_PANE" identifying the tmux pane the command ran in.
	Metadata map[string]string `json:",omitempty"`
}

// Metadata keys under which a run's terminal-pane identity is stored.
const (
	// MetaTMUXPane is the tmux pane id (e.g. "%7").
	MetaTMUXPane = "TMUX_PANE"
	// MetaZellijPane and MetaZellijSession together identify a zellij pane;
	// the pane id is only unique within a session.
	MetaZellijPane    = "ZELLIJ_PANE_ID"
	MetaZellijSession = "ZELLIJ_SESSION_NAME"
)

// Meta returns the metadata value for key, or "" if absent.
func (r Run) Meta(key string) string {
	if r.Metadata == nil {
		return ""
	}
	return r.Metadata[key]
}

type ChainOp string

const (
	ChainAnd  ChainOp = "&&"
	ChainPipe ChainOp = "|"
)

// ChainLink represents one segment in a chained command (e.g. cmd1 && cmd2 | cmd3).
// Op is the operator that connects this segment to the previous one; it is empty
// for the first segment when represented via AllSegments().
type ChainLink struct {
	Op      ChainOp
	Command string
	Env     []EnvVar
	Args    []Arg
	Stdout  StreamRedirect
	Stderr  StreamRedirect
}

type Invocation struct {
	ID      string
	Command string
	Env     []EnvVar
	Args    []Arg
	Chain   []ChainLink // subsequent commands; nil = simple single command
	Runs    []Run       // newest-first
	Tags    []string
	// Parent is the CommandFingerprint of the invocation this one was edited
	// from, empty when the command was not derived from another.
	Parent string `json:",omitempty"`
	Stdout StreamRedirect
	Stderr StreamRedirect
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

// IsChain reports whether this invocation consists of multiple chained commands.
func (inv Invocation) IsChain() bool { return len(inv.Chain) > 0 }

// DisplayCommand returns a human-readable representation of the segment with
// secrets masked (values marked Redacted are shown as ••••).
func (l ChainLink) DisplayCommand() string {
	return strings.Join(segmentDisplayParts(l.Command, l.Env, l.Args, l.Stdout, l.Stderr), " ")
}

// AllSegments returns every segment of the invocation as a flat slice.
// Segment 0 is the main command (Op is empty); subsequent segments carry their
// connecting operator (ChainAnd or ChainPipe).
func (inv Invocation) AllSegments() []ChainLink {
	first := ChainLink{
		Command: inv.Command,
		Env:     inv.Env,
		Args:    inv.Args,
		Stdout:  inv.Stdout,
		Stderr:  inv.Stderr,
	}
	return append([]ChainLink{first}, inv.Chain...)
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
	parts := segmentDisplayParts(inv.Command, inv.Env, inv.Args, inv.Stdout, inv.Stderr)
	for _, link := range inv.Chain {
		parts = append(parts, string(link.Op))
		parts = append(parts, segmentDisplayParts(link.Command, link.Env, link.Args, link.Stdout, link.Stderr)...)
	}
	return strings.Join(parts, " ")
}

func segmentDisplayParts(command string, env []EnvVar, args []Arg, stdout, stderr StreamRedirect) []string {
	parts := make([]string, 0, len(env)+1+len(args)+2)
	for _, e := range env {
		parts = append(parts, e.Display())
	}
	if command != "" {
		parts = append(parts, command)
	}
	for _, a := range args {
		parts = append(parts, a.Display())
	}
	if s := stdout.StdoutShell(); s != "" {
		parts = append(parts, s)
	}
	if s := stderr.StderrShell(); s != "" {
		parts = append(parts, s)
	}
	return parts
}

func segmentRawParts(command string, env []EnvVar, args []Arg, stdout, stderr StreamRedirect) []string {
	parts := make([]string, 0, len(env)+1+len(args)+2)
	for _, e := range env {
		if e.Key != "" {
			parts = append(parts, e.RawDisplay())
		}
	}
	if command != "" {
		parts = append(parts, command)
	}
	for _, a := range args {
		parts = append(parts, a.RawDisplay())
	}
	if s := stdout.StdoutShell(); s != "" {
		parts = append(parts, s)
	}
	if s := stderr.StderrShell(); s != "" {
		parts = append(parts, s)
	}
	return parts
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
	for _, link := range inv.Chain {
		tokens = append(tokens, string(link.Op))
		tokens = append(tokens, segmentRawParts(link.Command, link.Env, link.Args, link.Stdout, link.Stderr)...)
	}
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
	fp := segmentFingerprint(inv.Command, inv.Env, inv.Args, inv.Stdout, inv.Stderr)
	for _, link := range inv.Chain {
		fp += "|" + string(link.Op) + "|" + segmentFingerprint(link.Command, link.Env, link.Args, link.Stdout, link.Stderr)
	}
	return fp
}

func segmentFingerprint(command string, env []EnvVar, args []Arg, stdout, stderr StreamRedirect) string {
	var flags []string
	var envStrs []string
	var positionals []string
	for _, e := range env {
		if e.Key != "" {
			envStrs = append(envStrs, e.RawDisplay())
		}
	}
	for _, a := range args {
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
	slices.Sort(envStrs)
	slices.Sort(flags)
	return strings.Join(envStrs, ",") + "|" + command + "|" + strings.Join(flags, ",") + "|" + strings.Join(positionals, ",") + "|" + stdout.StdoutShell() + "|" + stderr.StderrShell()
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

func (inv Invocation) ArgDisplayTokens() []string {
	tokens := make([]string, len(inv.Args))
	for i, a := range inv.Args {
		tokens[i] = a.Display()
	}
	return tokens
}

func ArgTokenSimilarity(a, b []string) (float64, []bool) {
	maxLen := maxInt(len(a), len(b))
	if maxLen == 0 {
		return 0, nil
	}
	matches := 0
	diffMask := make([]bool, maxLen)
	for i := range maxLen {
		var va, vb string
		if i < len(a) {
			va = a[i]
		}
		if i < len(b) {
			vb = b[i]
		}
		if va == vb {
			matches++
		} else {
			diffMask[i] = true
		}
	}
	return float64(matches) / float64(maxLen), diffMask
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (inv Invocation) LastRunInDir(cwd string) (Run, bool) {
	for _, run := range inv.Runs {
		if run.Cwd == cwd {
			return run, true
		}
	}
	return Run{}, false
}

// LastRunWithMeta returns the newest run whose metadata[key] == value.
func (inv Invocation) LastRunWithMeta(key, value string) (Run, bool) {
	for _, run := range inv.Runs {
		if run.Meta(key) == value {
			return run, true
		}
	}
	return Run{}, false
}

// LastRunWithMetaAll returns the newest run whose metadata matches every
// key/value pair in want. An empty want matches nothing.
func (inv Invocation) LastRunWithMetaAll(want map[string]string) (Run, bool) {
	if len(want) == 0 {
		return Run{}, false
	}
	for _, run := range inv.Runs {
		match := true
		for k, v := range want {
			if run.Meta(k) != v {
				match = false
				break
			}
		}
		if match {
			return run, true
		}
	}
	return Run{}, false
}
