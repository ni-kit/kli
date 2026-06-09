package tui

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ni-kit/kli/internal/domain"
)

type invItem struct {
	inv          domain.Invocation
	toggleMarker string
}

func (i invItem) Title() string {
	return fmt.Sprintf("%s%s%-10s  %s%s%s",
		i.toggleMarker,
		envDot(i.inv),
		i.inv.Command,
		i.inv.ArgsPreview(),
		chainListSummary(i.inv.Chain),
		renderTags(i.inv.Tags),
	)
}

func chainListSummary(chain []domain.ChainLink) string {
	if len(chain) == 0 {
		return ""
	}
	var sb strings.Builder
	for _, link := range chain {
		sb.WriteString("  ")
		switch link.Op {
		case domain.ChainAnd:
			sb.WriteString(chainAndStyle.Render("&&"))
		case domain.ChainPipe:
			sb.WriteString(chainPipeStyle.Render("|"))
		}
		sb.WriteString(" ")
		sb.WriteString(chainDimStyle.Render(link.Command))
	}
	return sb.String()
}

func (i invItem) Description() string {
	last := i.inv.LastRun()
	runs := fmt.Sprintf("%d run", len(i.inv.Runs))
	if len(i.inv.Runs) != 1 {
		runs += "s"
	}
	if last.Cwd == "" {
		return fmt.Sprintf("%s  •  %s", last.RunAt.Format("02/01/2006 15:04"), runs)
	}
	return fmt.Sprintf("%s  •  %s  •  %s", last.RunAt.Format("02/01/2006 15:04"), last.Cwd, runs)
}

func (i invItem) FilterValue() string {
	return i.inv.FullCommand()
}

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255"))
	previewStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
	previewHint  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	searchStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	searchActive = lipgloss.NewStyle().Foreground(lipgloss.Color("111"))
	searchHelp   = lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	tagEditStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("178"))
	envDotStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("178"))
	toggleCur    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("113"))
	toggleAlt    = lipgloss.NewStyle().Foreground(lipgloss.Color("178"))

	tagColors = []lipgloss.Style{
		lipgloss.NewStyle().Foreground(lipgloss.Color("178")), // yellow
		lipgloss.NewStyle().Foreground(lipgloss.Color("74")),  // blue
		lipgloss.NewStyle().Foreground(lipgloss.Color("113")), // green
		lipgloss.NewStyle().Foreground(lipgloss.Color("204")), // red
		lipgloss.NewStyle().Foreground(lipgloss.Color("215")), // orange
		lipgloss.NewStyle().Foreground(lipgloss.Color("141")), // purple
		lipgloss.NewStyle().Foreground(lipgloss.Color("81")),  // cyan
		lipgloss.NewStyle().Foreground(lipgloss.Color("210")), // pink
		lipgloss.NewStyle().Foreground(lipgloss.Color("149")), // lime
		lipgloss.NewStyle().Foreground(lipgloss.Color("221")), // gold
		lipgloss.NewStyle().Foreground(lipgloss.Color("111")), // periwinkle
		lipgloss.NewStyle().Foreground(lipgloss.Color("203")), // salmon
		lipgloss.NewStyle().Foreground(lipgloss.Color("79")),  // teal
		lipgloss.NewStyle().Foreground(lipgloss.Color("183")), // lavender
		lipgloss.NewStyle().Foreground(lipgloss.Color("208")), // amber
		lipgloss.NewStyle().Foreground(lipgloss.Color("117")), // sky
	}

	tagColorMap = map[string]lipgloss.Style{}
)

func envDot(inv domain.Invocation) string {
	if len(inv.Env) == 0 {
		return "  "
	}
	return envDotStyle.Render("•") + " "
}

func buildTagColors(invocations []domain.Invocation) {
	seen := map[string]struct{}{}
	for _, inv := range invocations {
		for _, t := range inv.Tags {
			seen[t] = struct{}{}
		}
	}
	sorted := make([]string, 0, len(seen))
	for t := range seen {
		sorted = append(sorted, t)
	}
	slices.Sort(sorted)
	tagColorMap = make(map[string]lipgloss.Style, len(sorted))
	for i, t := range sorted {
		tagColorMap[t] = tagColors[i%len(tagColors)]
	}
}

func tagColor(tag string) lipgloss.Style {
	if s, ok := tagColorMap[tag]; ok {
		return s
	}
	return tagColors[0]
}

func renderTags(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(" ")
	b.WriteString(tagEditStyle.Render("["))
	for i, t := range tags {
		if i > 0 {
			b.WriteString(tagEditStyle.Render(","))
		}
		b.WriteString(tagColor(t).Render(t))
	}
	b.WriteString(tagEditStyle.Render("]"))
	return b.String()
}

const previewLines = 3 // title + 2 content rows

type historyMode int

const (
	histModeNormal historyMode = iota
	histModeSearch
	histModeEditTags
)

