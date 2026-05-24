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
		if err := historySvc.Record(service.Parse(opts.execArgv)); err != nil {
			fmt.Fprintln(os.Stderr, "kli: failed to save history:", err)
		}
		execArgv(opts.execArgv)
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
		inv, err := latestInvocation(invocations, opts.currentDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		if opts.latestAction == latestEcho {
			fmt.Println(strings.Join(inv.RawArgv(), " "))
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
			argv := inv.RawArgv()
			if err := historySvc.Record(service.Parse(argv)); err != nil {
				fmt.Fprintln(os.Stderr, "kli: failed to save history:", err)
			}
			execArgv(argv)
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

func latestInvocation(invocations []domain.Invocation, currentDirOnly bool) (*domain.Invocation, error) {
	if len(invocations) == 0 {
		return nil, fmt.Errorf("kli: no history yet")
	}
	if !currentDirOnly {
		return &invocations[0], nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("kli: cannot resolve current directory: %w", err)
	}

	var best *domain.Invocation
	var bestRun domain.Run
	for i := range invocations {
		run, ok := invocations[i].LastRunInDir(cwd)
		if !ok {
			continue
		}
		if best == nil || run.RunAt.After(bestRun.RunAt) {
			best = &invocations[i]
			bestRun = run
		}
	}
	if best == nil {
		return nil, fmt.Errorf("kli: no history for current directory")
	}
	return best, nil
}

func confirmAction(inv *domain.Invocation, action latestAction) (bool, error) {
	fmt.Println("$", strings.Join(inv.RawArgv(), " "))
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
		executed := service.Parse(app.RecordArgv())
		if err := historySvc.Record(executed); err != nil {
			fmt.Fprintln(os.Stderr, "kli: failed to save history:", err)
		}

		execArgv(argv)
	}
}

func execArgv(argv []string) {
	fmt.Println("$", strings.Join(argv, " "))

	bin, err := exec.LookPath(argv[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "kli: command not found:", argv[0])
		os.Exit(1)
	}
	if err := syscall.Exec(bin, argv, os.Environ()); err != nil {
		fmt.Fprintln(os.Stderr, "kli: exec failed:", err)
		os.Exit(1)
	}
}
