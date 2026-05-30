package tui

import (
	"fmt"
	"os"
	"slices"
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

	redirectLabelStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	redirectDefaultStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	redirectActiveStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("111"))
	redirectFileStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("228"))
	redirectMissingStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("255")).Background(lipgloss.Color("196"))

	chainDimStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	chainAndStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("70"))
	chainPipeStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))

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
		Redirects:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "toggle redirects")),
		Help:        key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "toggle help")),
	}
)

const numCols = 2

type displayRowKind int

const (
	rowArg displayRowKind = iota
	rowEnv
	rowCommand
	rowRedirect
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
	Redirects   key.Binding
	Help        key.Binding
}

type displayRow struct {
	kind     displayRowKind
	name     string
	value    string
	disabled bool                  // excluded from exec and copy
	secret   bool                  // value passed to exec but shown as •••• in copy
	redirect domain.StreamRedirect // used only for rowRedirect rows
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

func hasArgRow(rows []displayRow) bool {
	for _, dr := range rows {
		if dr.kind == rowArg {
			return true
		}
	}
	return false
}

func buildRows(args []domain.Arg) []displayRow {
	rows := make([]displayRow, 0, max(1, len(args)))
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
	if len(rows) == 0 {
		rows = append(rows, displayRow{kind: rowArg})
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
	inv              domain.Invocation
	width            int
	rows             []displayRow   // current chain segment's rows
	chainIdx         int            // index of currently edited chain segment
	segRows          [][]displayRow // rows for all chain segments; segRows[chainIdx] may be stale
	row              int            // 0..len(rows)-1 = arg rows; len(rows) = Run button
	col              int
	mode             detailMode
	cArmed           bool
	dArmed           bool
	input            textinput.Model
	execRequested    bool
	undo             *undoEntry
	copied           bool
	saved            bool
	helpVisible      bool
	redirectsVisible bool
}

func (m detailModel) canExec() bool {
	if m.chainLen() > 1 {
		allRows := m.allSegmentRows()
		segs := m.inv.AllSegments()
		for i, rows := range allRows {
			fallback := ""
			if i < len(segs) {
				fallback = segs[i].Command
			}
			if rowsCommand(rows, fallback) == "" {
				return false
			}
		}
		return true
	}
	if m.CurrentCommand() == "" {
		return false
	}
	for _, dr := range m.rows {
		if dr.kind == rowRedirect && dr.redirect.Target == domain.RedirectFile && dr.redirect.File == "" {
			return false
		}
	}
	return true
}

func (m detailModel) ExecRequested() bool                   { return m.execRequested }
func (m detailModel) SaveRequested() bool                   { return m.saved }
func (m detailModel) InvocationID() string                  { return m.inv.ID }
func (m detailModel) OriginalInvocation() domain.Invocation { return m.inv }
func (m detailModel) CurrentCommand() string                { return rowsCommand(m.rows, m.inv.Command) }
func (m detailModel) CurrentEnv() []domain.EnvVar           { return rowsToEnv(m.rows) }
func (m detailModel) CurrentArgs() []domain.Arg             { return rowsToArgs(m.rows) }
func (m detailModel) CurrentStdout() domain.StreamRedirect  { return m.currentRedirect("stdout") }
func (m detailModel) CurrentStderr() domain.StreamRedirect  { return m.currentRedirect("stderr") }

// CurrentInvocation returns the full live state of all chain segments as a
// domain.Invocation, suitable for saving or recording.
func (m detailModel) CurrentInvocation() domain.Invocation {
	return m.buildLiveInvocation()
}

// chainLen returns the total number of chain segments (1 for a simple command).
func (m detailModel) chainLen() int { return len(m.segRows) }

func (m detailModel) currentRedirect(name string) domain.StreamRedirect {
	for _, dr := range m.rows {
		if dr.kind == rowRedirect && dr.name == name {
			return dr.redirect
		}
	}
	return domain.StreamRedirect{}
}
func (m detailModel) onButton() bool { return m.row == len(m.rows) }

func (m detailModel) onCommandRow() bool {
	return !m.onButton() && m.rows[m.row].kind == rowCommand
}

func (m *detailModel) normalizeCursor() {
	if m.onButton() {
		return
	}
	if m.rows[m.row].kind == rowCommand || m.rows[m.row].kind == rowRedirect {
		m.col = 1
	}
}

func newDetailModel(inv domain.Invocation, width int) detailModel {
	ti := textinput.New()
	ti.CharLimit = 256

	segs := inv.AllSegments()
	segRows := make([][]displayRow, len(segs))
	for i, seg := range segs {
		rows := append(append(buildEnvRows(seg.Env), buildCommandRow(seg.Command)), buildRows(seg.Args)...)
		rows = append(rows, buildRedirectRows(seg.Stdout, seg.Stderr)...)
		segRows[i] = rows
	}

	redirectsVisible := !inv.Stdout.IsZero() || !inv.Stderr.IsZero()
	if !redirectsVisible {
		for _, link := range inv.Chain {
			if !link.Stdout.IsZero() || !link.Stderr.IsZero() {
				redirectsVisible = true
				break
			}
		}
	}

	return detailModel{
		inv:              inv,
		width:            width,
		rows:             segRows[0],
		chainIdx:         0,
		segRows:          segRows,
		input:            ti,
		redirectsVisible: redirectsVisible,
	}
}

// switchToChain saves the current segment's rows and loads the target segment.
func (m *detailModel) switchToChain(idx int) {
	if idx < 0 || idx >= m.chainLen() {
		return
	}
	m.segRows[m.chainIdx] = m.rows
	m.chainIdx = idx
	m.rows = m.segRows[idx]
	m.undo = nil
}

// allSegmentRows returns a snapshot of all segments' rows, with the current
// segment's live rows (which may differ from segRows[chainIdx]).
func (m detailModel) allSegmentRows() [][]displayRow {
	result := make([][]displayRow, len(m.segRows))
	copy(result, m.segRows)
	result[m.chainIdx] = m.rows
	return result
}

// redirectFromRows looks up a redirect row by name ("stdout" or "stderr").
func redirectFromRows(rows []displayRow, name string) domain.StreamRedirect {
	for _, dr := range rows {
		if dr.kind == rowRedirect && dr.name == name {
			return dr.redirect
		}
	}
	return domain.StreamRedirect{}
}

// buildLiveInvocation assembles the current edits from all chain segments into
// a complete Invocation, preserving ID, Tags, and Runs from the original.
func (m detailModel) buildLiveInvocation() domain.Invocation {
	allRows := m.allSegmentRows()
	segs := m.inv.AllSegments()

	fallback0 := ""
	if len(segs) > 0 {
		fallback0 = segs[0].Command
	}
	inv := domain.Invocation{
		ID:      m.inv.ID,
		Command: rowsCommand(allRows[0], fallback0),
		Env:     rowsToEnv(allRows[0]),
		Args:    rowsToArgs(allRows[0]),
		Stdout:  redirectFromRows(allRows[0], "stdout"),
		Stderr:  redirectFromRows(allRows[0], "stderr"),
		Runs:    m.inv.Runs,
		Tags:    m.inv.Tags,
	}
	for i := 1; i < len(allRows) && i-1 < len(m.inv.Chain); i++ {
		origLink := m.inv.Chain[i-1]
		inv.Chain = append(inv.Chain, domain.ChainLink{
			Op:      origLink.Op,
			Command: rowsCommand(allRows[i], origLink.Command),
			Env:     rowsToEnv(allRows[i]),
			Args:    rowsToArgs(allRows[i]),
			Stdout:  redirectFromRows(allRows[i], "stdout"),
			Stderr:  redirectFromRows(allRows[i], "stderr"),
		})
	}
	return inv
}

// buildEmptySegmentRows returns a fresh set of display rows for a new chain segment.
func buildEmptySegmentRows() []displayRow {
	rows := buildEnvRows(nil)
	rows = append(rows, buildCommandRow(""))
	rows = append(rows, buildRows(nil)...)
	rows = append(rows, buildRedirectRows(domain.StreamRedirect{}, domain.StreamRedirect{})...)
	return rows
}

// liveCurrentCommand returns the command for the current chain segment,
// including any live (uncommitted) text in the input when the command row is active.
func (m detailModel) liveCurrentCommand() string {
	if m.mode == modeEditing && m.onCommandRow() {
		return m.input.Value()
	}
	segs := m.inv.AllSegments()
	fallback := ""
	if m.chainIdx < len(segs) {
		fallback = segs[m.chainIdx].Command
	}
	return rowsCommand(m.rows, fallback)
}

// isCurrentSegmentEmpty reports whether the committed command of the current
// chain segment is empty (used to decide whether to delete it on navigation).
func (m detailModel) isCurrentSegmentEmpty() bool {
	segs := m.inv.AllSegments()
	fallback := ""
	if m.chainIdx < len(segs) {
		fallback = segs[m.chainIdx].Command
	}
	return rowsCommand(m.rows, fallback) == ""
}

// createChainLink inserts a new empty chain segment immediately after the
// current one with the given operator and opens its command row for editing.
// When called from edit mode it first commits the active cell; when called
// from normal mode the commit step is skipped.
func (m detailModel) createChainLink(op domain.ChainOp) (detailModel, tea.Cmd) {
	if m.mode == modeEditing {
		// Commit the live cell value.
		m.undo = &undoEntry{row: m.row, col: m.col, value: m.currentCell()}
		m.setCell(m.input.Value())
		m.input.Blur()
		m.mode = modeNormal
	}

	// Save current segment's rows.
	m.segRows[m.chainIdx] = m.rows

	newRows := buildEmptySegmentRows()
	insertAt := m.chainIdx + 1

	// Insert into segRows.
	newSegRows := make([][]displayRow, 0, len(m.segRows)+1)
	newSegRows = append(newSegRows, m.segRows[:insertAt]...)
	newSegRows = append(newSegRows, newRows)
	newSegRows = append(newSegRows, m.segRows[insertAt:]...)
	m.segRows = newSegRows

	// Insert a new ChainLink at position chainIdx in inv.Chain.
	// Chain[i] holds the Op for segment i+1, so a link at chainIdx covers the
	// new segment at insertAt.
	newChain := make([]domain.ChainLink, 0, len(m.inv.Chain)+1)
	newChain = append(newChain, m.inv.Chain[:m.chainIdx]...)
	newChain = append(newChain, domain.ChainLink{Op: op})
	newChain = append(newChain, m.inv.Chain[m.chainIdx:]...)
	m.inv.Chain = newChain

	// Switch to the new segment and open its command row for editing.
	m.chainIdx = insertAt
	m.rows = m.segRows[insertAt]
	m.undo = nil

	for i, dr := range m.rows {
		if dr.kind == rowCommand {
			m.row = i
			m.col = 1
			return m.startEditing("")
		}
	}
	return m, nil
}

// deleteSegmentGoLeft removes the current chain segment and moves focus to the
// previous one. The caller must update m.row/m.col/normalizeCursor afterwards.
func (m *detailModel) deleteSegmentGoLeft() {
	idx := m.chainIdx
	// Remove the segment's rows.
	m.segRows = append(m.segRows[:idx:idx], m.segRows[idx+1:]...)
	// Remove the op entry: Chain[idx-1] for idx>0; Chain[0] for idx==0.
	opIdx := idx - 1
	if opIdx < 0 {
		opIdx = 0
	}
	if opIdx < len(m.inv.Chain) {
		m.inv.Chain = append(m.inv.Chain[:opIdx:opIdx], m.inv.Chain[opIdx+1:]...)
	}
	m.chainIdx = idx - 1
	m.rows = m.segRows[m.chainIdx]
	m.undo = nil
}

// deleteSegmentGoRight removes the current chain segment and moves focus to the
// next one (which slides into the current index). Caller updates row/col after.
func (m *detailModel) deleteSegmentGoRight() {
	idx := m.chainIdx
	m.segRows = append(m.segRows[:idx:idx], m.segRows[idx+1:]...)
	// Remove the op at max(0, idx-1): same formula as deleteSegmentGoLeft.
	opIdx := idx - 1
	if opIdx < 0 {
		opIdx = 0
	}
	if opIdx < len(m.inv.Chain) {
		m.inv.Chain = append(m.inv.Chain[:opIdx:opIdx], m.inv.Chain[opIdx+1:]...)
	}
	// chainIdx stays the same; it now points at the old idx+1 content.
	m.rows = m.segRows[m.chainIdx]
	m.undo = nil
}

// lastVisibleRowIdx returns the index of the last row that is visible given
// the current redirectsVisible setting.
func (m detailModel) lastVisibleRowIdx() int {
	for i := len(m.rows) - 1; i >= 0; i-- {
		if !m.redirectsVisible && m.rows[i].kind == rowRedirect {
			continue
		}
		return i
	}
	return 0
}

func buildRedirectRows(stdout, stderr domain.StreamRedirect) []displayRow {
	return []displayRow{
		{kind: rowRedirect, name: "stdout", redirect: stdout},
		{kind: rowRedirect, name: "stderr", redirect: stderr},
	}
}

func parseFileRedirectInput(s string) (file string, append bool) {
	if strings.HasPrefix(s, ">>") {
		return strings.TrimSpace(s[2:]), true
	}
	if strings.HasPrefix(s, ">") {
		return strings.TrimSpace(s[1:]), false
	}
	return strings.TrimSpace(s), false
}

// availableEnvNames returns env var names to suggest for $VAR completion.
// The invocation's own env rows come first; system env follows, sorted.
func (m detailModel) availableEnvNames() []string {
	seen := map[string]struct{}{}
	var names []string
	for _, dr := range m.rows {
		if dr.kind == rowEnv && dr.name != "" {
			if _, ok := seen[dr.name]; !ok {
				seen[dr.name] = struct{}{}
				names = append(names, dr.name)
			}
		}
	}
	sys := os.Environ()
	sysNames := make([]string, 0, len(sys))
	for _, kv := range sys {
		if k, _, ok := strings.Cut(kv, "="); ok && k != "" {
			sysNames = append(sysNames, k)
		}
	}
	slices.Sort(sysNames)
	for _, name := range sysNames {
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			names = append(names, name)
		}
	}
	return names
}

// isEnvIdentifier reports whether every rune in s is a valid env-var name
// character (letter, digit, or underscore).
func isEnvIdentifier(s string) bool {
	for _, r := range s {
		if r != '_' && !(r >= 'A' && r <= 'Z') && !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}

// envCompletion returns the suffix to complete the $VAR or ${VAR name the
// user is currently typing, or "" if no suggestion applies.
// For ${VAR the returned suffix includes the closing }.
func (m detailModel) envCompletion() string {
	val := m.input.Value()
	names := m.availableEnvNames()

	// ${VAR pattern — last ${ with no closing } yet.
	if braceIdx := strings.LastIndex(val, "${"); braceIdx >= 0 {
		prefix := val[braceIdx+2:]
		if len(prefix) > 0 && !strings.Contains(prefix, "}") && isEnvIdentifier(prefix) {
			for _, name := range names {
				if strings.HasPrefix(name, prefix) && len(name) > len(prefix) {
					return name[len(prefix):] + "}"
				}
			}
			return ""
		}
	}

	// $VAR pattern — last $ followed only by identifier characters.
	if idx := strings.LastIndex(val, "$"); idx >= 0 {
		prefix := val[idx+1:]
		if len(prefix) > 0 && isEnvIdentifier(prefix) {
			for _, name := range names {
				if strings.HasPrefix(name, prefix) && len(name) > len(prefix) {
					return name[len(prefix):]
				}
			}
		}
	}
	return ""
}

func (m detailModel) IsEditing() bool { return m.mode == modeEditing }

// WithCommandEditing positions the cursor on the command row and opens it for
// editing immediately — used when creating a new blank command.
func (m detailModel) WithCommandEditing() (detailModel, tea.Cmd) {
	for i, dr := range m.rows {
		if dr.kind == rowCommand {
			m.row = i
			m.col = 1
			return m.startEditing(dr.value)
		}
	}
	return m, nil
}

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
			if !hasArgRow(m.rows) {
				insertAt := 0
				for i, dr := range m.rows {
					if dr.kind == rowCommand {
						insertAt = i + 1
						break
					}
				}
				newRows := make([]displayRow, 0, len(m.rows)+1)
				newRows = append(newRows, m.rows[:insertAt]...)
				newRows = append(newRows, displayRow{kind: rowArg})
				newRows = append(newRows, m.rows[insertAt:]...)
				m.rows = newRows
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
			if !m.redirectsVisible {
				for m.row > 0 && m.rows[m.row].kind == rowRedirect {
					m.row--
				}
			}
		}
		m.normalizeCursor()
	case key.Matches(msg, detailKeys.Down):
		if m.row < len(m.rows) {
			m.row++
			if !m.redirectsVisible {
				for m.row < len(m.rows) && m.rows[m.row].kind == rowRedirect {
					m.row++
				}
			}
		}
		m.normalizeCursor()
	case key.Matches(msg, detailKeys.Left):
		switch {
		case m.onButton():
			// nothing
		case (m.onCommandRow() || m.col == 0) && m.chainIdx > 0:
			if m.isCurrentSegmentEmpty() {
				m.deleteSegmentGoLeft()
			} else {
				m.switchToChain(m.chainIdx - 1)
			}
			m.row = m.lastVisibleRowIdx()
			m.col = numCols - 1
			m.normalizeCursor()
		case !m.onCommandRow() && m.col > 0:
			m.col--
		}
	case key.Matches(msg, detailKeys.Right):
		switch {
		case m.onButton():
			// nothing
		case (m.onCommandRow() || m.col == numCols-1) && m.chainIdx < m.chainLen()-1:
			if m.isCurrentSegmentEmpty() {
				m.deleteSegmentGoRight()
			} else {
				m.switchToChain(m.chainIdx + 1)
			}
			m.row = 0
			m.col = 0
			m.normalizeCursor()
		case !m.onCommandRow() && m.col < numCols-1:
			m.col++
		}
	case key.Matches(msg, detailKeys.Enter):
		if m.onButton() {
			if m.canExec() {
				m.execRequested = true
				return m, tea.Quit
			}
			return m, nil
		}
		if m.rows[m.row].kind == rowRedirect {
			if m.rows[m.row].redirect.Target == domain.RedirectFile {
				return m.startEditing(m.currentCell())
			}
			m.rows[m.row].redirect = m.rows[m.row].redirect.Next()
			return m, nil
		}
		return m.startEditing(m.currentCell())
	case key.Matches(msg, detailKeys.Insert):
		if !m.onButton() {
			if m.rows[m.row].kind == rowRedirect {
				if m.rows[m.row].redirect.Target == domain.RedirectFile {
					return m.startEditing(m.currentCell())
				}
				m.rows[m.row].redirect = m.rows[m.row].redirect.Next()
				return m, nil
			}
			return m.startEditing(m.currentCell())
		}
	case key.Matches(msg, detailKeys.X):
		if m.canExec() {
			m.execRequested = true
			return m, tea.Quit
		}
		return m, nil
	case key.Matches(msg, detailKeys.C):
		if !m.onButton() {
			m.cArmed = true
		}
	case key.Matches(msg, detailKeys.ToggleRow):
		if !m.onButton() && m.rows[m.row].kind == rowRedirect {
			m.rows[m.row].redirect = m.rows[m.row].redirect.Next()
		} else if !m.onButton() && !m.onCommandRow() {
			m.rows[m.row].disabled = !m.rows[m.row].disabled
		}
	case key.Matches(msg, detailKeys.ToggleValue):
		if !m.onButton() && !m.onCommandRow() && m.rows[m.row].kind != rowRedirect {
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
		if !m.onButton() && !m.onCommandRow() && m.rows[m.row].kind != rowRedirect {
			m.dArmed = true
		}
	case key.Matches(msg, detailKeys.AddRow):
		if !m.onButton() && !m.onCommandRow() && m.rows[m.row].kind != rowRedirect {
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
	case key.Matches(msg, detailKeys.Redirects):
		m.redirectsVisible = !m.redirectsVisible
		if !m.redirectsVisible && !m.onButton() && m.rows[m.row].kind == rowRedirect {
			for m.row > 0 && m.rows[m.row].kind == rowRedirect {
				m.row--
			}
			m.normalizeCursor()
		}
	case key.Matches(msg, detailKeys.Help):
		m.helpVisible = !m.helpVisible
	case msg.String() == "&":
		if m.CurrentCommand() != "" {
			return m.createChainLink(domain.ChainAnd)
		}
	case msg.String() == "|":
		if m.CurrentCommand() != "" {
			return m.createChainLink(domain.ChainPipe)
		}
	case key.Matches(msg, detailKeys.Undo):
		if m.undo != nil {
			if m.undo.rows != nil {
				m.rows = m.undo.rows
				m.row = m.undo.rowPos
			} else {
				m.row, m.col = m.undo.row, m.undo.col
				m.setCell(m.undo.value)
			}
			m.normalizeCursor()
			m.undo = nil
		}
	}
	return m, nil
}

func (m detailModel) currentCell() string {
	if m.onButton() {
		return ""
	}
	if m.rows[m.row].kind == rowRedirect {
		r := m.rows[m.row].redirect
		if r.Append {
			return ">>" + r.File
		}
		return ">" + r.File
	}
	if m.onCommandRow() {
		return m.rows[m.row].value
	}
	if m.col == 0 {
		return m.rows[m.row].name
	}
	return m.rows[m.row].value
}

func (m *detailModel) setCell(s string) {
	if m.onButton() {
		return
	}
	if m.rows[m.row].kind == rowRedirect {
		file, app := parseFileRedirectInput(s)
		m.rows[m.row].redirect.File = file
		m.rows[m.row].redirect.Append = app
		return
	}
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
	case "tab":
		if completion := m.envCompletion(); completion != "" {
			m.input.SetValue(m.input.Value() + completion)
			m.input.CursorEnd()
		}
		return m, nil
	case "&", "|":
		// Create a new chain link if the current segment already has a command.
		if m.liveCurrentCommand() != "" {
			op := domain.ChainAnd
			if msg.String() == "|" {
				op = domain.ChainPipe
			}
			return m.createChainLink(op)
		}
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
	return m.buildLiveInvocation().RawCommandTokens()
}

func (m detailModel) copyCommand() string {
	if m.chainLen() > 1 {
		return m.buildLiveInvocation().FullCommand()
	}
	parts := append(envTokens(m.rows, false), append([]string{m.CurrentCommand()}, mergedTokens(m.rows, false)...)...)
	if s := m.CurrentStdout().StdoutShell(); s != "" {
		parts = append(parts, s)
	}
	if s := m.CurrentStderr().StderrShell(); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, " ")
}

func (m detailModel) liveCommand() string {
	rows := make([]displayRow, len(m.rows))
	copy(rows, m.rows)
	if m.mode == modeEditing && !m.onButton() {
		if rows[m.row].kind == rowRedirect {
			file, app := parseFileRedirectInput(m.input.Value())
			rows[m.row].redirect.File = file
			rows[m.row].redirect.Append = app
		} else if m.col == 0 {
			rows[m.row].name = m.input.Value()
		} else {
			rows[m.row].value = m.input.Value()
		}
	}
	parts := append(envTokens(rows, true), append([]string{rowsCommand(rows, m.inv.Command)}, mergedTokens(rows, true)...)...)
	for _, dr := range rows {
		if dr.kind != rowRedirect {
			continue
		}
		if dr.name == "stdout" {
			if s := dr.redirect.StdoutShell(); s != "" {
				parts = append(parts, s)
			}
		} else {
			if s := dr.redirect.StderrShell(); s != "" {
				parts = append(parts, s)
			}
		}
	}
	return strings.Join(parts, " ")
}

// staticSegmentCommand renders a chain segment's command string from rows
// (without live editing state).
func staticSegmentCommand(rows []displayRow, fallbackCmd string) string {
	parts := append(envTokens(rows, true), append([]string{rowsCommand(rows, fallbackCmd)}, mergedTokens(rows, true)...)...)
	for _, dr := range rows {
		if dr.kind != rowRedirect {
			continue
		}
		if dr.name == "stdout" {
			if s := dr.redirect.StdoutShell(); s != "" {
				parts = append(parts, s)
			}
		} else {
			if s := dr.redirect.StderrShell(); s != "" {
				parts = append(parts, s)
			}
		}
	}
	return strings.Join(parts, " ")
}

// liveChainPreview renders the full chain command with styling: the current
// segment is pink (headerStyle), others are dimmed, operators are colored.
func (m detailModel) liveChainPreview() string {
	allRows := m.allSegmentRows()
	segs := m.inv.AllSegments()
	var sb strings.Builder
	for i, rows := range allRows {
		fallbackCmd := ""
		if i < len(segs) {
			fallbackCmd = segs[i].Command
		}
		var cmdStr string
		if i == m.chainIdx {
			cmdStr = m.liveCommand()
		} else {
			cmdStr = staticSegmentCommand(rows, fallbackCmd)
		}
		if i > 0 && i < len(segs) {
			op := segs[i].Op
			switch op {
			case domain.ChainAnd:
				sb.WriteString(" " + chainAndStyle.Render("&&") + " ")
			case domain.ChainPipe:
				sb.WriteString(" " + chainPipeStyle.Render("|") + " ")
			default:
				sb.WriteString(" ")
			}
		}
		if i == m.chainIdx {
			sb.WriteString(headerStyle.Render(cmdStr))
		} else {
			sb.WriteString(chainDimStyle.Render(cmdStr))
		}
	}
	return sb.String()
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
	if m.chainLen() > 1 {
		b.WriteString("  " + m.liveChainPreview() + "\n")
	} else {
		maxW := m.width - 4
		for _, l := range wrapText(m.liveCommand(), maxW) {
			b.WriteString(headerStyle.Render("  "+l) + "\n")
		}
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
	if m.redirectsVisible {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			cellHeader.Render(padStr("stream", colNameW)),
			cellHeader.Render(padStr("redirect to", colValueW)),
		))
		b.WriteString("  " + sectionStyle.Render(strings.Repeat("─", colNameW+colValueW+4)) + "\n")
		for r, dr := range m.rows {
			if dr.kind != rowRedirect {
				continue
			}
			b.WriteString(m.renderRedirectRow(r, dr) + "\n")
		}
		b.WriteString("\n")
	}
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
		b.WriteString(hintStyle.Render("  y: copy cell  •  Y: copy cmd  •  p: paste  •  x: exec  •  r: redirects  •  esc: back  •  q: quit  •  ?: hide") + "\n")
	} else {
		b.WriteString(hintStyle.Render("  hjkl: navigate  •  i: edit  •  a: add row  •  dd: delete  •  m/M: move flag  •  S: save  •  x: exec  •  r: redirects  •  ?: more") + "\n")
	}

	return b.String()
}

