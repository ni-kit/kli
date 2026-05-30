package tui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/atotto/clipboard"

	"github.com/ni-kit/kli/internal/domain"
)

const (
	colNameW  = 22
	colValueW = 36
)

var (
	headerStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	sectionStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245"))
	locationStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("108"))
	hintStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	cellSelected   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("33"))
	cellName       = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	cellValue      = lipgloss.NewStyle().Foreground(lipgloss.Color("220"))
	cellEnvName    = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	cellEnvValue   = lipgloss.NewStyle().Foreground(lipgloss.Color("228"))
	cellEnvEmpty   = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Italic(true)
	cellDisabled   = lipgloss.NewStyle().Foreground(lipgloss.Color("238")).Strikethrough(true)
	disabledMark   = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	cellEnvHint    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cellHeader     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("245"))
	rowActiveStyle = lipgloss.NewStyle().Background(lipgloss.Color("237"))

	runBtnNormal  = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	runBtnFocused = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("34")).Padding(0, 2)

	detailKeys = detailKeyMap{
		Up:          key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:        key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Left:        key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "left")),
		Right:       key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "right")),
		Enter:       key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "edit/run")),
		Insert:      key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "edit cell")),
		C:           key.NewBinding(key.WithKeys("c"), key.WithHelp("ci", "change cell")),
		D:           key.NewBinding(key.WithKeys("d"), key.WithHelp("dd", "delete row")),
		Paste:       key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "paste")),
		Yank:        key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy cell")),
		YankCmd:     key.NewBinding(key.WithKeys("Y"), key.WithHelp("Y", "copy command")),
		AddRow:      key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add flag/value below")),
		X:           key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "exec")),
		Undo:        key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "undo")),
		ToggleRow:   key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "toggle row")),
		ToggleValue: key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "secret value")),
		Save:        key.NewBinding(key.WithKeys("S"), key.WithHelp("S", "save")),
		PushCell:    key.NewBinding(key.WithKeys("M"), key.WithHelp("M", "push cell to next line")),
		MergeBack:   key.NewBinding(key.WithKeys("m"), key.WithHelp("m", "merge flag into previous row")),
		Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
	}
)

const numCols = 2

type displayRowKind int

const (
	rowArg displayRowKind = iota
	rowEnv
	rowCommand
)

type detailMode int

const (
	modeNormal detailMode = iota
	modeEditing
)

type detailKeyMap struct {
	Up          key.Binding
	Down        key.Binding
	Left        key.Binding
	Right       key.Binding
	Enter       key.Binding
	Insert      key.Binding
	C           key.Binding
	D           key.Binding
	Paste       key.Binding
	Yank        key.Binding
	YankCmd     key.Binding
	AddRow      key.Binding
	X           key.Binding
	Undo        key.Binding
	ToggleRow   key.Binding
	ToggleValue key.Binding
	Save        key.Binding
	PushCell    key.Binding
	MergeBack   key.Binding
	Help        key.Binding
}

type displayRow struct {
	kind     displayRowKind
	name     string
	value    string
	disabled bool // excluded from exec and copy
	secret   bool // value passed to exec but shown as •••• in copy
}

func buildEnvRows(env []domain.EnvVar) []displayRow {
	rows := make([]displayRow, 0, max(1, len(env)))
	for _, e := range env {
		rows = append(rows, displayRow{kind: rowEnv, name: e.Key, value: e.Value, secret: e.Redacted})
	}
	if len(rows) == 0 {
		rows = append(rows, displayRow{kind: rowEnv})
	}
	return rows
}

func buildCommandRow(command string) displayRow {
	return displayRow{kind: rowCommand, name: "command", value: command}
}

