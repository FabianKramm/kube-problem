package runner

import (
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/util/node"
)

// OkayStatus container status
var OkayStatus = map[string]bool{
	"Completed": true,
	"Running":   true,
}

// CriticalStatus container status
var CriticalStatus = map[string]bool{
	"Error":                      true,
	"Unknown":                    true,
	"ImagePullBackOff":           true,
	"CrashLoopBackOff":           true,
	"RunContainerError":          true,
	"ErrImagePull":               true,
	"CreateContainerConfigError": true,
	"InvalidImageName":           true,
	"Evicted":                    true,
}

func (r *Runner) doWatchNamespace(namespace string) error {
	podList, err := r.client.Client().CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, pod := range podList.Items {
		var problem *problemDesc

		status := GetPodStatus(&pod)
		if CriticalStatus[status] {
			msg := fmt.Sprintf("Pod '%s/%s' has critical status '%s'", pod.Namespace, pod.Name, status)
			problem = &problemDesc{
				problemType: problemTypePodStatus,

				message: msg,
				id:      pod.Name + "/" + pod.Namespace + string(problemTypePodStatus),

				kind:      resourceKindPod,
				name:      pod.Name,
				namespace: pod.Namespace,
				occured:   time.Now(),
			}
		} else if OkayStatus[status] {
			for _, containerStatus := range pod.Status.ContainerStatuses {
				if containerStatus.LastTerminationState.Terminated != nil && time.Since(containerStatus.LastTerminationState.Terminated.FinishedAt.Time) <= time.Hour && containerStatus.LastTerminationState.Terminated.ExitCode != 0 {
					msg := fmt.Sprintf("Pod '%s/%s' has restarted %d seconds ago due to '%s' with exit code '%d'", pod.Namespace, pod.Name, time.Since(containerStatus.LastTerminationState.Terminated.FinishedAt.Time)/time.Second, containerStatus.LastTerminationState.Terminated.Reason, containerStatus.LastTerminationState.Terminated.ExitCode)
					problem = &problemDesc{
						problemType: problemTypePodRestarts,

						message: msg,
						id:      pod.Name + "/" + pod.Namespace + string(problemTypePodRestarts),

						kind:      resourceKindPod,
						name:      pod.Name,
						namespace: pod.Namespace,
						occured:   time.Now(),
					}

					break
				}
			}
		} else {
			msg := fmt.Sprintf("Pod '%s/%s' is not starting with status '%s'", pod.Namespace, pod.Name, status)
			problem = &problemDesc{
				problemType: problemTypePodPending,

				message: msg,
				id:      pod.Name + "/" + pod.Namespace + string(problemTypePodPending),

				kind:      resourceKindPod,
				name:      pod.Name,
				namespace: pod.Namespace,
				occured:   time.Now(),
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
				if problem.kind == resourceKindPod && problem.name == pod.Name && problem.namespace == pod.Namespace {
					err = r.resolveProblem(problem)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

// GetPodStatus returns the pod status as a string
// Taken from https://github.com/kubernetes/kubernetes/pkg/printers/internalversion/printers.go
func GetPodStatus(pod *v1.Pod) string {
	reason := string(pod.Status.Phase)

	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	initializing := false

	for i := range pod.Status.InitContainerStatuses {
		container := pod.Status.InitContainerStatuses[i]

		switch {
		case container.State.Terminated != nil && container.State.Terminated.ExitCode == 0:
			continue
		case container.State.Terminated != nil:
			// initialization is failed
			if len(container.State.Terminated.Reason) == 0 {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Init:Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("Init:ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else {
				reason = "Init:" + container.State.Terminated.Reason
			}
			initializing = true
		case container.State.Waiting != nil && len(container.State.Waiting.Reason) > 0 && container.State.Waiting.Reason != "PodInitializing":
			reason = "Init:" + container.State.Waiting.Reason
			initializing = true
		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(pod.Spec.InitContainers))
			initializing = true
		}
		break
	}
	if !initializing {
		hasRunning := false

		for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
			container := pod.Status.ContainerStatuses[i]

			if container.State.Waiting != nil && container.State.Waiting.Reason != "" {
				reason = container.State.Waiting.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason != "" {
				reason = container.State.Terminated.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason == "" {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else if container.Ready && container.State.Running != nil {
				hasRunning = true
			}
		}

		// change pod status back to "Running" if there is at least one container still reporting as "Running" status
		if reason == "Completed" && hasRunning {
			reason = "Running"
		}
	}

	if pod.DeletionTimestamp != nil && pod.Status.Reason == node.NodeUnreachablePodReason {
		reason = "Unknown"
	} else if pod.DeletionTimestamp != nil {
		reason = "Terminating"
	}

	return reason
}
