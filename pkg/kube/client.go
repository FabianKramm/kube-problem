package kube

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client is the kubernetes client that holds the rest config and clientset
type Client interface {
	Config() *rest.Config
	Client() *kubernetes.Clientset
}

type client struct {
	config *rest.Config
	client *kubernetes.Clientset
}

func (c *client) Config() *rest.Config {
	return c.config
}

func (c *client) Client() *kubernetes.Clientset {
	return c.client
}

// GetInClusterClient retrieves a new kubernetes clientset
func GetInClusterClient() (Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &client{
		config: config,
		client: clientset,
	}, nil
}

// GetDefaultClient retrieves the default config client
func GetDefaultClient() (Client, error) {
	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, err
	}

	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &client{
		config: config,
		client: clientset,
	}, nil
}
