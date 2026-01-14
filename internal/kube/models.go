package kube

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

// Ctx Wrapper around the entities we want to expose to a consumer
type Ctx struct {
	Discovery *DiscoveryClient
	Dynamic   *DynamicClient
}

// ApiResource Metadata we want to extract for a K8s type
type ApiResource struct {
	Name         string
	Kind         string
	Namespaced   bool
	Watchable    bool
	GVR          schema.GroupVersionResource
	SubResources []string
}

type DiscoveryClient struct {
	Client discovery.DiscoveryInterface
}

type DynamicClient struct {
	Client dynamic.Interface
}
