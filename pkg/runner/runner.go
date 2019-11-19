package runner

import (
	"fmt"
	"log"
	"time"

	"github.com/FabianKramm/kube-problem/pkg/kube"
	"github.com/FabianKramm/kube-problem/pkg/metrics"
	"github.com/FabianKramm/kube-problem/pkg/slack"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultInterval = time.Minute
const reportInterval = time.Minute * 60

type problemType string

const (
	problemTypeNodeCondition        problemType = "NodeCondition"
	problemTypeNodeResourcePressure problemType = "NodeResourcePressure"

	problemTypePodStatus   problemType = "PodStatus"
	problemTypePodRestarts problemType = "PodRestarts"
	problemTypePodPending  problemType = "PodPending"
)

// Runner is continously checking for problems in a cluster
type Runner struct {
	client        kube.Client
	metricsClient *metrics.Client
	slackClient   *slack.Client

	watchNodes      bool
	watchNamespaces []string

	problems map[string]*problemDesc
}

type problemDesc struct {
	problemType problemType
	kind        string
	name        string
	namespace   string

	id      string
	message string

	resolvedCounter int
	occuredCounter  int

	reported bool
	occured  time.Time
}

// NewRunner creates a new runner
func NewRunner(client kube.Client, slackClient *slack.Client, watchNodes bool, watchNamespaces []string) (*Runner, error) {
	metricsClient, err := metrics.NewMetricsClient(client)
	if err != nil {
		return nil, err
	}

	if watchNodes {
		// Check if we can access nodes
		_, err := client.Client().CoreV1().Nodes().List(metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("Error retrieving nodes: %v", err)
		}
	}

	if len(watchNamespaces) > 0 {
		// Check if namespaces exist
		for _, namespace := range watchNamespaces {
			_, err := client.Client().CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
			if err != nil {
				return nil, fmt.Errorf("Error retrieving namespace %s: %v", namespace, err)
			}
		}
	}

	return &Runner{
		client:        client,
		metricsClient: metricsClient,
		slackClient:   slackClient,

		watchNodes:      watchNodes,
		watchNamespaces: watchNamespaces,
	}, nil
}

// Start starts the runner (blocking)
func (r *Runner) Start() error {
	log.Printf("Starting runner with interval of %d seconds", defaultInterval/time.Second)

	for {
		// Watch nodes
		if r.watchNodes {
			err := r.doWatchNodes()
			if err != nil {
				return err
			}
		}

		// Watch namespaces
		if len(r.watchNamespaces) > 0 {
			for _, namespace := range r.watchNamespaces {
				err := r.doWatchNamespace(namespace)
				if err != nil {
					return err
				}
			}
		}

		// Sleep
		time.Sleep(defaultInterval)

		// Cleanup old problems
		for key, problem := range r.problems {
			if time.Since(problem.occured) > time.Minute*30 {
				delete(r.problems, key)
			}
		}
	}
}

func (r *Runner) reportProblem(problem *problemDesc) error {
	if r.problems[problem.id] == nil {
		r.problems[problem.id] = problem
	}

	r.problems[problem.id].occuredCounter++

	// Node condition
	if r.problems[problem.id].problemType == problemTypeNodeCondition {
		return r.sendReportMessage(r.problems[problem.id])
	}

	// Node resource pressure
	if r.problems[problem.id].problemType == problemTypeNodeResourcePressure && r.problems[problem.id].occuredCounter >= 10 {
		return r.sendReportMessage(r.problems[problem.id])
	}

	// Pod critical status
	if r.problems[problem.id].problemType == problemTypePodStatus {
		return r.sendReportMessage(r.problems[problem.id])
	}

	// Pod pending
	if r.problems[problem.id].problemType == problemTypePodPending && r.problems[problem.id].occuredCounter >= 30 {
		return r.sendReportMessage(r.problems[problem.id])
	}

	// Pod restarts
	if r.problems[problem.id].problemType == problemTypePodRestarts {
		return r.sendReportMessage(r.problems[problem.id])
	}

	return nil
}

func (r *Runner) resolveProblem(problem *problemDesc) error {
	problem = r.problems[problem.id]
	problem.resolvedCounter++

	// Node condition
	if problem.problemType == problemTypeNodeCondition {
		delete(r.problems, problem.id)
		if problem.reported {
			return r.sendResolveMessage(problem)
		}

		return nil
	}

	// Node resource pressure
	if problem.problemType == problemTypeNodeResourcePressure && problem.resolvedCounter >= 5 {
		delete(r.problems, problem.id)
		if problem.reported {
			return r.sendResolveMessage(problem)
		}

		return nil
	}

	// Pod critical status
	if problem.problemType == problemTypePodStatus && problem.resolvedCounter >= 10 {
		delete(r.problems, problem.id)
		if problem.reported {
			return r.sendResolveMessage(problem)
		}

		return nil
	}

	// Pod pending
	if problem.problemType == problemTypePodPending && problem.resolvedCounter >= 10 {
		delete(r.problems, problem.id)
		if problem.reported {
			return r.sendResolveMessage(problem)
		}

		return nil
	}

	return nil
}

func (r *Runner) sendResolveMessage(problem *problemDesc) error {
	msg := fmt.Sprintf("%s everyone :v:, it's me again, remember the problem with %s %s? Good news, seems like this is not a problem anymore :tada:", getGreeting(), problem.kind, problem.name)
	log.Printf("Sending resolve message to slack (%s)", msg)
	return r.slackClient.SendMessage(msg)
}

func (r *Runner) sendReportMessage(problem *problemDesc) error {
	if problem.reported {
		return nil
	}

	problem.reported = true
	if problem.namespace != "" {
		msg := fmt.Sprintf("%s everyone :wave:, there seems to be a problem with %s %s in namespace %s: %s", getGreeting(), problem.kind, problem.name, problem.namespace, problem.message)
		log.Printf("Sending report message to slack (%s)", msg)
		return r.slackClient.SendMessage(msg)
	}

	msg := fmt.Sprintf("%s everyone :wave:, there seems to be a problem with %s %s: %s", getGreeting(), problem.kind, problem.name, problem.message)
	log.Printf("Sending report message to slack (%s)", msg)
	return r.slackClient.SendMessage(msg)
}

func getGreeting() string {
	now := time.Now()
	if now.Hour() < 12 {
		return "Good morning"
	} else if now.Hour() < 15 {
		return "Hello"
	} else if now.Hour() < 18 {
		return "Good afternoon"
	}

	return "Good evening"
}
