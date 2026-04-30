package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	tea "charm.land/bubbletea/v2"

	"github.com/ni-kit/kli/internal/repo"
	"github.com/ni-kit/kli/internal/service"
	"github.com/ni-kit/kli/internal/tui"
)

func main() {
	histPath, err := repo.HistoryPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "kli: cannot resolve history path:", err)
		os.Exit(1)
	}
	histRepo := repo.NewJSONHistoryRepo(histPath)
	historySvc := service.NewHistoryService(histRepo)
	importSvc := service.NewImportService(histRepo)

	args := os.Args[1:]

	if len(args) > 0 && args[0] == "--import" {
		n, err := importSvc.ImportShellHistory()
		if err != nil {
			fmt.Fprintln(os.Stderr, "kli: import failed:", err)
			os.Exit(1)
		}
		fmt.Printf("kli: imported %d new entries\n", n)
		os.Exit(0)
	}

	if len(args) > 1 && (args[0] == "-x" || args[0] == "--exec") {
		argv := args[1:]
		parsed := service.Parse(argv)
		if err := historySvc.Record(parsed); err != nil {
			fmt.Fprintln(os.Stderr, "kli: failed to save history:", err)
		}
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

	latest := len(args) > 0 && (args[0] == "-l" || args[0] == "--latest")
	latestExec := len(args) > 0 && args[0] == "-lx"

	isFlag := latest || latestExec
	if len(args) > 0 && !isFlag {
		parsed := service.Parse(args)
		if err := historySvc.Record(parsed); err != nil {
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

	if latestExec {
		if len(invocations) == 0 {
			fmt.Fprintln(os.Stderr, "kli: no history yet")
			os.Exit(1)
		}
		inv := invocations[0]
		argv := make([]string, 0, 1+len(inv.Args))
		argv = append(argv, inv.Command)
		for _, arg := range inv.Args {
			argv = append(argv, arg.Display())
		}
		executed := service.Parse(argv)
		if err := historySvc.Record(executed); err != nil {
			fmt.Fprintln(os.Stderr, "kli: failed to save history:", err)
		}
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

	var app *tui.App
	if latest {
		if len(invocations) == 0 {
			fmt.Fprintln(os.Stderr, "kli: no history yet")
			os.Exit(1)
		}
		app = tui.NewAppOnDetail(invocations[0], invocations, historySvc)
	} else {
		app = tui.NewApp(invocations, historySvc)
	}

	p := tea.NewProgram(app)
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if app.ExecRequested() {
		argv := app.ExecArgv()
		executed := service.Parse(argv)
		if err := historySvc.Record(executed); err != nil {
			fmt.Fprintln(os.Stderr, "kli: failed to save history:", err)
		}

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
}
