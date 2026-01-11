package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/alexei-ozerov/kube-traverse/internal/fsm"
	"github.com/alexei-ozerov/kube-traverse/internal/kube"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

/*
State Machine & Application Context
*/

// State will track the possible states that the UI is capable of showing
const (
	gvr fsm.State = iota
	namespace
	children
	reference
	resource
)

// Events will track different actions which can impact the state.
const (
	getData fsm.Event = iota
	transitionScreen
)

type kubeData struct {
	clients     kube.Ctx
	gvrList     []kube.ApiResource
	selectedGvr kube.ApiResource
}

type applicationContext struct {
	state fsm.Entity
	kube  kubeData
}

func (c *applicationContext) fetchKubeData() error {
	kubeCfg, err := kube.InitializeK8sClientConfig()
	if err != nil {
		return fmt.Errorf("error initializing k8s client: %v", err)
	}

	discoClient, err := kube.NewDiscoveryClient(kubeCfg)
	if err != nil {
		return fmt.Errorf("error initializing discovery client: %v", err)
	}

	dynClient, err := kube.GetDynamicClient(kubeCfg)
	if err != nil {
		return fmt.Errorf("error initializing dynamic client: %v", err)
	}

	c.kube.clients.Discovery = discoClient
	c.kube.clients.Dynamic = dynClient

	return nil
}

/*
State Transitions
*/

func (c *applicationContext) gvrGetData() (fsm.State, bool) {
	return 0, false
}

func (c *applicationContext) gvrTransitionScreen() (fsm.State, bool) {
	if c.kube.selectedGvr.Namespaced {
		return namespace, true
	}

	return resource, true
}

func (c *applicationContext) namespaceGetData() (fsm.State, bool) {
	return namespace, false
}

func (c *applicationContext) namespaceTransitionScreen() (fsm.State, bool) {
	return resource, true
}

/*
Runtime
*/

func main() {
	ctx := applicationContext{}
	pCtx := &ctx

	err := pCtx.fetchKubeData()
	if err != nil {
		log.Fatal(err)
	}

	pCtx.state = fsm.Entity{}
	pCtx.state.SetInitialState(gvr)
	pCtx.state.SetMachine([][]fsm.StateFn{
		{pCtx.gvrGetData, pCtx.gvrTransitionScreen},
		{pCtx.namespaceGetData, pCtx.namespaceTransitionScreen},
	})

	gvrList, err := pCtx.kube.clients.Discovery.GetListableResources()
	if err != nil {
		log.Fatal(err)
	}
	ctx.kube.gvrList = gvrList

	m := &model{
		ctx: pCtx,
	}

	// Initialize GVR List (For demo purposes)
	// TODO (ozerova): Remove once ready to implement functionality properly
	var items []list.Item
	for _, gvr := range m.ctx.kube.gvrList {
		items = append(items, item(gvr.Name))
	}
	m.list = initializeGvrList(items)

	// Init Program & Run TUI
	m.program = tea.NewProgram(m)

	globalCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.initNamespaceWatcher(globalCtx)

	if _, err := m.program.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
