package util

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
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

func getKubeConfigFile() (string, error) {
	kubeConfigFile := os.Getenv(kubeConfigFileEnv)
	if kubeConfigFile == "" {
		user, err := user.Current()
		if err != nil {
			return "", err
		}
		kubeConfigFile = path.Join(user.HomeDir, ".kube", "config")
	}

	return kubeConfigFile, nil
}

func NewDynamicClient() (dynamic.Interface, error) {
	kubeConfigFile, err := getKubeConfigFile()
	if err != nil {
		return nil, err
	}
	fmt.Printf("Use kubeconfig file: %s\n", kubeConfigFile)

	clusterCfg, err := clientcmd.BuildConfigFromFlags("", kubeConfigFile)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(clusterCfg)
	if err != nil {
		return nil, err
	}

	return dynamicClient, nil
}

func GetClusterImageSets(dynamicClient dynamic.Interface) (
	*unstructured.UnstructuredList, error) {

	obj, err := dynamicClient.Resource(imagesetGVR).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	return obj, nil
}
