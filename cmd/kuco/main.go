package main

import (
	"fmt"
	"github.com/alexei-ozerov/kube-traverse/internal/fsm"
	"github.com/alexei-ozerov/kube-traverse/internal/kube"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"log"
	"os"
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

type appCtx struct {
	// State
	state fsm.Entity

	// Kube
	clients     kube.Ctx
	gvrList     []kube.ApiResource
	selectedGvr kube.ApiResource

	// Ui
	title string
}

func (c *appCtx) fetchKubeData() error {
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

	c.clients.Discovery = discoClient
	c.clients.Dynamic = dynClient

	return nil
}

/*
State Transitions
*/

func (c *appCtx) gvrGetData() (fsm.State, bool) {
	return 0, false
}

func (c *appCtx) gvrTransitionScreen() (fsm.State, bool) {
	if c.selectedGvr.Namespaced {
		return namespace, true
	}

	return resource, true
}

/*
Runtime
*/

func main() {
	ctx := appCtx{}
	pCtx := &ctx

	err := pCtx.fetchKubeData()
	if err != nil {
		log.Fatal(err)
	}

	pCtx.state = fsm.Entity{}
	pCtx.state.SetInitialState(gvr)
	pCtx.state.SetMachine([][]fsm.StateFn{
		{pCtx.gvrGetData, pCtx.gvrTransitionScreen},
		{},
	})

	gvrList, err := pCtx.clients.Discovery.GetListableResources()
	if err != nil {
		log.Fatal(err)
	}
	ctx.gvrList = gvrList

	m := &model{
		ctx: pCtx,
	}

	// Initialize GVR List (For demo purposes)
	// TODO (ozerova): Remove once ready to implement functionality properly
	var items []list.Item
	for _, gvr := range m.ctx.gvrList {
		items = append(items, item(gvr.Name))
	}
	m.list = initializeGvrList(items, m)

	// Init Program & Run TUI
	m.program = tea.NewProgram(m)
	if _, err := m.program.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
