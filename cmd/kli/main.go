package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	tea "charm.land/bubbletea/v2"

	"github.com/ni-kit/kli/internal/domain"
	"github.com/ni-kit/kli/internal/repo"
	"github.com/ni-kit/kli/internal/service"
	"github.com/ni-kit/kli/internal/tui"
)

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

type latestAction int

const (
	latestNone latestAction = iota
	latestOpen
	latestExec
	latestEcho
)

type cliOptions struct {
	importHistory bool
	execArgv      []string
	recordArgv    []string
	openRecorded  bool
	latestAction  latestAction
	confirm       bool
	currentDir    bool
	initialSearch string
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

	opts := parseCLIOptions(os.Args[1:])
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
		execArgv(inv.ExpandedArgv(), inv.ExpandedEnv(), inv.Stdout, inv.Stderr)
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
			rawTokens := inv.RawCommandTokens()
			if err := historySvc.Record(service.Parse(rawTokens)); err != nil {
				fmt.Fprintln(os.Stderr, "kli: failed to save history:", err)
			}
			execArgv(inv.ExpandedArgv(), inv.ExpandedEnv(), inv.Stdout, inv.Stderr)
		}

		runApp(tui.NewAppOnDetail(*inv, invocations, historySvc), historySvc)
		return
	}

	var app *tui.App
	if opts.openRecorded {
		if len(invocations) == 0 {
			fmt.Fprintln(os.Stderr, "kli: no history yet")
			os.Exit(1)
		}
		app = tui.NewAppOnDetail(invocations[0], invocations, historySvc)
	} else if opts.initialSearch != "" {
		app = tui.NewAppWithSearch(invocations, historySvc, opts.initialSearch)
	} else {
		app = tui.NewApp(invocations, historySvc)
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
	if len(args) > 1 && (args[0] == "-x" || args[0] == "--exec") {
		return cliOptions{execArgv: args[1:]}
	}
	if len(args) == 1 && args[0] == "." {
		return cliOptions{initialSearch: "P:."}
	}
	if opts, ok := latestOptions(args, "-l", "--latest", latestOpen, true); ok {
		return opts
	}
	if opts, ok := latestOptions(args, "-ly", "", latestOpen, false); ok {
		return opts
	}
	if opts, ok := latestOptions(args, "-lx", "", latestExec, true); ok {
		return opts
	}
	if opts, ok := latestOptions(args, "-lxy", "", latestExec, false); ok {
		return opts
	}
	if opts, ok := latestOptions(args, "-le", "", latestEcho, true); ok {
		return opts
	}
	return cliOptions{recordArgv: args, openRecorded: len(args) > 0}
}

func latestOptions(args []string, short, long string, action latestAction, confirm bool) (cliOptions, bool) {
	if len(args) == 0 {
		return cliOptions{}, false
	}
	if args[0] != short && (long == "" || args[0] != long) {
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
	fmt.Println("$", strings.Join(inv.RawCommandTokens(), " "))
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

func runApp(app *tui.App, historySvc service.HistoryService) {
	p := tea.NewProgram(app)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if app.ExecRequested() {
		argv := app.ExecArgv()
		env := app.ExecEnv()
		stdout := app.ExecStdout()
		stderr := app.ExecStderr()
		executed := service.Parse(app.RecordArgv())
		executed.Stdout = stdout
		executed.Stderr = stderr
		if err := historySvc.Record(executed); err != nil {
			fmt.Fprintln(os.Stderr, "kli: failed to save history:", err)
		}

		execArgv(argv, env, stdout, stderr)
	}
}

func execArgv(argv []string, env []string, stdout, stderr domain.StreamRedirect) {
	var preview []string
	preview = append(preview, env...)
	preview = append(preview, argv...)
	if s := stdout.StdoutShell(); s != "" {
		preview = append(preview, s)
	}
	if s := stderr.StderrShell(); s != "" {
		preview = append(preview, s)
	}
	fmt.Println("$", strings.Join(preview, " "))

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
