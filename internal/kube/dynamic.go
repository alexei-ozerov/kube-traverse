package kube

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

func GetDynamicClient(config *rest.Config) (*DynamicClient, error) {
	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &DynamicClient{Client: dynClient}, nil
}
