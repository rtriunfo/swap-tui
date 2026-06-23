package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up        key.Binding
	Down      key.Binding
	Kill      key.Binding // x — SIGTERM
	ForceKill key.Binding // X — SIGKILL
	Confirm   key.Binding
	Cancel    key.Binding
	Refresh   key.Binding
	Sort      key.Binding // s — toggle sort mode
	Filter    key.Binding // / — enter filter mode
	Quit      key.Binding
}

var keys = keyMap{
	Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Kill:      key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "SIGTERM")),
	ForceKill: key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "SIGKILL")),
	Confirm:   key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "confirm")),
	Cancel:    key.NewBinding(key.WithKeys("n", "esc"), key.WithHelp("n/esc", "cancel")),
	Refresh:   key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	Sort:      key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "toggle sort")),
	Filter:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
}

func (k keyMap) help() string {
	return "  ↑/k up  ↓/j down  x SIGTERM  X SIGKILL  s sort  / filter  r refresh  q quit"
}
