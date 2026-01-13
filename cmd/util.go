package main

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
)

func initializeGvrList(items []list.Item) list.Model {
	const defaultWidth = 14
	l := list.New(items, itemDelegate{}, defaultWidth, listHeight)
	customKeys := newListKeyMap()
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			customKeys.selectItem,
			customKeys.back,
		}
	}

	l.Title = "GVRs"
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(true)
	l.Styles.Title = titleStyle
	l.Styles.PaginationStyle = paginationStyle
	l.Styles.HelpStyle = helpStyle
	l.Styles.NoItems = lipgloss.NewStyle().PaddingLeft(4)
	l.Styles.FilterPrompt = lipgloss.NewStyle().PaddingLeft(4)
	l.Styles.FilterCursor = lipgloss.NewStyle().Foreground(lipgloss.Color("170"))

	return l
}

type listKeyMap struct {
	selectItem key.Binding
	back       key.Binding
}

// NewListKeyMap initializes the custom keys for the UI
func newListKeyMap() listKeyMap {
	return listKeyMap{
		selectItem: key.NewBinding(
			key.WithKeys("enter", "l"),
			key.WithHelp("enter/l", "select"),
		),
		back: key.NewBinding(
			key.WithKeys("h", "left"),
			key.WithHelp("esc/h/←", "back"),
		),
	}
}
