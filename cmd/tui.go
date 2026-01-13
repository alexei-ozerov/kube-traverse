package main

import (
	"fmt"
	"github.com/charmbracelet/bubbles/viewport"
	"gopkg.in/yaml.v3"
	"io"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const listHeight = 28

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
)

type item string

func (i item) FilterValue() string {
	return fmt.Sprintf("%s", i)
}

type itemDelegate struct{}

func (d itemDelegate) Height() int                             { return 1 }
func (d itemDelegate) Spacing() int                            { return 0 }
func (d itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (d itemDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(item)
	if !ok {
		return
	}

	str := fmt.Sprintf("%s", i)

	fn := itemStyle.Render
	if index == m.Index() {
		fn = func(s ...string) string {
			return selectedItemStyle.Render("  " + strings.Join(s, " "))
		}
	}

	_, err := fmt.Fprint(w, fn(str))
	if err != nil {
		return
	}
}

/*
Messages
*/

type ResourceUpdateMsg []*unstructured.Unstructured

/*
Model Methods
*/

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.entity.GetCurrentState() == spec {
		var viewportCmd tea.Cmd
		m.entity.Data.viewport, viewportCmd = m.entity.Data.viewport.Update(msg)
		cmds = append(cmds, viewportCmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.entity.Data.list.SetSize(msg.Width, msg.Height-2)

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "l", "enter":
			if m.handleForward() {
				m.syncList()
			}

		case "h", "left":
			m.entity.Dispatch(transitionScreenBackward)
			m.syncList()
		}

	case ResourceUpdateMsg:
		m.entity.Data.unstructured = msg
		m.syncList()
	}

	var listCmd tea.Cmd
	m.entity.Data.list, listCmd = m.entity.Data.list.Update(msg)
	cmds = append(cmds, listCmd)

	return m, tea.Batch(cmds...)
}

func (m *model) handleForward() bool {
	selected, ok := m.entity.Data.list.SelectedItem().(item)
	if !ok {
		return false
	}

	selStr := string(selected)
	state := m.entity.GetCurrentState()

	switch state {
	case gvr:
		m.entity.Data.gvrChoice = selStr
		m.entity.Data.getGvrFromString()
		m.runInformer()
	case namespace:
		m.entity.Data.ns = selStr
	case resource:
		for _, obj := range m.entity.Data.unstructured {
			if obj.GetName() == selStr {
				m.entity.Data.selectedResource = obj
				break
			}
		}
		m.entity.Data.viewport = viewport.New(m.entity.Data.list.Width(), m.entity.Data.list.Height()-4)
		m.syncSpec()
	}

	m.entity.Data.list.FilterInput.Reset()
	m.entity.Data.list.SetFilterState(0)
	m.entity.Dispatch(transitionScreenForward)

	return true
}

func (m *model) View() string {
	if m.entity.GetCurrentState() == spec {
		scrollPercent := fmt.Sprintf("%3.f%%", m.entity.Data.viewport.ScrollPercent()*100)

		return fmt.Sprintf(
			"Viewing Spec: %s (%s)\n\n%s\n\n%s",
			m.entity.Data.selectedResource.GetName(),
			scrollPercent,
			m.entity.Data.viewport.View(),
			helpStyle.Render("↑/↓: Scroll • h/←: Back"),
		)
	}
	return "\n" + m.entity.Data.list.View()
}

/*
Custom Methods
*/

func (m *model) syncList() {
	state := m.entity.GetCurrentState()
	var items []list.Item
	var title string

	switch state {
	case gvr:
		title = "Resources (GVRs)"
		for _, g := range m.entity.Data.gvrList {
			items = append(items, item(g.Name))
		}

	case namespace:
		title = fmt.Sprintf("Namespaces (%s)", m.entity.Data.selectedGvr.Name)
		for _, ns := range m.entity.Data.namespaces {
			items = append(items, item(ns))
		}

	case resource:
		title = fmt.Sprintf("Resources (%s)", m.entity.Data.selectedGvr.Name)
		if m.entity.Data.selectedGvr.Namespaced {
			title += " in " + m.entity.Data.ns
		}

		var names []string
		for _, unstr := range m.entity.Data.unstructured {
			// If namespaced, only show items in selected namespace
			if !m.entity.Data.selectedGvr.Namespaced || unstr.GetNamespace() == m.entity.Data.ns {
				names = append(names, unstr.GetName())
			}
		}
		slices.Sort(names)
		for _, name := range names {
			items = append(items, item(name))
		}
	}

	m.entity.Data.list.Title = title
	m.entity.Data.list.SetItems(items)
}

func (m *model) syncSpec() {
	if m.entity.Data.selectedResource == nil {
		return
	}

	for _, obj := range m.entity.Data.unstructured {
		if obj.GetName() == m.entity.Data.selectedResource.GetName() &&
			obj.GetNamespace() == m.entity.Data.selectedResource.GetNamespace() {
			m.entity.Data.selectedResource = obj
			break
		}
	}

	yamlData, err := yaml.Marshal(m.entity.Data.selectedResource.Object)
	if err != nil {
		m.entity.Data.viewport.SetContent("Error marshaling spec: " + err.Error())
		return
	}

	m.entity.Data.viewport.SetContent(string(yamlData))
}
