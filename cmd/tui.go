package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"gopkg.in/yaml.v3"
	"k8s.io/api/core/v1"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const listHeight = 28

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2).Bold(true).Foreground(lipgloss.Color("170"))
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
type NamespaceUpdateMsg []string
type LogChunkMsg string

/*
Model Methods
*/

func (m *model) Init() tea.Cmd {
	return tea.Batch(
		m.listenForResourceUpdates(),
		m.listenForNamespaceUpdates())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	if m.entity.GetCurrentState() == spec || m.entity.GetCurrentState() == logs {
		var viewportCmd tea.Cmd
		m.entity.Data.viewport, viewportCmd = m.entity.Data.viewport.Update(msg)
		cmds = append(cmds, viewportCmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.entity.Data.list.SetSize(msg.Width, msg.Height-2)

	case LogChunkMsg:
		m.entity.Data.mu.Lock()
		m.entity.Data.logBuffer += string(msg)
		m.entity.Data.viewport.SetContent(m.entity.Data.logBuffer)
		m.entity.Data.viewport.GotoBottom()
		m.entity.Data.mu.Unlock()
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "l", "enter":
			if m.entity.Data.list.FilterState() != list.Filtering {
				cmd, transitioned := m.handleForward()
				if transitioned {
					m.syncList()
					if cmd != nil {
						cmds = append(cmds, cmd)
					}

					return m, tea.Batch(cmds...)
				}
			}

		case "h", "left":
			m.entity.Dispatch(transitionScreenBackward)
			m.syncList()

			return m, nil
		}

	case ResourceUpdateMsg:
		m.entity.Data.mu.Lock()
		m.entity.Data.unstructured = msg
		m.entity.Data.mu.Unlock()

		if m.entity.GetCurrentState() == resource {
			m.syncList()
		}
		cmds = append(cmds, m.listenForResourceUpdates())

	case NamespaceUpdateMsg:
		m.entity.Data.mu.Lock()
		m.entity.Data.namespaces = msg
		m.entity.Data.mu.Unlock()

		if m.entity.GetCurrentState() == namespace {
			m.syncList()
		}
		cmds = append(cmds, m.listenForNamespaceUpdates())

	}

	var listCmd tea.Cmd
	m.entity.Data.list, listCmd = m.entity.Data.list.Update(msg)
	cmds = append(cmds, listCmd)

	return m, tea.Batch(cmds...)
}

func (m *model) handleForward() (tea.Cmd, bool) {
	selected, ok := m.entity.Data.list.SelectedItem().(item)
	if !ok {
		return nil, false
	}

	var cmd tea.Cmd
	selStr := string(selected)
	state := m.entity.GetCurrentState()

	switch state {
	case gvr:
		m.entity.Data.mu.Lock()
		m.entity.Data.gvrChoice = selStr
		m.entity.Data.mu.Unlock()

		m.entity.Data.getGvrFromString()
		cmd = m.runInformer()

	case namespace:
		m.entity.Data.mu.Lock()
		m.entity.Data.nsChoice = selStr
		if selStr == "all" {
			m.entity.Data.nsChoice = ""
		}
		m.entity.Data.mu.Unlock()

	case resource:
		m.entity.Data.mu.RLock()
		unstructuredItems := m.entity.Data.unstructured
		m.entity.Data.mu.RUnlock()

		for _, obj := range unstructuredItems {
			if obj.GetName() == selStr {
				m.entity.Data.mu.Lock()
				m.entity.Data.selectedResource = obj
				m.entity.Data.viewport = viewport.New(m.entity.Data.list.Width(), m.entity.Data.list.Height()-4)
				m.entity.Data.mu.Unlock()
				break
			}
		}
		m.entity.Data.choice = ""

	case action:
		m.entity.Data.mu.Lock()
		m.entity.Data.choice = selStr
		m.entity.Data.mu.Unlock()
		if selStr == "spec" {
			m.syncSpec()
		}

	case container:
		m.entity.Data.mu.Lock()
		m.entity.Data.selectedContainer = selStr
		m.entity.Data.logBuffer = ""
		m.entity.Data.mu.Unlock()

		cmd = m.startLiveLogs()
	}

	m.entity.Data.list.ResetFilter()
	m.entity.Dispatch(transitionScreenForward)
	return cmd, true
}

