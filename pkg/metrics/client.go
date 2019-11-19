package metrics

import (
	metricsapi "k8s.io/metrics/pkg/apis/metrics"
	metricsV1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/FabianKramm/kube-problem/pkg/kube"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubectl/pkg/metricsutil"
)

var (
	supportedMetricsAPIVersions = []string{
		"v1beta1",
	}
)

// Client is the client we use to retrieve the metrics
type Client struct {
	apiClient      metricsclientset.Interface
	heapsterClient *metricsutil.HeapsterMetricsClient

	isAPIAvailable bool
}

// NewMetricsClient creates a new metrics client
func NewMetricsClient(kubeClient kube.Client) (*Client, error) {
	client, err := metricsclientset.NewForConfig(kubeClient.Config())
	if err != nil {
		return nil, errors.Wrap(err, "new metrics clientset")
	}

	heapsterClient := metricsutil.NewHeapsterMetricsClient(kubeClient.Client().CoreV1(), metricsutil.DefaultHeapsterNamespace, metricsutil.DefaultHeapsterScheme, metricsutil.DefaultHeapsterService, metricsutil.DefaultHeapsterPort)

	isAPIAvailable, err := IsMetricsAPIAvailable(kubeClient)
	if err != nil {
		return nil, errors.Wrap(err, "is metrics api available")
	}

	return &Client{
		apiClient:      client,
		heapsterClient: heapsterClient,
		isAPIAvailable: isAPIAvailable,
	}, nil
}

// GetNodeMetrics retrieves the node metrics for a given name or labelSelector
func (c *Client) GetNodeMetrics(name, labelSelector string) (*metricsapi.NodeMetricsList, error) {
	var err error
	selector := labels.Everything()
	if labelSelector != "" {
		selector, err = labels.Parse(labelSelector)
		if err != nil {
			return nil, err
		}
	}

	metrics := &metricsapi.NodeMetricsList{}
	if c.isAPIAvailable {
		metrics, err = getNodeMetricsFromMetricsAPI(c.apiClient, name, selector)
		if err != nil {
			return nil, err
		}
	} else {
		metrics, err = c.heapsterClient.GetNodeMetrics(name, selector.String())
		if err != nil {
			return nil, err
		}
	}

	return metrics, nil
}

// GetPodMetrics retrieves the pod metrics for a given namespace and resource name
func (c *Client) GetPodMetrics(namespace, name, selector string, allNamespaces bool) (*metricsapi.PodMetricsList, error) {
	var err error

	metrics := &metricsapi.PodMetricsList{}
	if c.isAPIAvailable {
		metrics, err = getMetricsFromMetricsAPI(c.apiClient, namespace, name, selector, allNamespaces)
		if err != nil {
			return nil, err
		}
	} else {
		labelSelector := labels.Everything()
		if selector != "" {
			labelSelector, err = labels.Parse(selector)
			if err != nil {
				return nil, errors.Wrap(err, "parse selector")
			}
		}

		metrics, err = c.heapsterClient.GetPodMetrics(namespace, name, allNamespaces, labelSelector)
		if err != nil {
			return nil, err
		}
	}

	return metrics, nil
}

func getNodeMetricsFromMetricsAPI(metricsClient metricsclientset.Interface, resourceName string, selector labels.Selector) (*metricsapi.NodeMetricsList, error) {
	var err error
	versionedMetrics := &metricsV1beta1api.NodeMetricsList{}
	mc := metricsClient.MetricsV1beta1()
	nm := mc.NodeMetricses()
	if resourceName != "" {
		m, err := nm.Get(resourceName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		versionedMetrics.Items = []metricsV1beta1api.NodeMetrics{*m}
	} else {
		versionedMetrics, err = nm.List(metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			return nil, err
		}
	}

	metrics := &metricsapi.NodeMetricsList{}
	err = metricsV1beta1api.Convert_v1beta1_NodeMetricsList_To_metrics_NodeMetricsList(versionedMetrics, metrics, nil)
	if err != nil {
		return nil, err
	}
	return metrics, nil
}

func getMetricsFromMetricsAPI(metricsClient metricsclientset.Interface, namespace, resourceName string, selector string, allNamespaces bool) (*metricsapi.PodMetricsList, error) {
	var err error
	ns := metav1.NamespaceAll
	if !allNamespaces {
		ns = namespace
	}

	versionedMetrics := &metricsV1beta1api.PodMetricsList{}
	if resourceName != "" {
		m, err := metricsClient.MetricsV1beta1().PodMetricses(ns).Get(resourceName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		versionedMetrics.Items = []metricsV1beta1api.PodMetrics{*m}
	} else {
		versionedMetrics, err = metricsClient.MetricsV1beta1().PodMetricses(ns).List(metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return nil, err
		}
	}

	metrics := &metricsapi.PodMetricsList{}
	err = metricsV1beta1api.Convert_v1beta1_PodMetricsList_To_metrics_PodMetricsList(versionedMetrics, metrics, nil)
	if err != nil {
		return nil, err
	}

	return metrics, nil
}

// IsMetricsAPIAvailable checks if the metrics api is available
func IsMetricsAPIAvailable(kubeClient kube.Client) (bool, error) {
	apiGroups, err := kubeClient.Client().DiscoveryClient.ServerGroups()
	if err != nil {
		return false, err
	}

	for _, discoveredAPIGroup := range apiGroups.Groups {
		if discoveredAPIGroup.Name != metricsapi.GroupName {
			continue
		}
		for _, version := range discoveredAPIGroup.Versions {
			for _, supportedVersion := range supportedMetricsAPIVersions {
				if version.Version == supportedVersion {
					return true, nil
				}
			}
		}
	}

	return false, nil
}
