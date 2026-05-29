package tui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/ni-kit/kli/internal/domain"
	"github.com/ni-kit/kli/internal/service"
)

type screen int

const (
	screenHistory screen = iota
	screenDetail
)

type App struct {
	screen         screen
	invocations    []domain.Invocation
	historySvc     service.HistoryService
	history        historyModel
	detail         detailModel
	width          int
	height         int
	histExecArgv   []string
	histRecordArgv []string
	initialSearch  string
	startOnDetail  *domain.Invocation // non-nil → open detail screen immediately
}

func NewApp(invocations []domain.Invocation, historySvc service.HistoryService) *App {
	return NewAppWithSearch(invocations, historySvc, "")
}

func NewAppWithSearch(invocations []domain.Invocation, historySvc service.HistoryService, initialSearch string) *App {
	return &App{
		screen:        screenHistory,
		invocations:   invocations,
		historySvc:    historySvc,
		initialSearch: initialSearch,
	}
}

func NewAppOnDetail(inv domain.Invocation, invocations []domain.Invocation, historySvc service.HistoryService) *App {
	return &App{
		screen:        screenDetail,
		invocations:   invocations,
		historySvc:    historySvc,
		startOnDetail: &inv,
	}
}

func (a *App) Init() tea.Cmd {
	a.history = newHistoryModel(a.invocations, a.width, a.height, a.initialSearch)
	if a.startOnDetail != nil {
		a.detail = newDetailModel(*a.startOnDetail, a.width)
	}
	return nil
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.history.setSize(msg.Width, msg.Height)
		return a, nil

	case setTagsInvMsg:
		if err := a.historySvc.SetTags(msg.id, msg.tags); err == nil {
			if invs, err := a.historySvc.All(); err == nil {
				a.invocations = invs
				a.history.reloadInvocations(invs)
			}
		}
		return a, nil

	case deleteInvMsg:
		if err := a.historySvc.Delete(msg.id); err == nil {
			if invs, err := a.historySvc.All(); err == nil {
				a.invocations = invs
				a.history.reloadInvocations(invs)
			}
		}
		return a, nil

	case tea.KeyPressMsg:
		switch a.screen {
		case screenHistory:
			if a.history.mode != histModeNormal {
				var cmd tea.Cmd
				a.history, cmd = a.history.Update(msg)
				return a, cmd
			}
			switch {
			case key.Matches(msg, appKeys.Quit):
				return a, tea.Quit
			case key.Matches(msg, appKeys.Enter):
				inv := a.history.selectedInvocation()
				if inv != nil {
					a.detail = newDetailModel(*inv, a.width)
					a.screen = screenDetail
				}
				return a, nil
			case msg.String() == "x":
				inv := a.history.selectedInvocation()
				if inv != nil {
					a.histExecArgv = inv.ExpandedArgv()
					a.histRecordArgv = inv.RawArgv()
					return a, tea.Quit
				}
				return a, nil
			default:
				var cmd tea.Cmd
				a.history, cmd = a.history.Update(msg)
				return a, cmd
			}

		case screenDetail:
			if a.detail.IsEditing() {
				if msg.String() == "ctrl+c" {
					return a, tea.Quit
				}
				var cmd tea.Cmd
				a.detail, cmd = a.detail.Update(msg)
				return a, cmd
			}
			switch {
			case key.Matches(msg, appKeys.Back):
				a.screen = screenHistory
				return a, nil
			case key.Matches(msg, appKeys.Quit):
				return a, tea.Quit
			default:
				var cmd tea.Cmd
				a.detail, cmd = a.detail.Update(msg)
				if a.detail.SaveRequested() {
					a.saveDetail()
				}
				return a, cmd
			}
		}

	default:
		if a.screen == screenDetail && a.detail.IsEditing() {
			var cmd tea.Cmd
			a.detail, cmd = a.detail.Update(msg)
			return a, cmd
		}
	}

	if a.screen == screenHistory {
		var cmd tea.Cmd
		a.history, cmd = a.history.Update(msg)
		return a, cmd
	}
	return a, nil
}

func (a *App) View() tea.View {
	var content string
	switch a.screen {
	case screenDetail:
		content = a.detail.View()
	default:
		content = a.history.View()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (a *App) ExecRequested() bool {
	return len(a.histExecArgv) > 0 || (a.screen == screenDetail && a.detail.ExecRequested())
}

func (a *App) ExecArgv() []string {
	if len(a.histExecArgv) > 0 {
		return a.histExecArgv
	}
	return a.detail.ExecArgv()
}

func (a *App) saveDetail() {
	orig := a.detail.OriginalInvocation()
	currentArgs := a.detail.CurrentArgs()

	candidate := domain.Invocation{Command: orig.Command, Args: currentArgs}
	if candidate.CommandFingerprint() == orig.CommandFingerprint() {
		// only layout changed (reordering) — mutate in place
		_ = a.historySvc.SaveLayout(orig.ID, currentArgs)
	} else {
		// values/flags changed — record as a new entry
		_ = a.historySvc.Record(candidate)
	}

	if invs, err := a.historySvc.All(); err == nil {
		a.invocations = invs
		a.history.reloadInvocations(invs)
	}
}

// RecordArgv returns the unexpanded argv to save to history (env vars kept as-is).
func (a *App) RecordArgv() []string {
	if len(a.histRecordArgv) > 0 {
		return a.histRecordArgv
	}
	return a.detail.rawArgv()
}
