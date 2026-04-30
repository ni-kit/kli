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

var appKeys = appKeyMap{
	Enter: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
	Back:  key.NewBinding(key.WithKeys("esc", "backspace"), key.WithHelp("esc", "back")),
	Quit:  key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

type histKeyMap struct {
	Tag key.Binding
}

var histKeys = histKeyMap{
	Tag: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "edit tags")),
}

func historyKeyMap() list.KeyMap {
	km := list.DefaultKeyMap()
	km.Quit.SetEnabled(false)
	return km
}
