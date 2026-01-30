package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap defines all key bindings for the dashboard.
type KeyMap struct {
	Up         key.Binding
	Down       key.Binding
	Start      key.Binding
	Stop       key.Binding
	Restart    key.Binding
	Open       key.Binding
	StartAll   key.Binding
	StopAll    key.Binding
	ToggleProxy key.Binding
	ViewLogs   key.Binding
	Quit       key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Start: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "start"),
		),
		Stop: key.NewBinding(
			key.WithKeys("x"),
			key.WithHelp("x", "stop"),
		),
		Restart: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "restart"),
		),
		Open: key.NewBinding(
			key.WithKeys("o"),
			key.WithHelp("o", "open in browser"),
		),
		StartAll: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "start all"),
		),
		StopAll: key.NewBinding(
			key.WithKeys("X"),
			key.WithHelp("X", "stop all"),
		),
		ToggleProxy: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "toggle proxy"),
		),
		ViewLogs: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "view logs"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// ShortHelp returns a compact help string.
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Start, k.Stop, k.Restart, k.Open,
		k.StartAll, k.StopAll, k.ToggleProxy,
		k.ViewLogs, k.Quit,
	}
}

// FullHelp returns the full set of key bindings for the help view.
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Start, k.Stop, k.Restart, k.Open},
		{k.StartAll, k.StopAll, k.ToggleProxy},
		{k.ViewLogs, k.Quit},
	}
}
