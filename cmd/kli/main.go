package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ni-kit/kli/internal/domain"
	"github.com/ni-kit/kli/internal/repo"
	"github.com/ni-kit/kli/internal/service"
	"github.com/ni-kit/kli/internal/tui"
)

// applyRedirects applies stdout/stderr redirects to the current process file
// descriptors via dup2 — used immediately before syscall.Exec.
func applyRedirects(stdout, stderr domain.StreamRedirect) error {
	if err := applyStreamRedirect(1, stdout); err != nil {
		return err
	}
	return applyStreamRedirect(2, stderr)
}

func applyStreamRedirect(fd int, r domain.StreamRedirect) error {
	switch r.Target {
	case domain.RedirectDefault:
		return nil
	case domain.RedirectToOther:
		src := 2
		if fd == 2 {
			src = 1
		}
		return syscall.Dup2(src, fd)
	case domain.RedirectNull:
		f, err := os.OpenFile("/dev/null", os.O_WRONLY, 0)
		if err != nil {
			return err
		}
		return syscall.Dup2(int(f.Fd()), fd)
	case domain.RedirectFile:
		flags := os.O_WRONLY | os.O_CREATE
		if r.Append {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
		}
		f, err := os.OpenFile(r.File, flags, 0o644)
		if err != nil {
			return err
		}
		return syscall.Dup2(int(f.Fd()), fd)
	}
	return nil
}

// openWriteTarget opens a file or /dev/null for a redirect. Returns nil writer
// when the redirect target is RedirectToOther (caller handles that case).
func openWriteTarget(r domain.StreamRedirect) (io.Writer, error) {
	switch r.Target {
	case domain.RedirectNull:
		return io.Discard, nil
	case domain.RedirectFile:
		flags := os.O_WRONLY | os.O_CREATE
		if r.Append {
			flags |= os.O_APPEND
		} else {
			flags |= os.O_TRUNC
		}
		return os.OpenFile(r.File, flags, 0o644)
	}
	return nil, nil
}

// applyRedirectsToCmd sets cmd.Stdout/Stderr according to redirect config.
// Must be called after cmd.Stdout/Stderr are already set to their defaults.
func applyRedirectsToCmd(cmd *exec.Cmd, stdout, stderr domain.StreamRedirect) error {
	if !stdout.IsZero() {
		if stdout.Target == domain.RedirectToOther {
			cmd.Stdout = cmd.Stderr
		} else {
			w, err := openWriteTarget(stdout)
			if err != nil {
				return err
			}
			cmd.Stdout = w
		}
	}
	if !stderr.IsZero() {
		if stderr.Target == domain.RedirectToOther {
			cmd.Stderr = cmd.Stdout
		} else {
			w, err := openWriteTarget(stderr)
			if err != nil {
				return err
			}
			cmd.Stderr = w
		}
	}
	return nil
}

type latestAction int

const (
	latestNone latestAction = iota
	latestOpen
	latestExec
	latestEcho
)

type toggleAction int

const (
	toggleNone toggleAction = iota
	toggleExec
	toggleEcho
)

type cliOptions struct {
	importHistory   bool
	execArgv        []string
	recordArgv      []string
	openRecorded    bool
	latestAction    latestAction
	toggleAction    toggleAction
	toggleEchoAfter bool
	confirm         bool
	currentDir      bool
	initialSearch   string
}

