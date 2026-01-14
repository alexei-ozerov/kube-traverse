package main

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/alexei-ozerov/kube-traverse/internal/kube"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type appData struct {
	// Lifecycle
	mu             sync.RWMutex
	cancelInformer context.CancelFunc
	cancelLog      context.CancelFunc
	informerWg     sync.WaitGroup
	program        *tea.Program

	// Channels
	resourceUpdates  chan []*unstructured.Unstructured
	namespaceUpdates chan []string
	shutdownChannels chan struct{}

	// Kube
	clients     kube.Ctx
	gvrList     []kube.ApiResource
	selectedGvr *kube.ApiResource
	dynFact     dynamicinformer.DynamicSharedInformerFactory

	// Tui
	list              list.Model
	choice            string
	gvrChoice         string
	nsChoice          string
	namespaces        []string
	resources         []list.Item
	unstructured      []*unstructured.Unstructured
	viewport          viewport.Model
	selectedResource  *unstructured.Unstructured
	selectedContainer string
	logBuffer         string
}

func newAppData() *appData {
	return &appData{
		resourceUpdates:  make(chan []*unstructured.Unstructured, 10),
		namespaceUpdates: make(chan []string, 10),
		shutdownChannels: make(chan struct{}),
		namespaces:       []string{"all"},
	}
}

func (a *appData) getGvrFromString() {
	a.mu.RLock()
	gvrChoice := a.gvrChoice
	a.mu.RUnlock()

	for i := range a.gvrList {
		if a.gvrList[i].Name == gvrChoice {
			a.mu.Lock()
			a.selectedGvr = &a.gvrList[i]
			a.mu.Unlock()
			break
		}
	}
}

func (a *appData) convertGvrToItemList() {
	var itemNames []string
	for _, gvr := range a.gvrList {
		itemNames = append(itemNames, gvr.Name)
	}
	slices.Sort(itemNames)

	var items []list.Item
	for _, gvr := range itemNames {
		items = append(items, item(gvr))
	}
	a.list = initializeGvrList(items)
}

func (a *appData) fetchKubeData() error {
	kubeCfg, err := kube.InitializeK8sClientConfig()
	if err != nil {
		return err
	}

	discoClient, err := kube.NewDiscoveryClient(kubeCfg)
	if err != nil {
		return err
	}

	dynClient, err := kube.GetDynamicClient(kubeCfg)
	if err != nil {
		return err
	}

	typedClient, err := kubernetes.NewForConfig(kubeCfg)
	if err != nil {
		return err
	}

	a.clients.Discovery = discoClient
	a.clients.Dynamic = dynClient
	a.clients.Typed = typedClient

	return nil
}

func (a *appData) initNamespaceWatcher(ctx context.Context) {
	nsGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	factory := dynamicinformer.NewDynamicSharedInformerFactory(a.clients.Dynamic.Client, time.Minute*30)
	informer := factory.ForResource(nsGVR).Informer()

	syncNamespaces := func() {
		objs := informer.GetStore().List()
		nsNames := make([]string, 0, len(objs))
		for _, obj := range objs {
			if unstr, ok := obj.(*unstructured.Unstructured); ok {
				nsNames = append(nsNames, unstr.GetName())
			}
		}

		slices.Sort(nsNames)
		nsNames = append([]string{"all"}, nsNames...)
		select {
		case a.namespaceUpdates <- nsNames:
		case <-ctx.Done():
		default:
			// Channel full, skip update
		}
	}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { syncNamespaces() },
		DeleteFunc: func(obj interface{}) { syncNamespaces() },
	})

	go informer.Run(ctx.Done())
}

func (a *appData) shutdown() {
	close(a.shutdownChannels)

	a.mu.Lock()
	if a.cancelInformer != nil {
		a.cancelInformer()
	}
	a.mu.Unlock()

	a.informerWg.Wait()

	close(a.resourceUpdates)
	close(a.namespaceUpdates)
}