func buildRows(args []domain.Arg) []displayRow {
	rows := make([]displayRow, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a.Kind {
		case domain.ArgShortFlag, domain.ArgLongFlag:
			name := flagLabel(a)
			value := ""
			secret := a.Redacted
			if i+1 < len(args) && args[i+1].Kind == domain.ArgFlagValue {
				i++
				value = args[i].Value
				if args[i].Redacted {
					secret = true
				}
			}
			rows = append(rows, displayRow{kind: rowArg, name: name, value: value, secret: secret})
		case domain.ArgPositional:
			rows = append(rows, displayRow{kind: rowArg, name: "", value: a.Value, secret: a.Redacted})
		case domain.ArgFlagValue:
			rows = append(rows, displayRow{kind: rowArg, name: "", value: a.Value, secret: a.Redacted})
		}
	}
	return rows
}

func rowsToArgs(rows []displayRow) []domain.Arg {
	args := make([]domain.Arg, 0, len(rows)*2)
	for _, dr := range rows {
		if dr.kind != rowArg {
			continue
		}
		if dr.name == "" {
			if dr.value != "" {
				args = append(args, domain.Arg{Kind: domain.ArgPositional, Value: dr.value, Redacted: dr.secret})
			}
			continue
		}
		switch {
		case len(dr.name) >= 2 && dr.name[:2] == "--":
			args = append(args, domain.Arg{Kind: domain.ArgLongFlag, Name: dr.name[2:], Redacted: dr.secret})
		case dr.name[0] == '-':
			args = append(args, domain.Arg{Kind: domain.ArgShortFlag, Name: dr.name[1:], Redacted: dr.secret})
		default:
			args = append(args, domain.Arg{Kind: domain.ArgPositional, Value: dr.name, Redacted: dr.secret})
		}
		if dr.value != "" {
			args = append(args, domain.Arg{Kind: domain.ArgFlagValue, Value: dr.value, Redacted: dr.secret})
		}
	}
	return args
}

func rowsToEnv(rows []displayRow) []domain.EnvVar {
	env := make([]domain.EnvVar, 0, len(rows))
	for _, dr := range rows {
		if dr.kind != rowEnv || dr.disabled || dr.name == "" {
			continue
		}
		env = append(env, domain.EnvVar{Key: dr.name, Value: dr.value, Redacted: dr.secret})
	}
	return env
}

func rowsCommand(rows []displayRow, fallback string) string {
	for _, dr := range rows {
		if dr.kind == rowCommand {
			return dr.value
		}
	}
	return fallback
}

func hasEnvRow(rows []displayRow) bool {
	for _, dr := range rows {
		if dr.kind == rowEnv {
			return true
		}
	}
	return false
}

func hasCommandRow(rows []displayRow) bool {
	for _, dr := range rows {
		if dr.kind == rowCommand {
			return true
		}
	}
	return false
}

func countEnvRows(rows []displayRow) int {
	n := 0
	for _, dr := range rows {
		if dr.kind == rowEnv {
			n++
		}
	}
	return n
}

func flagLabel(a domain.Arg) string {
	switch a.Kind {
	case domain.ArgShortFlag:
		return "-" + a.Name
	case domain.ArgLongFlag:
		return "--" + a.Name
	}
	return ""
}

type undoEntry struct {
	row, col int
	value    string
	rows     []displayRow // non-nil → row add/delete undo; nil → cell edit undo
	rowPos   int
}

type copiedMsg struct{}
type savedMsg struct{}

type detailModel struct {
	inv           domain.Invocation
	width         int
	rows          []displayRow
	row           int // 0..len(rows)-1 = arg rows; len(rows) = Run button
	col           int
	mode          detailMode
	cArmed        bool
	dArmed        bool
	input         textinput.Model
	execRequested bool
	undo          *undoEntry
	copied        bool
	saved         bool
	helpVisible   bool
}