func main() {
	histPath, err := repo.HistoryPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kli: cannot resolve history path:", err)
		os.Exit(1)
	}
	histRepo := repo.NewJSONHistoryRepo(histPath)
	historySvc := service.NewHistoryService(histRepo)
	importSvc := service.NewImportService(histRepo)
	togglePath, err := repo.TogglePath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kli: cannot resolve toggle path:", err)
		os.Exit(1)
	}
	toggleSvc := service.NewToggleService(repo.NewJSONToggleRepo(togglePath))

	opts := parseCLIOptions(os.Args[1:])
	if opts.toggleAction != toggleNone {
		cwd, _ := os.Getwd()
		toggle, err := toggleSvc.Get(cwd)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if opts.toggleAction == toggleEcho {
			echoToggle(*toggle)
			return
		}
		if opts.confirm {
			echoToggle(*toggle)
			ok, err := confirmToggleAction()
			if err != nil {
				fmt.Fprintln(os.Stderr, "kli: prompt failed:", err)
				os.Exit(1)
			}
			if !ok {
				return
			}
		}
		inv := toggle.Alternative()
		if inv == nil {
			fmt.Fprintln(os.Stderr, "kli: alternative toggle command is not configured")
			os.Exit(1)
		}
		code := runInvocationAndWait(*inv)
		if code == 0 {
			nextState := toggle.AlternativeState()
			if err := toggleSvc.SetState(cwd, nextState); err != nil {
				fmt.Fprintln(os.Stderr, "kli: failed to save toggle state:", err)
				os.Exit(1)
			}
			if opts.toggleEchoAfter {
				toggle.State = nextState
				echoToggle(*toggle)
			}
		}
		os.Exit(code)
	}

	if opts.importHistory {
		n, err := importSvc.ImportShellHistory()
		if err != nil {
			fmt.Fprintln(os.Stderr, "kli: import failed:", err)
			os.Exit(1)
		}
		fmt.Printf("kli: imported %d new entries\n", n)
		os.Exit(0)
	}

	if len(opts.execArgv) > 0 {
		inv := service.Parse(opts.execArgv)
		if err := historySvc.Record(inv); err != nil {
			fmt.Fprintln(os.Stderr, "kli: failed to save history:", err)
		}
		execInvocation(inv)
	}

	if len(opts.recordArgv) > 0 {
		if err := historySvc.Record(service.Parse(opts.recordArgv)); err != nil {
			fmt.Fprintln(os.Stderr, "kli: failed to save history:", err)
		}
	}

	invocations, err := historySvc.All()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kli: failed to load history:", err)
		os.Exit(1)
	}
	if len(invocations) == 0 {
		fmt.Fprintln(os.Stderr, "kli: no history found, importing from shell history…")
		if n, err := importSvc.ImportShellHistory(); err == nil && n > 0 {
			fmt.Fprintf(os.Stderr, "kli: imported %d entries\n", n)
			invocations, _ = historySvc.All()
		}
	}

	if opts.latestAction != latestNone {
		cwd, _ := os.Getwd()
		inv, err := historySvc.Latest(cwd, opts.currentDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		if opts.latestAction == latestEcho {
			fmt.Println(strings.Join(inv.RawCommandTokens(), " "))
			return
		}

		if opts.confirm {
			ok, err := confirmAction(inv, opts.latestAction)
			if err != nil {
				fmt.Fprintln(os.Stderr, "kli: prompt failed:", err)
				os.Exit(1)
			}
			if !ok {
				return
			}
		}

		if opts.latestAction == latestExec {
			if err := historySvc.Record(service.Parse(inv.RawCommandTokens())); err != nil {
				fmt.Fprintln(os.Stderr, "kli: failed to save history:", err)
			}
			execInvocation(*inv)
		}

		runApp(tui.NewAppOnDetailWithToggles(*inv, invocations, historySvc, toggleSvc), historySvc)
		return
	}

	var app *tui.App
	if opts.openRecorded {
		if len(invocations) == 0 {
			fmt.Fprintln(os.Stderr, "kli: no history yet")
			os.Exit(1)
		}
		app = tui.NewAppOnDetailWithToggles(invocations[0], invocations, historySvc, toggleSvc)
	} else if opts.initialSearch != "" {
		app = tui.NewAppWithSearchAndToggles(invocations, historySvc, toggleSvc, opts.initialSearch)
	} else {
		app = tui.NewAppWithSearchAndToggles(invocations, historySvc, toggleSvc, "")
	}

	runApp(app, historySvc)
}

func parseCLIOptions(args []string) cliOptions {
	if len(args) == 0 {
		return cliOptions{}
	}
	if args[0] == "--import" {
		return cliOptions{importHistory: true}
	}
	if opts, ok := latestOptions(args, "--latest", latestOpen, true); ok {
		return opts
	}
	if opts, ok := flagSetOptions(args); ok {
		return opts
	}
	if len(args) > 1 && (args[0] == "-x" || args[0] == "--exec") {
		return cliOptions{execArgv: args[1:]}
	}
	if len(args) == 1 && args[0] == "." {
		return cliOptions{initialSearch: "P:."}
	}
	return cliOptions{recordArgv: args, openRecorded: len(args) > 0}
}

type cliFlagSet struct {
	letters map[rune]bool
	rest    []string
}

func flagSetOptions(args []string) (cliOptions, bool) {
	set, ok := parseShortFlagSet(args)
	if !ok {
		return cliOptions{}, false
	}

	hasLatest := set.has('l')
	hasToggle := set.has('t')
	if hasLatest == hasToggle {
		return cliOptions{}, false
	}
	if hasToggle {
		return toggleFlagSetOptions(set)
	}
	return latestFlagSetOptions(set)
}

