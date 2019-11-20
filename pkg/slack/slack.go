package slack

import (
	"errors"
	"log"
	"strings"

	slackapi "github.com/nlopes/slack"
)

// Client is the slack client struct
type Client struct {
	API     *slackapi.Client
	Channel string
}

// NewClient creates a new slack client to use
func NewClient(token, channel string) (*Client, error) {
	if token == "" {
		return nil, errors.New("No slack token provided. Is env variable SLACK_TOKEN set?")
	}
	if channel == "" {
		return nil, errors.New("No slack channel provided. Is env variable SLACK_CHANNEL set?")
	}

	return &Client{
		API:     slackapi.New(token),
		Channel: channel,
	}, nil
}

// GetChannelInfo returns the channel info
func (c *Client) GetChannelInfo() (*slackapi.Channel, error) {
	return c.API.GetConversationInfo(c.Channel, false)
}

// SendMessage sends a new slack message to the channel
func (c *Client) SendMessage(message string) error {
	var err error
	shouldRetry := true
	for shouldRetry {
		_, _, err = c.API.PostMessage(c.Channel, slackapi.MsgOptionText(message, false))
		shouldRetry = isNetErrorRetryable(err)
		if err != nil && shouldRetry {
			log.Printf("Retry sending to slack due to error: %v", err)
		}
	}

	return err
}

// isNetErrorRetryable - is network error retryable.
func isNetErrorRetryable(err error) bool {
	if err == nil {
		return false
	}

	if strings.Contains(err.Error(), "Connection closed by foreign host") {
		return true
	} else if strings.Contains(err.Error(), "net/http: TLS handshake timeout") {
		return true
	} else if strings.Contains(err.Error(), "i/o timeout") {
		return true
	} else if strings.Contains(err.Error(), "connection timed out") {
		return true
	}

	return false
}
