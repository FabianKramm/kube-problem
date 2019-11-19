package slack

import (
	"errors"

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
	_, _, err := c.API.PostMessage(c.Channel, slackapi.MsgOptionText(message, false))
	if err != nil {
		return err
	}

	return nil
}
