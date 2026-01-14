package main

import (
	"context"
	"fmt"
	"github.com/alexei-ozerov/kube-traverse/internal/fsm"
	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"log"
	"os"
)

// State will track the possible states that the UI is capable of showing
const (
	gvr fsm.State = iota
	namespace
	resource
	spec
)

// Events will track different actions which can impact the state.
const (
	transitionScreenForward fsm.Event = iota
	transitionScreenBackward
)

type model struct {
	entity *fsm.Entity[appData]
}

/*
Runtime
*/

func main() {
	logFile, err := setupLogging()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup logging: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	// Data
	d := newAppData()
	err = d.fetchKubeData()
	if err != nil {
		log.Fatal(err)
	}
	d.dynFact = dynamicinformer.NewDynamicSharedInformerFactory(d.clients.Dynamic.Client, 0)

	// Initialize FSM
	e := &fsm.Entity[appData]{
		Data: d,
	}

	// Initialize Model
	m := &model{entity: e}

	// Setup initial state
	e.SetInitialState(gvr)
	e.SetMachine([][]fsm.StateFn{
		{m.gvrTransitionScreenForward, m.gvrTransitionScreenBackward},
		{m.namespaceTransitionScreenForward, m.namespaceTransitionScreenBackward},
		{m.resourceTransitionScreenForward, m.resourceTransitionScreenBackward},
		{m.specTransitionScreenForward, m.specTransitionScreenBackward},
	})

	// Okay, this is probably pedantic...
	e.Data.program = tea.NewProgram(m, tea.WithAltScreen())

	// Initialize GVR List
	gvrList, err := d.clients.Discovery.GetListableResources()
	if err != nil {
		log.Fatal(err)
	}
	d.gvrList = gvrList
	d.convertGvrToItemList()

	// Start watching namespaces in goroutine
	globalCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d.initNamespaceWatcher(globalCtx)

	if _, err := e.Data.program.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}

	// Cleanup channels
	d.shutdown()
}