type historyModel struct {
	allInvocations []domain.Invocation
	filtered       []domain.Invocation
	list           list.Model
	width          int
	height         int
	mode           historyMode
	searchInput    textinput.Model
	tagInput       textinput.Model
	pendingD       bool
	cwd            string
	status         string
	toggle         *domain.Toggle
	tagTargetID    string
	tagTargetFP    string
}

func newHistoryModel(invocations []domain.Invocation, width, height int, initialSearch string, toggle *domain.Toggle) historyModel {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetSpacing(0)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color("#dddddd"))

	l := list.New(nil, delegate, width, max(1, height-previewLines))
	l.SetShowTitle(false)
	l.SetShowStatusBar(true)
	l.SetFilteringEnabled(false) // we do filtering ourselves
	l.KeyMap = historyKeyMap()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{histKeys.New}
	}

	si := textinput.New()
	si.Placeholder = "search… (C:cmd F:flag V:val P:path T:tag D:date R:asc/desc)"
	si.CharLimit = 200
	si.SetValue(initialSearch)

	ti := textinput.New()
	ti.Placeholder = "tag name"
	ti.CharLimit = 64

	cwd, _ := os.Getwd()
	buildTagColors(invocations)
	m := historyModel{
		allInvocations: invocations,
		list:           l,
		width:          width,
		height:         height,
		searchInput:    si,
		tagInput:       ti,
		cwd:            cwd,
		toggle:         toggle,
	}
	m.applyFilter()
	return m
}

func (m *historyModel) applyFilter() {
	selected := commandRef{}
	if inv := m.selectedInvocation(); inv != nil {
		selected = refForInvocation(*inv)
	}

	raw := m.searchInput.Value()
	if raw == "" {
		m.filtered = m.allInvocations
	} else {
		q := domain.ParseQuery(raw, m.cwd)
		m.filtered = q.Filter(m.allInvocations)
		q.Sort(m.filtered)
	}

	items := make([]list.Item, len(m.filtered))
	for i, inv := range m.filtered {
		items[i] = invItem{inv: inv, toggleMarker: m.toggleMarker(inv)}
	}
	m.list.SetItems(items)
	if selected.valid() {
		m.selectInvocationRef(selected)
	}
}

func (m historyModel) toggleMarker(inv domain.Invocation) string {
	if m.toggle == nil {
		return ""
	}
	fp := inv.CommandFingerprint()
	if m.toggle.Zero != nil && m.toggle.Zero.CommandFingerprint() == fp {
		return renderToggleMarker(domain.ToggleStateZero, m.toggle.State) + " "
	}
	if m.toggle.One != nil && m.toggle.One.CommandFingerprint() == fp {
		return renderToggleMarker(domain.ToggleStateOne, m.toggle.State) + " "
	}
	return ""
}

func renderToggleMarker(memberState, currentState domain.ToggleState) string {
	marker := fmt.Sprintf("(%d)", memberState)
	if memberState == currentState {
		return toggleCur.Render(marker)
	}
	return toggleAlt.Render(marker)
}

func (m historyModel) Update(msg tea.Msg) (historyModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch m.mode {
		case histModeSearch:
			return m.updateSearch(msg)
		case histModeEditTags:
			return m.updateEditTags(msg)
		default:
			return m.updateNormal(msg)
		}
	default:
		if m.mode == histModeSearch {
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			return m, cmd
		}
		if m.mode == histModeEditTags {
			var cmd tea.Cmd
			m.tagInput, cmd = m.tagInput.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}
}

func (m historyModel) updateNormal(msg tea.KeyPressMsg) (historyModel, tea.Cmd) {
	switch {
	case msg.String() == "/":
		m.pendingD = false
		m.mode = histModeSearch
		cmd := m.searchInput.Focus()
		return m, cmd
	case key.Matches(msg, histKeys.Tag):
		m.pendingD = false
		m.status = ""
		if inv := m.selectedInvocation(); inv != nil {
			m.mode = histModeEditTags
			m.tagTargetID = inv.ID
			m.tagTargetFP = inv.CommandFingerprint()
			m.tagInput.SetValue(strings.Join(inv.Tags, ", "))
			cmd := m.tagInput.Focus()
			return m, cmd
		}
	case key.Matches(msg, histKeys.ToggleAdd):
		m.pendingD = false
		if inv := m.selectedInvocation(); inv != nil {
			m.status = "added to toggle tuple for this directory"
			return m, addToggleMsg(*inv)
		}
	case key.Matches(msg, histKeys.Delete):
		if m.pendingD {
			m.pendingD = false
			if inv := m.selectedInvocation(); inv != nil {
				return m, deleteInvCmd(refForInvocation(*inv))
			}
		} else {
			m.pendingD = true
		}
		return m, nil
	default:
		m.pendingD = false
		m.status = ""
	}
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m historyModel) updateSearch(msg tea.KeyPressMsg) (historyModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.mode = histModeNormal
		m.searchInput.Blur()
		return m, nil
	case "esc":
		m.mode = histModeNormal
		m.searchInput.SetValue("")
		m.searchInput.Blur()
		m.applyFilter()
		return m, nil
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	m.applyFilter()
	return m, cmd
}