func parseShortFlagSet(args []string) (cliFlagSet, bool) {
	set := cliFlagSet{letters: map[rune]bool{}}
	i := 0
	for ; i < len(args); i++ {
		arg := args[i]
		if arg == "." {
			break
		}
		if !strings.HasPrefix(arg, "-") || strings.HasPrefix(arg, "--") || len(arg) < 2 {
			return cliFlagSet{}, false
		}
		for _, r := range arg[1:] {
			switch r {
			case 'e', 'l', 't', 'x', 'y':
				if set.letters[r] {
					return cliFlagSet{}, false
				}
				set.letters[r] = true
			default:
				return cliFlagSet{}, false
			}
		}
	}
	if len(set.letters) == 0 {
		return cliFlagSet{}, false
	}
	set.rest = args[i:]
	return set, true
}

func (s cliFlagSet) has(letter rune) bool {
	return s.letters[letter]
}

func toggleFlagSetOptions(set cliFlagSet) (cliOptions, bool) {
	if len(set.rest) > 0 || set.has('l') || set.has('x') {
		return cliOptions{}, false
	}
	echo := set.has('e')
	yes := set.has('y')
	if echo && !yes {
		return cliOptions{toggleAction: toggleEcho}, true
	}
	return cliOptions{toggleAction: toggleExec, toggleEchoAfter: echo, confirm: !yes}, true
}

func latestFlagSetOptions(set cliFlagSet) (cliOptions, bool) {
	if len(set.rest) > 1 || (len(set.rest) == 1 && set.rest[0] != ".") || set.has('t') {
		return cliOptions{}, false
	}
	if set.has('e') && set.has('x') {
		return cliOptions{}, false
	}
	action := latestOpen
	if set.has('e') {
		action = latestEcho
	} else if set.has('x') {
		action = latestExec
	}
	return cliOptions{
		latestAction: action,
		confirm:      !set.has('y'),
		currentDir:   len(set.rest) == 1,
	}, true
}

func latestOptions(args []string, long string, action latestAction, confirm bool) (cliOptions, bool) {
	if len(args) == 0 {
		return cliOptions{}, false
	}
	if args[0] != long {
		return cliOptions{}, false
	}
	switch len(args) {
	case 1:
		return cliOptions{latestAction: action, confirm: confirm}, true
	case 2:
		if args[1] == "." {
			return cliOptions{latestAction: action, confirm: confirm, currentDir: true}, true
		}
	}
	return cliOptions{}, false
}

func confirmAction(inv *domain.Invocation, action latestAction) (bool, error) {
	fmt.Println("$", inv.FullCommand())
	prompt := "Open in TUI? [y/N]: "
	if action == latestExec {
		prompt = "Execute? [y/N]: "
	}
	fmt.Print(prompt)

	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "y" || answer == "yes", nil
}

func confirmToggleAction() (bool, error) {
	fmt.Print("Execute alternative? [y/N]: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && line == "" {
		return false, err
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "y" || answer == "yes", nil
}

func echoToggle(toggle domain.Toggle) {
	const (
		reset  = "\033[0m"
		green  = "\033[32m"
		yellow = "\033[33m"
		dim    = "\033[2m"
	)

	current := "(not set)"
	if inv := toggle.Current(); inv != nil {
		current = inv.FullCommand()
	}
	alternative := "(not set)"
	if inv := toggle.Alternative(); inv != nil {
		alternative = inv.FullCommand()
	}

	fmt.Printf("%scurrent [%d]: %s%s\n", green, toggle.State, current, reset)
	fmt.Printf("%salternative [%d]: %s%s\n", yellow+dim, toggle.AlternativeState(), alternative, reset)
}

func runApp(app *tui.App, historySvc service.HistoryService) {
	p := tea.NewProgram(app)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if app.ExecRequested() {
		inv := app.ExecInvocation()

		// Record with a fresh run timestamp so the exec shows up in history.
		cwd, _ := os.Getwd()
		toRecord := inv
		toRecord.ID = fmt.Sprintf("%d", time.Now().UnixNano())
		toRecord.Runs = []domain.Run{{RunAt: time.Now(), Cwd: cwd}}
		if err := historySvc.Record(toRecord); err != nil {
			fmt.Fprintln(os.Stderr, "kli: failed to save history:", err)
		}

		execInvocation(inv)
	}
}

func runInvocationAndWait(inv domain.Invocation) int {
	if inv.Command == "" {
		fmt.Fprintln(os.Stderr, "kli: empty toggle command")
		return 1
	}
	fmt.Println("$", inv.FullCommand())
	groups := chainAndGroups(inv.AllSegments())
	for _, group := range groups {
		if code := runPipeGroupAndWait(group); code != 0 {
			return code
		}
	}
	return 0
}

// execInvocation executes an invocation, replacing the current process with the
// final command. Chains are handled natively without involving a shell.
func execInvocation(inv domain.Invocation) {
	if !inv.IsChain() {
		execSingle(inv.FullCommand(), inv.ExpandedArgv(), inv.ExpandedEnv(), inv.Stdout, inv.Stderr)
		return
	}
	execNativeChain(inv)
}

// execSingle prints a preview and replaces the current process via syscall.Exec.
func execSingle(display string, argv, env []string, stdout, stderr domain.StreamRedirect) {
	fmt.Println("$", display)

	bin, err := exec.LookPath(argv[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "kli: command not found:", argv[0])
		os.Exit(1)
	}
	if err := applyRedirects(stdout, stderr); err != nil {
		fmt.Fprintln(os.Stderr, "kli: redirect failed:", err)
		os.Exit(1)
	}
	if err := syscall.Exec(bin, argv, mergedEnv(os.Environ(), env)); err != nil {
		fmt.Fprintln(os.Stderr, "kli: exec failed:", err)
		os.Exit(1)
	}
}

// pipeEnds holds the two ends of an os.Pipe.
type pipeEnds struct{ r, w *os.File }

func mustPipe() pipeEnds {
	r, w, err := os.Pipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kli: pipe:", err)
		os.Exit(1)
	}
	return pipeEnds{r, w}
}