func (m detailModel) renderRedirectRow(r int, dr displayRow) string {
	isActive := r == m.row && !m.onButton()
	isStdout := dr.name == "stdout"
	label := padStr(dr.name, colNameW)

	fileMissing := dr.redirect.Target == domain.RedirectFile && dr.redirect.File == ""

	var valText string
	if isActive && m.mode == modeEditing {
		valText = m.input.View()
	} else if dr.redirect.Target == domain.RedirectFile {
		prefix := ">"
		if dr.redirect.Append {
			prefix = ">>"
		}
		if fileMissing {
			valText = padStr(prefix+" <enter file name>", colValueW)
		} else {
			valText = padStr(prefix+dr.redirect.File, colValueW)
		}
	} else {
		valText = padStr(dr.redirect.CarouselLabel(isStdout), colValueW)
	}

	if isActive {
		nameCell := redirectLabelStyle.Render(label)
		var valCell string
		if fileMissing && m.mode != modeEditing {
			valCell = redirectMissingStyle.Render(valText)
		} else {
			valCell = cellSelected.Render(valText)
			if m.mode == modeEditing {
				if ghost := m.envCompletion(); ghost != "" {
					valCell += hintStyle.Render(ghost)
				}
			}
		}
		return rowActiveStyle.Render(fmt.Sprintf("  %s  %s", nameCell, valCell))
	}

	nameCell := redirectLabelStyle.Render(label)
	var valCell string
	switch dr.redirect.Target {
	case domain.RedirectDefault:
		valCell = redirectDefaultStyle.Render(valText)
	case domain.RedirectToOther:
		valCell = redirectActiveStyle.Render(valText)
	case domain.RedirectNull:
		valCell = cellDisabled.Render(valText)
	case domain.RedirectFile:
		if fileMissing {
			valCell = redirectMissingStyle.Render(valText)
		} else {
			valCell = redirectFileStyle.Render(valText)
		}
	}
	return fmt.Sprintf("  %s  %s", nameCell, valCell)
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
		if m.mode == modeEditing && m.col == 1 {
			if ghost := m.envCompletion(); ghost != "" {
				valCell += hintStyle.Render(ghost)
			}
		}
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
	cmdEmpty := dr.value == ""

	var text string
	if isActive && m.mode == modeEditing {
		text = m.input.View()
	} else if cmdEmpty {
		text = padStr("<enter command>", width)
	} else {
		text = padStr(dr.value, width)
	}

	var cell string
	switch {
	case isActive && m.mode == modeEditing:
		cell = headerStyle.Render(text)
		if ghost := m.envCompletion(); ghost != "" {
			cell += hintStyle.Render(ghost)
		}
	case cmdEmpty:
		cell = redirectMissingStyle.Render(text)
	case isActive:
		cell = cellSelected.Render(text)
	default:
		cell = headerStyle.Render(text)
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
