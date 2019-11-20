package main

import (
	"log"
	"os"
	"strings"

	"github.com/FabianKramm/kube-problem/pkg/kube"
	"github.com/FabianKramm/kube-problem/pkg/runner"
	"github.com/FabianKramm/kube-problem/pkg/slack"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	// Try to get a cluster client
	client, err := kube.GetInClusterClient()
	if err != nil {
		var defaultClientErr error
		client, defaultClientErr = kube.GetDefaultClient()
		if defaultClientErr != nil {
			log.Fatal(err)
		}

		log.Println("Using kube config client")
	} else {
		log.Println(("Using in cluster kube client"))
	}

	// Create a new slack client
	slackClient, err := slack.NewClient(os.Getenv("SLACK_TOKEN"), os.Getenv("SLACK_CHANNEL"))
	if err != nil {
		log.Fatalf("Error creating slack client: %v", err)
	}

	// Verify the client is working
	slackChannel, err := slackClient.GetChannelInfo()
	if err != nil {
		log.Fatalf("Error getting slack channel info: %v", err)
	}
	log.Printf("Using slack channel '%s' for alerts", slackChannel.Name)

	os.Setenv("WATCH_NAMESPACES", "default")

	// Create the runner
	runner, err := runner.NewRunner(client, slackClient, os.Getenv("WATCH_NODES") != "false", strings.Split(os.Getenv("WATCH_NAMESPACES"), ","))
	if err != nil {
		log.Fatal(err)
	}

	// Start the runner
	err = runner.Start()
	if err != nil {
		log.Fatalf("Error in runner: %v", err)
	}
}