func (m detailModel) ExecRequested() bool                   { return m.execRequested }
func (m detailModel) SaveRequested() bool                   { return m.saved }
func (m detailModel) ExecArgv() []string                    { return m.liveArgv() }
func (m detailModel) ExecEnv() []string                     { return m.liveEnv() }
func (m detailModel) InvocationID() string                  { return m.inv.ID }
func (m detailModel) OriginalInvocation() domain.Invocation { return m.inv }
func (m detailModel) CurrentCommand() string                { return rowsCommand(m.rows, m.inv.Command) }
func (m detailModel) CurrentEnv() []domain.EnvVar           { return rowsToEnv(m.rows) }
func (m detailModel) CurrentArgs() []domain.Arg             { return rowsToArgs(m.rows) }
func (m detailModel) onButton() bool                        { return m.row == len(m.rows) }

func (m detailModel) onCommandRow() bool {
	return !m.onButton() && m.rows[m.row].kind == rowCommand
}

func (m *detailModel) normalizeCursor() {
	if m.onCommandRow() {
		m.col = 1
	}
}

func newDetailModel(inv domain.Invocation, width int) detailModel {
	ti := textinput.New()
	ti.CharLimit = 256
	return detailModel{
		inv:   inv,
		width: width,
		rows:  append(append(buildEnvRows(inv.Env), buildCommandRow(inv.Command)), buildRows(inv.Args)...),
		input: ti,
	}
}

func (m detailModel) IsEditing() bool { return m.mode == modeEditing }