func (m *model) View() string {
	state := m.entity.GetCurrentState()
	if state == spec || state == logs {
		m.entity.Data.mu.RLock()
		selectedResource := m.entity.Data.selectedResource
		viewportContainer := m.entity.Data.viewport
		m.entity.Data.mu.RUnlock()

		if selectedResource == nil {
			return "No resource selected"
		}

		title := "Viewing Spec"
		if state == logs {
			title = "Viewing Logs"
		}

		return fmt.Sprintf(
			"%s: %s (%3.f%%)\n\n%s\n\n%s",
			title, selectedResource.GetName(),
			viewportContainer.ScrollPercent()*100,
			viewportContainer.View(),
			helpStyle.Render("↑/↓: Scroll • h/←: Back"),
		)
	}
	return "\n" + m.entity.Data.list.View()
}

/*
Custom Methods
*/

func (m *model) listenForResourceUpdates() tea.Cmd {
	return func() tea.Msg {
		select {
		case resources, ok := <-m.entity.Data.resourceUpdates:
			if !ok {
				return nil // Channel closed
			}
			return ResourceUpdateMsg(resources)
		case <-m.entity.Data.shutdownChannels:
			return nil
		}
	}
}

func (m *model) listenForNamespaceUpdates() tea.Cmd {
	return func() tea.Msg {
		select {
		case namespaces, ok := <-m.entity.Data.namespaceUpdates:
			if !ok {
				return nil // Channel closed
			}
			return NamespaceUpdateMsg(namespaces)
		case <-m.entity.Data.shutdownChannels:
			return nil
		}
	}
}

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
		m.entity.Data.mu.RLock()
		selectedGvr := m.entity.Data.selectedGvr
		namespaces := m.entity.Data.namespaces
		m.entity.Data.mu.RUnlock()

		if selectedGvr != nil {
			title = fmt.Sprintf("Namespaces (%s)", selectedGvr.Name)
		}
		for _, ns := range namespaces {
			items = append(items, item(ns))
		}

	case resource:
		m.entity.Data.mu.RLock()
		selectedGvr := m.entity.Data.selectedGvr
		unstructuredItems := m.entity.Data.unstructured
		ns := m.entity.Data.nsChoice
		m.entity.Data.mu.RUnlock()

		if selectedGvr != nil {
			title = fmt.Sprintf("Resources (%s)", selectedGvr.Name)
		}

		var names []string
		for _, unstr := range unstructuredItems {
			if ns == "" || unstr.GetNamespace() == ns {
				names = append(names, unstr.GetName())
			}
		}

		slices.Sort(names)
		for _, name := range names {
			items = append(items, item(name))
		}

	case action:
		m.entity.Data.mu.RLock()
		selectedGvr := m.entity.Data.selectedGvr
		m.entity.Data.mu.RUnlock()

		if selectedGvr != nil {
			title = fmt.Sprintf("Actions for %s", selectedGvr.Name)
			for _, action := range selectedGvr.SubResources {
				items = append(items, item(action))
			}
		}
	case container:
		m.entity.Data.mu.RLock()
		selectedResource := m.entity.Data.selectedResource
		m.entity.Data.mu.RUnlock()

		title = "Select Container"
		if selectedResource != nil {
			containers, _, _ := unstructured.NestedSlice(selectedResource.Object, "spec", "containers")
			initContainers, _, _ := unstructured.NestedSlice(selectedResource.Object, "spec", "initContainers")

			for _, c := range append(containers, initContainers...) {
				if cMap, ok := c.(map[string]interface{}); ok {
					if name, ok := cMap["name"].(string); ok {
						items = append(items, item(name))
					}
				}
			}
		}
	}

	m.entity.Data.list.Title = title
	m.entity.Data.list.SetItems(items)

	m.entity.Data.list.ResetFilter()
	m.entity.Data.list.Select(0)
	m.entity.Data.list.Paginator.Page = 0
}