// execNativeChain runs a chained invocation natively:
//   - "&&" groups are executed sequentially; the chain aborts on a non-zero exit.
//   - "|" within a group connects processes via os.Pipe.
//   - The very last command in the chain replaces the current process via syscall.Exec.
func execNativeChain(inv domain.Invocation) {
	fmt.Println("$", inv.FullCommand())
	groups := chainAndGroups(inv.AllSegments())
	for i, group := range groups {
		if i == len(groups)-1 {
			execLastPipeGroup(group) // never returns
		} else {
			if code := runPipeGroupAndWait(group); code != 0 {
				os.Exit(code)
			}
		}
	}
}

// chainAndGroups splits a flat segment list into pipeline sub-groups separated
// by "&&". Segments within a group are connected by "|".
func chainAndGroups(segs []domain.ChainLink) [][]domain.ChainLink {
	var groups [][]domain.ChainLink
	var cur []domain.ChainLink
	for _, seg := range segs {
		if seg.Op == domain.ChainAnd && len(cur) > 0 {
			groups = append(groups, cur)
			cur = nil
		}
		cur = append(cur, seg)
	}
	if len(cur) > 0 {
		groups = append(groups, cur)
	}
	return groups
}

// execLastPipeGroup starts all but the last segment as child processes connected
// by pipes, then replaces the current process with the last segment via
// syscall.Exec. Never returns.
func execLastPipeGroup(segs []domain.ChainLink) {
	if len(segs) == 1 {
		seg := segs[0]
		argv, env := segExpandedArgvEnv(seg)
		execSingle(seg.DisplayCommand(), argv, env, seg.Stdout, seg.Stderr)
		return // unreachable — execSingle never returns
	}

	n := len(segs)
	pipes := make([]pipeEnds, n-1)
	for i := range pipes {
		pipes[i] = mustPipe()
	}

	// Start all but the last as background children.
	for i := 0; i < n-1; i++ {
		seg := segs[i]
		argv, env := segExpandedArgvEnv(seg)

		bin, err := exec.LookPath(argv[0])
		if err != nil {
			fmt.Fprintln(os.Stderr, "kli: command not found:", argv[0])
			os.Exit(1)
		}

		cmd := exec.Command(bin, argv[1:]...)
		cmd.Env = mergedEnv(os.Environ(), env)
		cmd.Stderr = os.Stderr

		if i == 0 {
			cmd.Stdin = os.Stdin
		} else {
			cmd.Stdin = pipes[i-1].r
		}
		cmd.Stdout = pipes[i].w

		// Stderr redirect for this segment (stdout is going into the pipe).
		if !seg.Stderr.IsZero() {
			if err := applyRedirectsToCmd(cmd, domain.StreamRedirect{}, seg.Stderr); err != nil {
				fmt.Fprintln(os.Stderr, "kli: redirect:", err)
				os.Exit(1)
			}
		}

		if err := cmd.Start(); err != nil {
			fmt.Fprintln(os.Stderr, "kli: exec:", err)
			os.Exit(1)
		}

		// Close the ends we've handed off to the child; keep only what we still need.
		pipes[i].w.Close()
		if i > 0 {
			pipes[i-1].r.Close()
		}
	}

	// Last segment: redirect stdin from the last pipe, then syscall.Exec.
	lastSeg := segs[n-1]
	argv, env := segExpandedArgvEnv(lastSeg)

	bin, err := exec.LookPath(argv[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "kli: command not found:", argv[0])
		os.Exit(1)
	}

	if err := syscall.Dup2(int(pipes[n-2].r.Fd()), 0 /*stdin*/); err != nil {
		fmt.Fprintln(os.Stderr, "kli: dup2:", err)
		os.Exit(1)
	}
	pipes[n-2].r.Close()

	if err := applyRedirects(lastSeg.Stdout, lastSeg.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "kli: redirect:", err)
		os.Exit(1)
	}
	if err := syscall.Exec(bin, argv, mergedEnv(os.Environ(), env)); err != nil {
		fmt.Fprintln(os.Stderr, "kli: exec failed:", err)
		os.Exit(1)
	}
}

