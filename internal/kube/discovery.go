package kube

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
)

func NewDiscoveryClient(config *rest.Config) (*DiscoveryClient, error) {
	d, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	return &DiscoveryClient{Client: d}, nil
}

func (d *DiscoveryClient) getCachePath() string {
	return filepath.Join(os.Getenv("HOME"), ".kube", "traverse_cache.json")
}

func (d *DiscoveryClient) GetCachedResources() ([]ApiResource, bool) {
	path := d.getCachePath()
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) > 24*time.Hour {
		return nil, false
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}

	var resources []ApiResource
	if err := json.Unmarshal(data, &resources); err != nil {
		return nil, false
	}
	return resources, true
}

func (d *DiscoveryClient) SaveResourcesToCache(resources []ApiResource) {
	data, _ := json.Marshal(resources)
	_ = os.WriteFile(d.getCachePath(), data, 0644)
}

func (d *DiscoveryClient) GetListableResources() ([]ApiResource, error) {
	_, allResourceLists, err := d.Client.ServerGroupsAndResources()
	if err != nil {
		log.Printf("discovery warning: %v\n", err)
	}

	preferredLists, _ := d.Client.ServerPreferredResources()
	preferredKeys := make(map[string]bool)
	for _, list := range preferredLists {
		gv, _ := schema.ParseGroupVersion(list.GroupVersion)
		for _, res := range list.APIResources {
			if !strings.Contains(res.Name, "/") && slices.Contains(res.Verbs, "list") {
				key := fmt.Sprintf("%s/%s/%s", gv.Group, gv.Version, res.Name)
				preferredKeys[key] = true
			}
		}
	}

	resourceMap := make(map[string]*ApiResource)
	for _, list := range allResourceLists {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil { continue }

		// Parents
		for _, res := range list.APIResources {
			if !strings.Contains(res.Name, "/") {
				key := fmt.Sprintf("%s/%s/%s", gv.Group, gv.Version, res.Name)
				if _, exists := resourceMap[key]; !exists {
					resourceMap[key] = &ApiResource{
						Name:         res.Name,
						Kind:         res.Kind,
						Namespaced:   res.Namespaced,
						Watchable:    slices.Contains(res.Verbs, "watch"),
						GVR:          schema.GroupVersionResource{Group: gv.Group, Version: gv.Version, Resource: res.Name},
						SubResources: []string{"spec"},
					}
				}
			}
		}

		for _, res := range list.APIResources {
			if strings.Contains(res.Name, "/") {
				parts := strings.Split(res.Name, "/")
				parentKey := fmt.Sprintf("%s/%s/%s", gv.Group, gv.Version, parts[0])
				if parent, ok := resourceMap[parentKey]; ok {
					if !slices.Contains(parent.SubResources, parts[1]) {
						parent.SubResources = append(parent.SubResources, parts[1])
					}
				}
			}
		}
	}

	var results []ApiResource
	seen := make(map[string]bool) 
	
	keys := make([]string, 0, len(preferredKeys))
	for k := range preferredKeys {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, key := range keys {
		if res, ok := resourceMap[key]; ok {
			if !seen[res.Name] {
				results = append(results, *res)
				seen[res.Name] = true
			}
		}
	}

	slices.SortFunc(results, func(a, b ApiResource) int {
		return strings.Compare(a.Name, b.Name)
	})

	return results, nil
}