func (m historyModel) updateEditTags(msg tea.KeyPressMsg) (historyModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		raw := m.tagInput.Value()
		targetID := m.tagTargetID
		targetFP := m.tagTargetFP
		m.tagTargetID = ""
		m.tagTargetFP = ""
		m.tagInput.Blur()
		m.mode = histModeNormal
		if targetID != "" {
			tags := parseTags(raw)
			return m, setTagsMsg(commandRef{id: targetID, fingerprint: targetFP}, tags)
		}
		return m, nil
	case "esc":
		m.tagTargetID = ""
		m.tagTargetFP = ""
		m.tagInput.Blur()
		m.mode = histModeNormal
		return m, nil
	}
	var cmd tea.Cmd
	m.tagInput, cmd = m.tagInput.Update(msg)
	return m, cmd
}

func parseTags(raw string) []string {
	parts := strings.Split(raw, ",")
	var tags []string
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

func (m historyModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(kliTitle) + "\n")

	switch m.mode {
	case histModeSearch:
		b.WriteString(searchActive.Render("  / ") + m.searchInput.View() + "\n")
		b.WriteString(searchHelp.Render("  C:cmd  F:flag  V:val  P:path  T:tag  D:date  R:asc/R:desc  —  enter: confirm  esc: clear") + "\n")
	case histModeEditTags:
		b.WriteString(tagEditStyle.Render("  tags: ") + m.tagInput.View() + "\n")
		b.WriteString(searchHelp.Render("  comma-separated  —  enter: save  esc: cancel") + "\n")
	default:
		raw := m.searchInput.Value()
		if inv := m.selectedInvocation(); inv != nil {
			maxW := m.width - 2
			if maxW < 10 {
				maxW = 10
			}
			preview := wrapText(inv.FullCommand(), maxW)[0] // first line only
			b.WriteString(previewStyle.Render("  "+preview) + "\n")
			hint := "  enter: edit  •  x: exec  •  /: search  •  t: edit tags  •  T: add toggle  •  dd: delete"
			if m.pendingD {
				b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("  dd: press d again to confirm delete") + "\n")
				b.WriteString(m.list.View())
				return b.String()
			}
			if m.status != "" {
				hint += searchStyle.Render("  [" + m.status + "]")
			}
			if raw != "" {
				hint += searchStyle.Render("  [" + raw + "]")
			}
			b.WriteString(previewHint.Render(hint) + "\n")
		} else {
			hint := "  /: search  •  t: edit tags  •  T: add toggle"
			if raw != "" {
				hint += searchStyle.Render("  [" + raw + "]")
			}
			b.WriteString(previewHint.Render(hint) + "\n")
			b.WriteString("\n")
		}
	}

	b.WriteString(m.list.View())
	return b.String()
}

func (m historyModel) selectedInvocation() *domain.Invocation {
	item := m.list.SelectedItem()
	if item == nil {
		return nil
	}
	ii := item.(invItem)
	return &ii.inv
}

func (m *historyModel) selectInvocationRef(ref commandRef) {
	for i, inv := range m.filtered {
		if ref.matches(inv) {
			m.list.Select(i)
			return
		}
	}
}

func (m *historyModel) setSize(w, h int) {
	m.width = w
	m.height = h
	m.list.SetWidth(w)
	m.list.SetHeight(max(1, h-previewLines))
}

func (m *historyModel) reloadInvocations(invs []domain.Invocation) {
	buildTagColors(invs)
	m.allInvocations = invs
	m.applyFilter()
}

func (m *historyModel) setToggle(toggle *domain.Toggle) {
	m.toggle = toggle
	m.applyFilter()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

type setTagsInvMsg struct {
	ref  commandRef
	tags []string
}

func setTagsMsg(ref commandRef, tags []string) tea.Cmd {
	return func() tea.Msg { return setTagsInvMsg{ref: ref, tags: tags} }
}

type deleteInvMsg struct{ ref commandRef }

func deleteInvCmd(ref commandRef) tea.Cmd {
	return func() tea.Msg { return deleteInvMsg{ref: ref} }
}

type addToggleInvMsg struct{ inv domain.Invocation }

func addToggleMsg(inv domain.Invocation) tea.Cmd {
	return func() tea.Msg { return addToggleInvMsg{inv: inv} }
}

type commandRef struct {
	id          string
	fingerprint string
}

func refForInvocation(inv domain.Invocation) commandRef {
	return commandRef{id: inv.ID, fingerprint: inv.CommandFingerprint()}
}

func (r commandRef) valid() bool {
	return r.id != "" || r.fingerprint != ""
}

func (r commandRef) matches(inv domain.Invocation) bool {
	return inv.ID == r.id && (r.fingerprint == "" || inv.CommandFingerprint() == r.fingerprint)
}
