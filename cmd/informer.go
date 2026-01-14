package main

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
	"log"
	"time"
)

func (m *model) runInformer() {
	// Cancel existing informer and wait for shutdown completion
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
		return
	}

	// In unwatchable, add Wg and start polling for manual sync
	if !selectedGvr.Watchable {
		m.entity.Data.informerWg.Add(1)
		go func() {
			defer m.entity.Data.informerWg.Done()
			m.startPolling(ctx)
		}()
		return
	}

	dynamicFactory := m.entity.Data.dynFact
	informer := dynamicFactory.ForResource(m.entity.Data.selectedGvr.GVR).Informer()

	syncToTUI := func() {
		objs := informer.GetStore().List()
		var objects []*unstructured.Unstructured
		for _, obj := range objs {
			if unstructuredResource, ok := obj.(*unstructured.Unstructured); ok {
				objects = append(objects, unstructuredResource)
			}
		}

		select {
		case m.entity.Data.resourceUpdates <- objects:
		case <-ctx.Done():
			return
		default:
			// Channel full, skip update
		}
	}

	// Callbacks for watcher (all of which sync the Items list from scratch) :3
	// TODO (ozerova): Consider a more efficient way of doing this please..
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { syncToTUI() },
		UpdateFunc: func(old, new interface{}) { syncToTUI() },
		DeleteFunc: func(obj interface{}) { syncToTUI() },
	})

	if err != nil {
		log.Printf("WARN: Issue adding event handler: %v", err)
		return
	}

	// Run informer, set Wg to watch when this routine needs to end
	m.entity.Data.informerWg.Add(1)
	go func() {
		defer m.entity.Data.informerWg.Done()
		informer.Run(ctx.Done())
	}()
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