func (m detailModel) Update(msg tea.Msg) (detailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.mode == modeEditing {
			return m.updateEditing(msg)
		}
		return m.updateNormal(msg)
	case copiedMsg:
		m.copied = false
		return m, nil
	case savedMsg:
		m.saved = false
		return m, nil
	default:
		if m.mode == modeEditing {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m detailModel) updateNormal(msg tea.KeyPressMsg) (detailModel, tea.Cmd) {
	if m.cArmed {
		m.cArmed = false
		if msg.String() == "i" && !m.onButton() {
			m.setCell("")
			return m.startEditing("")
		}
	}
	if m.dArmed {
		m.dArmed = false
		if key.Matches(msg, detailKeys.D) && !m.onButton() && !m.onCommandRow() && len(m.rows) > 0 {
			snapshot := make([]displayRow, len(m.rows))
			copy(snapshot, m.rows)
			m.undo = &undoEntry{rows: snapshot, rowPos: m.row}
			m.rows = append(m.rows[:m.row:m.row], m.rows[m.row+1:]...)
			if !hasEnvRow(m.rows) {
				m.rows = append([]displayRow{{kind: rowEnv}}, m.rows...)
				m.row = 0
			}
			if !hasCommandRow(m.rows) {
				insertAt := min(len(m.rows), countEnvRows(m.rows))
				m.rows = append(m.rows[:insertAt], append([]displayRow{buildCommandRow(m.inv.Command)}, m.rows[insertAt:]...)...)
				m.row = insertAt
			}
			if m.row >= len(m.rows) && m.row > 0 {
				m.row--
			}
			m.normalizeCursor()
			return m, nil
		}
	}

	switch {
	case key.Matches(msg, detailKeys.Up):
		if m.row > 0 {
			m.row--
		}
		m.normalizeCursor()
	case key.Matches(msg, detailKeys.Down):
		if m.row < len(m.rows) {
			m.row++
		}
		m.normalizeCursor()
	case key.Matches(msg, detailKeys.Left):
		if !m.onButton() && !m.onCommandRow() && m.col > 0 {
			m.col--
		}
	case key.Matches(msg, detailKeys.Right):
		if !m.onButton() && !m.onCommandRow() && m.col < numCols-1 {
			m.col++
		}
	case key.Matches(msg, detailKeys.Enter):
		if m.onButton() {
			m.execRequested = true
			return m, tea.Quit
		}
		return m.startEditing(m.currentCell())
	case key.Matches(msg, detailKeys.Insert):
		if !m.onButton() {
			return m.startEditing(m.currentCell())
		}
	case key.Matches(msg, detailKeys.X):
		m.execRequested = true
		return m, tea.Quit
	case key.Matches(msg, detailKeys.C):
		if !m.onButton() {
			m.cArmed = true
		}
	case key.Matches(msg, detailKeys.ToggleRow):
		if !m.onButton() && !m.onCommandRow() {
			m.rows[m.row].disabled = !m.rows[m.row].disabled
		}
	case key.Matches(msg, detailKeys.ToggleValue):
		if !m.onButton() && !m.onCommandRow() {
			m.rows[m.row].secret = !m.rows[m.row].secret
		}
	case key.Matches(msg, detailKeys.Save):
		m.saved = true
		return m, clearSavedAfter()
	case key.Matches(msg, detailKeys.MergeBack):
		m = m.mergeBack()
	case key.Matches(msg, detailKeys.PushCell):
		if !m.onButton() {
			m, _ = m.pushCell()
		}
	case key.Matches(msg, detailKeys.Yank):
		if !m.onButton() {
			clipboard.WriteAll(m.currentCell()) //nolint:errcheck
			m.copied = true
			return m, clearCopiedAfter()
		}
	case key.Matches(msg, detailKeys.YankCmd):
		clipboard.WriteAll(m.copyCommand()) //nolint:errcheck
		m.copied = true
		return m, clearCopiedAfter()
	case key.Matches(msg, detailKeys.Paste):
		if !m.onButton() {
			text, err := clipboard.ReadAll()
			if err == nil && text != "" {
				m.setCell(text)
			}
		}
	case key.Matches(msg, detailKeys.D):
		if !m.onButton() && !m.onCommandRow() {
			m.dArmed = true
		}
	case key.Matches(msg, detailKeys.AddRow):
		if !m.onButton() && !m.onCommandRow() {
			snapshot := make([]displayRow, len(m.rows))
			copy(snapshot, m.rows)
			insertAt := m.row + 1
			m.undo = &undoEntry{rows: snapshot, rowPos: m.row}
			newRows := make([]displayRow, len(m.rows)+1)
			copy(newRows, m.rows[:insertAt])
			newRows[insertAt] = displayRow{kind: m.rows[m.row].kind}
			copy(newRows[insertAt+1:], m.rows[insertAt:])
			m.rows = newRows
			m.row = insertAt
			m.col = 0
			m.normalizeCursor()
			return m.startEditing("")
		}
	case key.Matches(msg, detailKeys.Help):
		m.helpVisible = !m.helpVisible
	case key.Matches(msg, detailKeys.Undo):
		if m.undo != nil {
			if m.undo.rows != nil {
				m.rows = m.undo.rows
				m.row = m.undo.rowPos
			} else {
				if m.undo.col == 0 {
					m.rows[m.undo.row].name = m.undo.value
				} else {
					m.rows[m.undo.row].value = m.undo.value
				}
				m.row, m.col = m.undo.row, m.undo.col
			}
			m.normalizeCursor()
			m.undo = nil
		}
	}
	return m, nil
}

func (m detailModel) currentCell() string {
	if m.onCommandRow() {
		return m.rows[m.row].value
	}
	if m.col == 0 {
		return m.rows[m.row].name
	}
	return m.rows[m.row].value
}

func (m *detailModel) setCell(s string) {
	if m.onCommandRow() {
		m.rows[m.row].value = s
		return
	}
	if m.col == 0 {
		m.rows[m.row].name = s
	} else {
		m.rows[m.row].value = s
	}
}

func (m detailModel) startEditing(initial string) (detailModel, tea.Cmd) {
	m.normalizeCursor()
	m.mode = modeEditing
	m.input.SetValue(initial)
	cmd := m.input.Focus()
	return m, cmd
}

func (m detailModel) updateEditing(msg tea.KeyPressMsg) (detailModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.undo = &undoEntry{row: m.row, col: m.col, value: m.currentCell()}
		m.setCell(m.input.Value())
		m.input.Blur()
		m.mode = modeNormal
		return m, nil
	case "esc":
		m.input.Blur()
		m.mode = modeNormal
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m detailModel) pushCell() (detailModel, tea.Cmd) {
	if m.onButton() || m.rows[m.row].kind != rowArg {
		return m, nil
	}
	cur := m.currentCell()
	snapshot := make([]displayRow, len(m.rows))
	copy(snapshot, m.rows)
	m.undo = &undoEntry{rows: snapshot, rowPos: m.row}

	insertAt := m.row + 1

	if m.col == 0 && isShortFlagCluster(m.rows[m.row]) && len(m.rows[m.row].name) > 2 {
		// cluster: peel last flag letter; reuse next row only if completely empty
		peeled := "-" + string(cur[len(cur)-1])
		if insertAt < len(m.rows) && m.rows[insertAt].name == "" && m.rows[insertAt].value == "" {
			m.rows[m.row].name = cur[:len(cur)-1]
			m.rows[insertAt].name = peeled
		} else {
			newRows := make([]displayRow, len(m.rows)+1)
			copy(newRows, m.rows[:insertAt])
			copy(newRows[insertAt+1:], m.rows[insertAt:])
			newRows[m.row] = displayRow{kind: rowArg, name: cur[:len(cur)-1], value: ""}
			newRows[insertAt] = displayRow{kind: rowArg, name: peeled, value: ""}
			m.rows = newRows
		}
	} else if m.col == 0 && isSingleShortFlag(m.rows[m.row]) && m.rows[m.row].value == "" &&
		insertAt < len(m.rows) && isShortFlagCluster(m.rows[insertAt]) {
		// single short flag pushed into next short flag cluster — merge forward
		m.rows[insertAt].name = "-" + cur[1:] + m.rows[insertAt].name[1:]
		m.rows[m.row].name = ""
	} else if m.col == 0 && insertAt < len(m.rows) && m.rows[insertAt].name == "" {
		// next row has empty flag slot — move flag there without inserting
		m.rows[insertAt].name = cur
		m.rows[m.row].name = ""
	} else if m.col == 1 && insertAt < len(m.rows) && m.rows[insertAt].value == "" {
		// next row has empty value slot — move value there without inserting
		m.rows[insertAt].value = cur
		m.rows[m.row].value = ""
	} else {
		newRows := make([]displayRow, len(m.rows)+1)
		copy(newRows, m.rows[:insertAt])
		copy(newRows[insertAt+1:], m.rows[insertAt:])
		if m.col == 0 {
			newRows[m.row] = displayRow{kind: rowArg, name: "", value: m.rows[m.row].value}
			newRows[insertAt] = displayRow{kind: rowArg, name: cur, value: ""}
		} else {
			newRows[m.row] = displayRow{kind: rowArg, name: m.rows[m.row].name, value: ""}
			newRows[insertAt] = displayRow{kind: rowArg, name: "", value: cur}
		}
		m.rows = newRows
	}

	m.row = insertAt
	// col stays as-is: col=1 lands on the value slot, col=0 on the name slot
	return m, nil
}

func isSingleShortFlag(dr displayRow) bool {
	return len(dr.name) == 2 && dr.name[0] == '-'
}

func isShortFlagCluster(dr displayRow) bool {
	return len(dr.name) >= 2 && dr.name[0] == '-' && dr.name[1] != '-' && dr.value == ""
}

func (m detailModel) mergeBack() detailModel {
	if m.onButton() || m.row == 0 || m.rows[m.row].kind != rowArg || m.rows[m.row-1].kind != rowArg {
		return m
	}
	cur := m.rows[m.row]
	prev := m.rows[m.row-1]

	// value cell: move into previous row's value slot if it's empty
	if m.col == 1 && cur.value != "" && prev.value == "" {
		snapshot := make([]displayRow, len(m.rows))
		copy(snapshot, m.rows)
		m.undo = &undoEntry{rows: snapshot, rowPos: m.row}
		m.rows[m.row-1].value = cur.value
		m.rows[m.row].value = ""
		m.row--
		return m
	}

	isFlag := cur.name != "" && cur.name[0] == '-'
	if !isFlag {
		return m
	}

	// short-flag cluster merge: prev must be a short flag cluster or empty
	isShort := isShortFlagCluster(cur) || isSingleShortFlag(cur)
	prevAcceptsCluster := isShortFlagCluster(prev) || prev.name == ""

	if isShort && prevAcceptsCluster {
		snapshot := make([]displayRow, len(m.rows))
		copy(snapshot, m.rows)
		m.undo = &undoEntry{rows: snapshot, rowPos: m.row}
		if prev.name == "" {
			m.rows[m.row-1].name = cur.name
		} else {
			m.rows[m.row-1].name = prev.name + cur.name[1:]
		}
		if cur.value != "" {
			m.rows[m.row].name = ""
			m.row--
		} else {
			m.rows = append(m.rows[:m.row], m.rows[m.row+1:]...)
			m.row--
		}
		return m
	}

	// any flag: move into previous row if its name is empty
	if prev.name == "" {
		snapshot := make([]displayRow, len(m.rows))
		copy(snapshot, m.rows)
		m.undo = &undoEntry{rows: snapshot, rowPos: m.row}
		m.rows[m.row-1].name = cur.name
		m.rows[m.row].name = ""
		m.row--
		return m
	}

	return m
}

func isBareSingleShortFlag(dr displayRow) bool {
	return len(dr.name) == 2 && dr.name[0] == '-' && dr.value == ""
}

func mergedTokens(rows []displayRow, secretsVisible bool) []string {
	var tokens []string
	i := 0
	for i < len(rows) {
		dr := rows[i]
		if dr.kind != rowArg || dr.disabled {
			i++
			continue
		}
		if isBareSingleShortFlag(dr) {
			cluster := string(dr.name[1]) // single letter
			j := i + 1
			for j < len(rows) && !rows[j].disabled && isBareSingleShortFlag(rows[j]) {
				cluster += string(rows[j].name[1])
				j++
			}
			if len(cluster) > 1 {
				tokens = append(tokens, "-"+cluster)
				i = j
				continue
			}
		}
		val := dr.value
		if !secretsVisible && dr.secret && val != "" {
			val = "••••"
		}
		if dr.name == "" {
			if val != "" {
				tokens = append(tokens, val)
			}
		} else {
			tokens = append(tokens, dr.name)
			if val != "" {
				tokens = append(tokens, val)
			}
		}
		i++
	}
	return tokens
}

func envTokens(rows []displayRow, secretsVisible bool) []string {
	var tokens []string
	for _, dr := range rows {
		if dr.kind != rowEnv || dr.disabled || dr.name == "" {
			continue
		}
		val := dr.value
		if !secretsVisible && dr.secret {
			val = "••••"
		}
		tokens = append(tokens, dr.name+"="+val)
	}
	return tokens
}

func (m detailModel) liveArgv() []string {
	return domain.ExpandExecArgvWithEnv(m.rawArgv(), m.liveEnv())
}

func (m detailModel) liveEnv() []string {
	return domain.ExpandEnvAssignments(envTokens(m.rows, true))
}

func (m detailModel) rawArgv() []string {
	return append([]string{m.CurrentCommand()}, mergedTokens(m.rows, true)...)
}

func (m detailModel) rawCommandTokens() []string {
	return append(envTokens(m.rows, true), m.rawArgv()...)
}

func (m detailModel) copyCommand() string {
	return strings.Join(append(envTokens(m.rows, false), append([]string{m.CurrentCommand()}, mergedTokens(m.rows, false)...)...), " ")
}

func (m detailModel) liveCommand() string {
	rows := make([]displayRow, len(m.rows))
	copy(rows, m.rows)
	if m.mode == modeEditing && !m.onButton() {
		if m.col == 0 {
			rows[m.row].name = m.input.Value()
		} else {
			rows[m.row].value = m.input.Value()
		}
	}
	return strings.Join(append(envTokens(rows, true), append([]string{rowsCommand(rows, m.inv.Command)}, mergedTokens(rows, true)...)...), " ")
}

func envHint(value string, env []string) string {
	if !strings.Contains(value, "$") && !strings.HasPrefix(value, "~") {
		return ""
	}
	expanded := domain.ExpandExecArgvWithEnv([]string{"_", value}, env)[1]
	if expanded == value {
		return ""
	}
	return "→ " + expanded
}

func (m detailModel) View() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(kliTitle) + "\n")
	maxW := m.width - 4
	for _, l := range wrapText(m.liveCommand(), maxW) {
		b.WriteString(headerStyle.Render("  "+l) + "\n")
	}
	last := m.inv.LastRun()
	b.WriteString(hintStyle.Render(fmt.Sprintf("  %s  •  %s", last.RunAt.Format("02/01/2006 15:04:05"), last.Cwd)) + "\n\n")
	env := m.liveEnv()

	b.WriteString(fmt.Sprintf("  %s  %s\n",
		cellHeader.Render(padStr("env var", colNameW)),
		cellHeader.Render(padStr("value", colValueW)),
	))
	b.WriteString("  " + lipgloss.NewStyle().Foreground(lipgloss.Color("178")).Render(strings.Repeat("─", colNameW+colValueW+4)) + "\n")
	for r, dr := range m.rows {
		if dr.kind != rowEnv {
			continue
		}
		b.WriteString(m.renderRow(r, dr, env) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("  Command") + "\n")
	for r, dr := range m.rows {
		if dr.kind != rowCommand {
			continue
		}
		b.WriteString(m.renderCommandRow(r, dr) + "\n")
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s  %s\n",
		cellHeader.Render(padStr("name/flag", colNameW)),
		cellHeader.Render(padStr("value", colValueW)),
	))
	b.WriteString("  " + sectionStyle.Render(strings.Repeat("─", colNameW+colValueW+4)) + "\n")
	for r, dr := range m.rows {
		if dr.kind != rowArg {
			continue
		}
		b.WriteString(m.renderRow(r, dr, env) + "\n")
	}

	b.WriteString("\n")
	btnStyle := runBtnNormal
	if m.onButton() {
		btnStyle = runBtnFocused
	}
	b.WriteString("  " + btnStyle.Render("exec command") + "\n")

	if len(m.inv.Runs) > 1 {
		b.WriteString("\n")
		b.WriteString(sectionStyle.Render("  Run history") + "\n")
		for _, r := range m.inv.Runs {
			b.WriteString(locationStyle.Render(fmt.Sprintf("    • %s  %s", r.RunAt.Format("02/01/2006 15:04:05"), r.Cwd)) + "\n")
		}
	}

	b.WriteString("\n")
	if m.mode == modeEditing {
		b.WriteString(hintStyle.Render("  enter: confirm  •  esc: cancel") + "\n")
	} else if m.copied {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("113")).Bold(true).Render("  ✓ copied!") + "\n")
		b.WriteString("\n")
	} else if m.saved {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("113")).Bold(true).Render("  ✓ saved!") + "\n")
		b.WriteString("\n")
	} else if m.helpVisible {
		b.WriteString(hintStyle.Render("  hjkl/arrows: navigate  •  i/enter: edit  •  ci: change cell  •  a: add row  •  dd: delete row  •  u: undo") + "\n")
		b.WriteString(hintStyle.Render("  m: merge flag back  •  M: push flag forward  •  space: toggle row  •  s: secret  •  S: save") + "\n")
		b.WriteString(hintStyle.Render("  y: copy cell  •  Y: copy cmd  •  p: paste  •  x: exec  •  esc: back  •  q: quit  •  ?: hide") + "\n")
	} else {
		b.WriteString(hintStyle.Render("  hjkl: navigate  •  i: edit  •  a: add row  •  dd: delete  •  m/M: move flag  •  S: save  •  x: exec  •  ?: more") + "\n")
	}

	return b.String()
}

