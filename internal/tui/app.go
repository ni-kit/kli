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

const kliTitle = "  kli — command history manager"

type App struct {
	screen        screen
	invocations   []domain.Invocation
	historySvc    service.HistoryService
	history       historyModel
	detail        detailModel
	width         int
	height        int
	histExecInv   *domain.Invocation // non-nil when exec was requested from history screen
	initialSearch string
	startOnDetail *domain.Invocation // non-nil → open detail screen immediately
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
		a.detail = newDetailModel(*a.startOnDetail, a.width, a.height)
	}
	return nil
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.history.setSize(msg.Width, msg.Height)
		a.detail.setSize(msg.Width, msg.Height)
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
					a.detail = newDetailModel(*inv, a.width, a.height)
					a.screen = screenDetail
				}
				return a, nil
			case msg.String() == "x":
				inv := a.history.selectedInvocation()
				if inv != nil {
					a.histExecInv = inv
					return a, tea.Quit
				}
				return a, nil
			case msg.String() == "a":
				detail, cmd := newDetailModel(service.NewBlankInvocation(), a.width, a.height).WithCommandEditing()
				a.detail = detail
				a.screen = screenDetail
				return a, cmd
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
		if a.screen == screenDetail {
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

// ExecRequested reports whether the user asked to execute a command.
func (a *App) ExecRequested() bool {
	return a.histExecInv != nil || (a.screen == screenDetail && a.detail.ExecRequested())
}

// ExecInvocation returns the invocation the user wants to execute.
// Callers must check ExecRequested() first.
func (a *App) ExecInvocation() domain.Invocation {
	if a.histExecInv != nil {
		return *a.histExecInv
	}
	return a.detail.CurrentInvocation()
}

func (a *App) saveDetail() {
	orig := a.detail.OriginalInvocation()
	updated := a.detail.CurrentInvocation()
	updated.Runs = []domain.Run{orig.LastRun()}
	_ = a.historySvc.SaveEdited(orig, updated)

	if invs, err := a.historySvc.All(); err == nil {
		a.invocations = invs
		a.history.reloadInvocations(invs)
	}
}
