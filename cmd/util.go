package main

import (
	"fmt"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"log"
	"os"
	"time"
)

func setupLogging() (*os.File, error) {
	if err := os.MkdirAll("logs", 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	logFileName := fmt.Sprintf("logs/kube-traverse-%s.log", time.Now().Format("2006-01-02_15-04-05"))
	logFile, err := os.OpenFile(logFileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	log.SetOutput(logFile)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	return logFile, nil
}

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

// TODO (ozerova): decide on if this is idiomatic or not.
func (m *model) lockResource() {
	m.entity.Data.mu.Lock()
}
func (m *model) unlockResource() {
	m.entity.Data.mu.Unlock()
}