// runPipeGroupAndWait runs a pipeline group, waits for all processes to finish,
// and returns the exit code of the last command in the group.
func runPipeGroupAndWait(segs []domain.ChainLink) int {
	if len(segs) == 1 {
		seg := segs[0]
		argv, env := segExpandedArgvEnv(seg)

		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = mergedEnv(os.Environ(), env)

		if err := applyRedirectsToCmd(cmd, seg.Stdout, seg.Stderr); err != nil {
			fmt.Fprintln(os.Stderr, "kli: redirect:", err)
			return 1
		}
		if err := cmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				return exitErr.ExitCode()
			}
			return 1
		}
		return 0
	}

	n := len(segs)
	pipes := make([]pipeEnds, n-1)
	for i := range pipes {
		pipes[i] = mustPipe()
	}

	cmds := make([]*exec.Cmd, n)
	for i, seg := range segs {
		argv, env := segExpandedArgvEnv(seg)

		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Env = mergedEnv(os.Environ(), env)
		cmd.Stderr = os.Stderr

		if i == 0 {
			cmd.Stdin = os.Stdin
		} else {
			cmd.Stdin = pipes[i-1].r
		}
		if i == n-1 {
			cmd.Stdout = os.Stdout
			if err := applyRedirectsToCmd(cmd, seg.Stdout, seg.Stderr); err != nil {
				fmt.Fprintln(os.Stderr, "kli: redirect:", err)
				return 1
			}
		} else {
			cmd.Stdout = pipes[i].w
			if !seg.Stderr.IsZero() {
				if err := applyRedirectsToCmd(cmd, domain.StreamRedirect{}, seg.Stderr); err != nil {
					fmt.Fprintln(os.Stderr, "kli: redirect:", err)
					return 1
				}
			}
		}

		if err := cmd.Start(); err != nil {
			fmt.Fprintln(os.Stderr, "kli: exec:", err)
			return 1
		}
		cmds[i] = cmd

		// Close the pipe ends we've handed to the child so they don't linger.
		if i < n-1 {
			pipes[i].w.Close() // child's stdout goes into the pipe
		}
		if i > 0 {
			pipes[i-1].r.Close() // child's stdin came from the pipe
		}
	}

	// Wait for all; the pipeline exit code is the last command's exit code.
	code := 0
	for i, cmd := range cmds {
		err := cmd.Wait()
		if i == n-1 && err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				code = exitErr.ExitCode()
			} else {
				code = 1
			}
		}
	}
	return code
}

// segExpandedArgvEnv returns the expanded argv and environment for a ChainLink.
func segExpandedArgvEnv(seg domain.ChainLink) (argv, env []string) {
	rawArgv := make([]string, 0, 1+len(seg.Args))
	rawArgv = append(rawArgv, seg.Command)
	for _, a := range seg.Args {
		rawArgv = append(rawArgv, a.RawDisplay())
	}

	var rawEnv []string
	for _, e := range seg.Env {
		if e.Key != "" {
			rawEnv = append(rawEnv, e.RawDisplay())
		}
	}

	env = domain.ExpandEnvAssignments(rawEnv)
	argv = domain.ExpandExecArgvWithEnv(rawArgv, env)
	return
}

func mergedEnv(base []string, overrides []string) []string {
	result := append([]string(nil), base...)
	index := make(map[string]int, len(base))
	for i, item := range result {
		key, _, ok := strings.Cut(item, "=")
		if ok {
			index[key] = i
		}
	}

	for _, item := range overrides {
		key, _, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		if i, exists := index[key]; exists {
			result[i] = item
			continue
		}
		index[key] = len(result)
		result = append(result, item)
	}
	return result
}
