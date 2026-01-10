package kube

import (
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

func NewDiscoveryClient(config *rest.Config) (*DiscoveryClient, error) {
	d, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	return &DiscoveryClient{client: d}, nil
}

func (d *DiscoveryClient) GetListableResources() ([]ApiResource, error) {
	lists, err := d.client.ServerPreferredResources()
	if err != nil {
		fmt.Printf("partial discovery results: %v\n", err)
	}

	var results []ApiResource
	for _, list := range lists {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}

		for _, res := range list.APIResources {
			if !slices.Contains(res.Verbs, "list") || strings.Contains(res.Name, "/") {
				continue
			}

			results = append(results, ApiResource{
				Name:       res.Name,
				Kind:       res.Kind,
				Namespaced: res.Namespaced,
				GVR: schema.GroupVersionResource{
					Group:    gv.Group,
					Version:  gv.Version,
					Resource: res.Name,
				},
			})
		}
	}

	return results, nil
}
