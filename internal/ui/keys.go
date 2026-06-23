package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Kill    key.Binding
	Confirm key.Binding
	Cancel  key.Binding
	Refresh key.Binding
	Quit    key.Binding
}

var keys = keyMap{
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Kill:    key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "send SIGTERM")),
	Confirm: key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "confirm")),
	Cancel:  key.NewBinding(key.WithKeys("n", "esc"), key.WithHelp("n/esc", "cancel")),
	Refresh: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

func (k keyMap) help() string {
	return "  ↑/k up  ↓/j down  x SIGTERM  r refresh  q quit"
}
