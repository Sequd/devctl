package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Tab     key.Binding
	Enter   key.Binding
	DocUp   key.Binding // docker up
	DocDown key.Binding // docker down
	Restart key.Binding
	Rebuild key.Binding
	Refresh key.Binding
	Logs    key.Binding
	Command key.Binding
	Create  key.Binding
	Edit    key.Binding
	Quit    key.Binding
}

var keys = keyMap{
	Up:      key.NewBinding(key.WithKeys("up", "k")),
	Down:    key.NewBinding(key.WithKeys("down", "j")),
	Tab:     key.NewBinding(key.WithKeys("tab")),
	Enter:   key.NewBinding(key.WithKeys("enter")),
	DocUp:   key.NewBinding(key.WithKeys("u")),
	DocDown: key.NewBinding(key.WithKeys("d")),
	Restart: key.NewBinding(key.WithKeys("r")),
	Rebuild: key.NewBinding(key.WithKeys("b")),
	Refresh: key.NewBinding(key.WithKeys("R")),
	Logs:    key.NewBinding(key.WithKeys("l")),
	Command: key.NewBinding(key.WithKeys(":")),
	Create:  key.NewBinding(key.WithKeys("c")),
	Edit:    key.NewBinding(key.WithKeys("e")),
	Quit:    key.NewBinding(key.WithKeys("q", "ctrl+c")),
}
