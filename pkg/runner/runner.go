package runner

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/FabianKramm/kube-problem/pkg/kube"
	"github.com/FabianKramm/kube-problem/pkg/metrics"
	"github.com/FabianKramm/kube-problem/pkg/slack"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultInterval = time.Second * 10
const reportInterval = time.Minute * 60

type problemType string

const (
	problemTypeNodeCondition        problemType = "NodeCondition"
	problemTypeNodeResourcePressure problemType = "NodeResourcePressure"

	problemTypePodStatus   problemType = "PodStatus"
	problemTypePodRestarts problemType = "PodRestarts"
	problemTypePodPending  problemType = "PodPending"
)

type resourceKind string

const (
	resourceKindPod  resourceKind = "Pod"
	resourceKindNode resourceKind = "Node"
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
	kind        resourceKind
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

		problems: make(map[string]*problemDesc),
	}, nil
}

// Start starts the runner (blocking)
func (r *Runner) Start() error {
	log.Printf("Starting runner with interval of %d seconds", defaultInterval/time.Second)

	for {
		start := time.Now()

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

		// Sleep for the remainding interval duration
		wait := defaultInterval - time.Since(start)
		if wait > 0 {
			time.Sleep(wait)
		}

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
	if r.problems[problem.id].reported == false {
		log.Printf("Problem occured (not reported yet, counter: %d): %s", r.problems[problem.id].occuredCounter, problem.message)
	}

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
	if problem.reported == true {
		log.Printf("Problem resolved ('%s') (resolving not reported yet, counter: %d)", problem.message, problem.resolvedCounter)
	}

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
	msg := fmt.Sprintf("%s do you remember the problem with %s '%s'? Good news, seems like this is not a problem anymore :tada:", getGreeting(), problem.kind, problem.name)
	log.Printf("Sending resolve message to slack (%s)", msg)
	return r.slackClient.SendMessage(msg)
}

func (r *Runner) sendReportMessage(problem *problemDesc) error {
	if problem.reported {
		return nil
	}

	problem.reported = true
	if problem.namespace != "" {
		msg := fmt.Sprintf("%s there seems to be a problem with %s '%s' in namespace '%s': %s", getGreeting(), problem.kind, problem.name, problem.namespace, problem.message)
		log.Printf("Sending report message to slack (%s)", msg)
		return r.slackClient.SendMessage(msg)
	}

	msg := fmt.Sprintf("%s there seems to be a problem with %s '%s': %s", getGreeting(), problem.kind, problem.name, problem.message)
	log.Printf("Sending report message to slack (%s)", msg)
	return r.slackClient.SendMessage(msg)
}

var greetings = []string{
	"Guys real talk :point_up:,",
	"It's me again, the lovely bot from the neighborhood and",
	"Alright, so",
	"Yo bois :dark_sunglasses:,",
	"Sorry to interrupt,",
	"I'm back :v:,",
	"Yes I know I'm annoying :grin:, but",
	"Where is the cluster admin :face_with_monocle:, because",
	"I just wanted to chill :expressionless: and then I checked the cluster one more time and",
	"What would you do without me? I just checked the cluster again and",
}

func getGreeting() string {
	rand.Seed(time.Now().Unix())

	num := rand.Intn(len(greetings) + 1)
	if num == len(greetings) {
		now := time.Now()
		if now.Weekday() == time.Sunday {
			return "Damn sorry to interrupt your Sunday :face_with_rolling_eyes:, but"
		} else if now.Weekday() == time.Saturday {
			return "Yes I know it's weekend, but"
		}

		if now.Hour() < 12 {
			return "Good morning everyone :wave:,"
		} else if now.Hour() < 15 {
			return "Hello everyone :wave:,"
		} else if now.Hour() < 18 {
			return "Good afternoon everyone :wave:,"
		}

		return "Good evening everyone :wave:,"
	}

	return greetings[num]
}
