package runner

import (
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	metricsapi "k8s.io/metrics/pkg/apis/metrics"
)

func (r *Runner) doWatchNodes() error {
	var nodeMetricsAvailable bool = false
	var nodeMetricsMap = map[string]*metricsapi.NodeMetrics{}

	nodeMetrics, err := r.metricsClient.GetNodeMetrics("", "")
	if err == nil && nodeMetrics != nil {
		nodeMetricsAvailable = true
		for _, nodeMetric := range nodeMetrics.Items {
			metric := nodeMetric
			nodeMetricsMap[nodeMetric.Name] = &metric
		}
	}

	nodeList, err := r.client.Client().CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, node := range nodeList.Items {
		problem, err := isNodeProblem(&node)
		if err != nil {
			return err
		} else if nodeMetricsAvailable && nodeMetricsMap[node.Name] == nil {
			msg := fmt.Sprintf("Metrics for node %s cannot be retrieved. This could mean the node crashed or is under heavy load", node.Name)
			problem = &problemDesc{
				problemType: problemTypeNodeResourcePressure,
				kind:        node.Kind,
				name:        node.Name,

				id:      msg,
				message: msg,
				occured: time.Now(),
			}
		} else if nodeMetricsAvailable && nodeMetricsMap[node.Name] != nil {
			cpuUsed := nodeMetricsMap[node.Name].Usage.Cpu().MilliValue()
			cpuAvail := node.Status.Capacity.Cpu().MilliValue()
			cpuUsage := float64(cpuUsed) / float64(cpuAvail)

			memUsed := nodeMetricsMap[node.Name].Usage.Memory().MilliValue()
			memAvail := node.Status.Capacity.Memory().MilliValue()
			memUsage := float64(memUsed) / float64(memAvail)

			if cpuUsage >= 0.95 {
				msg := fmt.Sprintf("Node %s has constantly around 100%% cpu usage, this could slow down workloads running on the node", node.Name)
				problem = &problemDesc{
					problemType: problemTypeNodeResourcePressure,
					kind:        node.Kind,
					name:        node.Name,

					id:      msg,
					message: msg,
					occured: time.Now(),
				}
			} else if memUsage >= 0.95 {
				msg := fmt.Sprintf("Node %s has constantly around 100%% memory usage, this could slow down workloads running on the node", node.Name)
				problem = &problemDesc{
					problemType: problemTypeNodeResourcePressure,
					kind:        node.Kind,
					name:        node.Name,

					id:      msg,
					message: msg,
					occured: time.Now(),
				}
			}

			// Handle problem reporting or resolving
			if problem != nil {
				err = r.reportProblem(problem)
				if err != nil {
					return err
				}
			} else {
				for _, problem := range r.problems {
					if problem.kind == node.Kind && problem.name == node.Name {
						err = r.resolveProblem(problem)
						if err != nil {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

func isNodeProblem(node *v1.Node) (*problemDesc, error) {
	// Check for conditions
	for _, condition := range node.Status.Conditions {
		if condition.Type != v1.NodeReady && condition.Status != v1.ConditionFalse {
			msg := fmt.Sprintf("Node %s has condition (%s): %s", node.Name, condition.Type, condition.Message)
			return &problemDesc{
				problemType: problemTypeNodeCondition,
				kind:        node.Kind,
				name:        node.Name,

				message: msg,
				id:      msg,
				occured: time.Now(),
			}, nil
		} else if condition.Type == v1.NodeReady && condition.Status != v1.ConditionTrue {
			msg := fmt.Sprintf("Node %s has ready status '%s': %s", node.Name, condition.Status, condition.Message)
			return &problemDesc{
				problemType: problemTypeNodeCondition,
				kind:        node.Kind,
				name:        node.Name,

				message: msg,
				id:      msg,
				occured: time.Now(),
			}, nil
		}
	}

	return nil, nil
}

