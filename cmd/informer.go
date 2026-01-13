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
	if m.entity.Data.cancelInformer != nil {
		m.entity.Data.cancelInformer()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.entity.Data.cancelInformer = cancel

	if !m.entity.Data.selectedGvr.Watchable {
		go m.startPolling(ctx)
		return
	}

	dynamicFactory := m.entity.Data.dynFact
	informer := dynamicFactory.ForResource(m.entity.Data.selectedGvr.GVR).Informer()

	syncToTUI := func() {
		if m.entity.Data.program == nil {
			return
		}

		objs := informer.GetStore().List()
		var objects []*unstructured.Unstructured
		for _, obj := range objs {
			if unstructuredResource, ok := obj.(*unstructured.Unstructured); ok {
				objects = append(objects, unstructuredResource)
			}
		}

		m.entity.Data.program.Send(ResourceUpdateMsg(objects))
	}

	// Callbacks for watcher (all of which sync the Items list from scratch) :3
	// TODO (ozerova): Consider a more efficient way of doing this please..
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { syncToTUI() },
		UpdateFunc: func(old, new interface{}) { syncToTUI() },
		DeleteFunc: func(obj interface{}) { syncToTUI() },
	})

	if err != nil {
		log.Fatal(err)
	}

	go informer.Run(ctx.Done())
}

func (m *model) pullResourcesOnce() {
	gvr := m.entity.Data.selectedGvr.GVR

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

	// Send to TUI
	m.entity.Data.program.Send(ResourceUpdateMsg(objects))
}

func (m *model) startPolling(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.pullResourcesOnce()
		}
	}
}