func (m *model) syncSpec() {
	m.entity.Data.mu.RLock()
	selectedResource := m.entity.Data.selectedResource
	unstructuredObject := m.entity.Data.unstructured
	m.entity.Data.mu.RUnlock()

	if selectedResource == nil {
		return
	}

	for _, obj := range unstructuredObject {
		if obj.GetName() == selectedResource.GetName() &&
			obj.GetNamespace() == selectedResource.GetNamespace() {

			m.entity.Data.mu.Lock()
			m.entity.Data.selectedResource = obj
			m.entity.Data.mu.Unlock()

			selectedResource = obj
			break
		}
	}

	yamlData, err := yaml.Marshal(selectedResource.Object)
	if err != nil {
		m.entity.Data.mu.Lock()
		m.entity.Data.viewport.SetContent("Error marshaling spec: " + err.Error())
		m.entity.Data.mu.Unlock()
		return
	}

	m.entity.Data.mu.Lock()
	m.entity.Data.viewport.SetContent(string(yamlData))
	m.entity.Data.mu.Unlock()
}

func (m *model) syncLogs() {
	m.entity.Data.mu.RLock()
	pod := m.entity.Data.selectedResource
	container := m.entity.Data.selectedContainer
	clientset := m.entity.Data.clients.Typed
	m.entity.Data.mu.RUnlock()

	if pod == nil || clientset == nil {
		log.Printf("Either pod or clientset is nil, cannot continue.")
		return
	}

	tailLines := int64(200)
	req := clientset.CoreV1().Pods(pod.GetNamespace()).GetLogs(pod.GetName(), &v1.PodLogOptions{
		Container: container,
		TailLines: &tailLines,
	})

	logStream, err := req.Stream(context.Background())
	if err != nil {
		m.entity.Data.mu.Lock()
		m.entity.Data.viewport.SetContent("Error fetching logs: " + err.Error())
		m.entity.Data.mu.Unlock()
		return
	}
	defer logStream.Close()

	logBytes, err := io.ReadAll(logStream)
	if err != nil {
		m.entity.Data.mu.Lock()
		m.entity.Data.viewport.SetContent("Error fetching logs: " + err.Error())
		m.entity.Data.mu.Unlock()
		return
	}

	m.entity.Data.mu.Lock()
	m.entity.Data.viewport.SetContent(string(logBytes))
	m.entity.Data.mu.Unlock()
}

func (m *model) startLiveLogs() tea.Cmd {
	return func() tea.Msg {
		m.entity.Data.mu.Lock()
		pod := m.entity.Data.selectedResource
		container := m.entity.Data.selectedContainer
		clientset := m.entity.Data.clients.Typed

		ctx, cancel := context.WithCancel(context.Background())
		m.entity.Data.cancelLog = cancel
		m.entity.Data.mu.Unlock()

		if pod == nil || clientset == nil {
			return nil
		}

		tailLines := int64(200)
		req := clientset.CoreV1().Pods(pod.GetNamespace()).GetLogs(pod.GetName(), &v1.PodLogOptions{
			Container: container,
			TailLines: &tailLines,
			Follow:    true,
		})

		stream, err := req.Stream(ctx)
		if err != nil {
			return nil
		}

		go func() {
			defer stream.Close()
			scanner := bufio.NewScanner(stream)
			for scanner.Scan() {
				m.entity.Data.program.Send(LogChunkMsg(scanner.Text() + "\n"))

				select {
				case <-ctx.Done():
					return
				default:
				}
			}
		}()
		return nil
	}
}
