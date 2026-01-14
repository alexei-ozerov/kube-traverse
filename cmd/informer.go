package main

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)


func (m *model) runInformer() tea.Cmd {
	return func() tea.Msg {
		// Moving Wait() inside the return function ensures it runs 
		// in a background goroutine, not the main UI thread.
		m.entity.Data.mu.Lock()
		if m.entity.Data.cancelInformer != nil {
			m.entity.Data.cancelInformer()
			m.entity.Data.mu.Unlock()
			m.entity.Data.informerWg.Wait() 
		} else {
			m.entity.Data.mu.Unlock()
		}

		ctx, cancel := context.WithCancel(context.Background())

		m.entity.Data.mu.Lock()
		m.entity.Data.cancelInformer = cancel
		selectedGvr := m.entity.Data.selectedGvr
		m.entity.Data.mu.Unlock()

		if selectedGvr == nil {
			return nil
		}

		if !selectedGvr.Watchable {
			m.entity.Data.informerWg.Go(func() {
				m.startPolling(ctx)
			})
			return nil
		}

		dynamicFactory := m.entity.Data.dynFact
		informer := dynamicFactory.ForResource(selectedGvr.GVR).Informer()

		syncToTUI := func() {
			objs := informer.GetStore().List()
			var objects []*unstructured.Unstructured
			for _, obj := range objs {
				if unstr, ok := obj.(*unstructured.Unstructured); ok {
					objects = append(objects, unstr)
				}
			}

			select {
			case m.entity.Data.resourceUpdates <- objects:
			case <-ctx.Done():
				return
			default:
			}
		}

		_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj any) { syncToTUI() },
			UpdateFunc: func(old, new any) { syncToTUI() },
			DeleteFunc: func(obj any) { syncToTUI() },
		})

		if err != nil {
			return nil
		}

		m.entity.Data.informerWg.Go(func() {
			informer.Run(ctx.Done())
		})
		return nil
	}
}
func (m *model) pullResourcesOnce() {
	m.entity.Data.mu.RLock()
	selectedGvr := m.entity.Data.selectedGvr
	m.entity.Data.mu.RUnlock()

	if selectedGvr == nil {
		return
	}

	gvr := selectedGvr.GVR

	list, err := m.entity.Data.clients.Dynamic.Client.
		Resource(gvr).
		List(context.Background(), metav1.ListOptions{})

	if err != nil {
		return
	}

	var objects []*unstructured.Unstructured
	for i := range list.Items {
		objects = append(objects, &list.Items[i])
	}

	select {
	case m.entity.Data.resourceUpdates <- objects:
	default:
		// Channel full, skip
	}
}

func (m *model) startPolling(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial pull
	m.pullResourcesOnce()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.pullResourcesOnce()
		}
	}
}
