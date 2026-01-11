package main

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/alexei-ozerov/kube-traverse/internal/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/tools/cache"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

type model struct {
	ctx *applicationContext

	ready bool

	// Display
	list   list.Model
	choice string

	namespaces []string
	ns         string

	// Lifecycle
	cancelInformer context.CancelFunc
	program        *tea.Program
}

/*
Messages
*/

type ResourceUpdateMsg []list.Item
type NamepaceUpdateMsg []string

/*
Model Methods
*/

func (m *model) Init() tea.Cmd {
	return nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ResourceUpdateMsg:
		if m.ctx.state.GetCurrentState() == namespace {
			items := make([]list.Item, len(m.namespaces))
			for i, ns := range m.namespaces {
				items[i] = item(ns)
			}
			return m, m.list.SetItems(items)
		}

		return m, m.list.SetItems(msg)

	case NamepaceUpdateMsg:
		m.namespaces = msg
		if m.ctx.state.GetCurrentState() == namespace {
			items := make([]list.Item, len(m.namespaces))
			for i, ns := range m.namespaces {
				items[i] = item(ns)
			}
			return m, m.list.SetItems(items)
		}

		return m, nil

	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil

	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "enter":
			selected, ok := m.list.SelectedItem().(item)
			if ok {
				// This transition only happens when a namespaced resource is selected
				// therefore, selectedGvr will always be set prior to this branch running
				if m.ctx.state.GetCurrentState() == namespace {
					m.ns = string(selected)

					m.list.FilterInput.Reset()
					m.list.SetFilterState(0)

					m.ctx.state.Dispatch(transitionScreen)
					m.startInformer(m.ctx.kube.selectedGvr)
				} else {
					m.choice = string(selected)

					var selectedGvr kube.ApiResource
					for _, g := range m.ctx.kube.gvrList {
						if g.Name == m.choice {
							selectedGvr = g
							break
						}
					}

					m.list.FilterInput.Reset()
					m.list.SetFilterState(0)

					// Transition the FSM
					m.ctx.kube.selectedGvr = selectedGvr
					m.startInformer(selectedGvr)
					m.ctx.state.Dispatch(transitionScreen)
				}

				m.setListTitle()
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *model) View() string {
	switch m.ctx.state.GetCurrentState() {
	case gvr:
		return "\n" + m.list.View()
	case namespace:
		return "\n" + m.list.View()
	case resource:
		return "\n" + m.list.View()
	case children:
		return "\nPlaceholder: children state"
	case reference:
		return "\nPlaceholder: reference state"
	default:
		panic("unhandled default case")
	}
}

/*
Custom Methods
*/

func (m *model) startInformer(gvr kube.ApiResource) {
	if m.cancelInformer != nil {
		m.cancelInformer()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancelInformer = cancel

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		m.ctx.kube.clients.Dynamic.Client,
		0,
		m.ns,
		nil,
	)
	genericInformer := factory.ForResource(gvr.GVR)
	informer := genericInformer.Informer()

	syncToTUI := func() {
		if m.program == nil {
			return
		}

		objs := informer.GetStore().List()
		names := make([]string, 0, len(objs))
		for _, obj := range objs {
			if unstructuredResource, ok := obj.(*unstructured.Unstructured); ok {
				names = append(names, unstructuredResource.GetName())
			}
		}
		slices.Sort(names)

		items := make([]list.Item, len(names))
		for i, name := range names {
			items[i] = item(name)
		}

		m.program.Send(ResourceUpdateMsg(items))
	}

	// Callbacks for watcher (all of which sync the Items list from scratch) :3
	// TODO (ozerova): Consider a more efficient way of doing this please..
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { syncToTUI() },
		UpdateFunc: func(old, new interface{}) { syncToTUI() },
		DeleteFunc: func(obj interface{}) { syncToTUI() },
	})
	if err != nil {
		return
	}

	go informer.Run(ctx.Done())
}

func (m *model) initNamespaceWatcher(ctx context.Context) {
	nsGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	factory := dynamicinformer.NewDynamicSharedInformerFactory(m.ctx.kube.clients.Dynamic.Client, time.Minute*30)
	informer := factory.ForResource(nsGVR).Informer()

	syncNamespaces := func() {
		if m.program == nil {
			return
		}

		objs := informer.GetStore().List()
		nsNames := make([]string, 0, len(objs))
		for _, obj := range objs {
			if unstr, ok := obj.(*unstructured.Unstructured); ok {
				nsNames = append(nsNames, unstr.GetName())
			}
		}

		slices.Sort(nsNames)

		m.program.Send(NamepaceUpdateMsg(nsNames))
	}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { syncNamespaces() },
		DeleteFunc: func(obj interface{}) { syncNamespaces() },
	})

	go informer.Run(ctx.Done())
}

func (m *model) setListTitle() {
	switch m.ctx.state.GetCurrentState() {
	case gvr:
		m.list.Title = "GVRs"
	case namespace:
		m.list.Title = "Namespaces"
	case resource:
		m.list.Title = strings.ToUpper(m.choice)
	case children:
		m.list.Title = "Children"
	case reference:
		m.list.Title = "References"
	default:
		panic("unhandled default case")
	}
}