func (m detailModel) renderRow(r int, dr displayRow, env []string) string {
	isActive := r == m.row && !m.onButton()

	var nameText string
	if isActive && m.col == 0 && m.mode == modeEditing {
		nameText = m.input.View()
	} else if dr.kind == rowEnv && dr.name == "" && dr.value == "" {
		nameText = padStr("enter env var here", colNameW)
	} else {
		nameText = padStr(dr.name, colNameW)
	}

	var valText string
	if isActive && m.col == 1 && m.mode == modeEditing {
		valText = m.input.View()
	} else {
		v := dr.value
		if dr.secret {
			v = "••••"
		}
		hint := ""
		if !dr.secret {
			hint = envHint(dr.value, env)
		}
		if hint != "" {
			valText = padStr(v, colValueW) + " " + hint
		} else {
			valText = padStr(v, colValueW)
		}
	}

	var nameCell, valCell string
	switch {
	case dr.disabled:
		nameCell = disabledMark.Render("✗ ") + cellDisabled.Render(padStr(dr.name, colNameW))
		valCell = cellDisabled.Render(padStr(dr.value, colValueW))
	case isActive:
		nameStyle := cellName
		valStyle := cellValue
		if dr.kind == rowEnv {
			nameStyle = cellEnvName
			valStyle = cellEnvValue
		}
		nameCell = styledCell(nameText, 0, m.col, nameStyle)
		valCell = styledCell(valText, 1, m.col, valStyle)
	default:
		nameStyle := cellName
		valStyle := cellValue
		if dr.kind == rowEnv {
			nameStyle = cellEnvName
			valStyle = cellEnvValue
			if dr.name == "" && dr.value == "" {
				nameStyle = cellEnvEmpty
			}
		}
		nameCell = nameStyle.Render(nameText)
		if hint := envHint(dr.value, env); hint != "" && !dr.secret {
			plain := padStr(dr.value, colValueW)
			if dr.secret {
				plain = padStr("••••", colValueW)
			}
			valCell = valStyle.Render(plain) + " " + cellEnvHint.Render(hint)
		} else {
			valCell = valStyle.Render(valText)
		}
	}

	line := fmt.Sprintf("  %s  %s", nameCell, valCell)
	if isActive {
		line = rowActiveStyle.Render(line)
	}
	return line
}

func (m detailModel) renderCommandRow(r int, dr displayRow) string {
	isActive := r == m.row && !m.onButton()
	width := colNameW + colValueW + 2

	var text string
	if isActive && m.mode == modeEditing {
		text = m.input.View()
	} else {
		text = padStr(dr.value, width)
	}

	cell := cellValue.Render(text)
	if isActive {
		cell = cellSelected.Render(text)
	}

	line := "  " + cell
	if isActive {
		line = rowActiveStyle.Render(line)
	}
	return line
}

func styledCell(text string, col, cursorCol int, base lipgloss.Style) string {
	if col == cursorCol {
		return cellSelected.Render(text)
	}
	return base.Render(text)
}

func padStr(s string, w int) string {
	if len(s) >= w {
		return s[:w]
	}
	return s + strings.Repeat(" ", w-len(s))
}

func clearCopiedAfter() tea.Cmd {
	return tea.Tick(3*time.Second, func(time.Time) tea.Msg { return copiedMsg{} })
}

func clearSavedAfter() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return savedMsg{} })
}
