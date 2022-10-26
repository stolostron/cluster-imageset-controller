package util

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	ChannelLabel      = "channel"
	kubeConfigFileEnv = "KUBECONFIG"
)

var imagesetGVR = schema.GroupVersionResource{
	Group:    "hive.openshift.io",
	Version:  "v1",
	Resource: "clusterimagesets",
}

func GetClusterImageSets(dynamicClient dynamic.Interface) (
	*unstructured.UnstructuredList, error) {

	obj, err := dynamicClient.Resource(imagesetGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return obj, nil
}
