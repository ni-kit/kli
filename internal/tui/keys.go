package tui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
)

type appKeyMap struct {
	Enter key.Binding
	Back  key.Binding
	Quit  key.Binding
}

type histKeyMap struct {
	New       key.Binding
	Tag       key.Binding
	ToggleAdd key.Binding
	Delete    key.Binding
}

var (
	appKeys = appKeyMap{
		Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Back:  key.NewBinding(key.WithKeys("esc", "backspace"), key.WithHelp("esc", "back")),
		Quit:  key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	}

	histKeys = histKeyMap{
		New:       key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "new command")),
		Tag:       key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "edit tags")),
		ToggleAdd: key.NewBinding(key.WithKeys("T"), key.WithHelp("T", "add toggle")),
		Delete:    key.NewBinding(key.WithKeys("d"), key.WithHelp("dd", "delete")),
	}
)

func historyKeyMap() list.KeyMap {
	km := list.DefaultKeyMap()
	km.Quit.SetEnabled(false)
	// remove d from NextPage so we can use it for dd-delete
	km.NextPage.SetKeys("right", "l", "pgdown", "f")
	return km
}
