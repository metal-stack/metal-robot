package github

import (
	"fmt"
	"net/http"

	"go.uber.org/zap"
	"gopkg.in/go-playground/webhooks.v5/github"
)

var (
	githubEvents = []github.Event{github.ReleaseEvent, github.PullRequestEvent}
)

// Controller that retrieves and handles github webhook events
type Controller struct {
	logger *zap.SugaredLogger
	hook   *github.Webhook
}

// NewController returns a new webhook controller
func NewController(logger *zap.SugaredLogger, webhookSecret string) (*Controller, error) {
	hook, err := github.New(github.Options.Secret(webhookSecret))
	if err != nil {
		return nil, err
	}

	controller := &Controller{
		logger: logger,
		hook:   hook,
	}
	return controller, nil
}

// Webhook handles a webhook event
func (c *Controller) Webhook(response http.ResponseWriter, request *http.Request) {
	payload, err := c.hook.Parse(request, githubEvents...)
	if err != nil {
		if err == github.ErrEventNotFound {
			c.logger.Warnw("received unexpected github event", "error", err)
			response.WriteHeader(http.StatusOK)
		} else {
			c.logger.Errorw("unable to handle github event", "error", err)
			response.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	switch payload := payload.(type) {
	case github.ReleasePayload:
		fmt.Printf("%+v", payload)
	case github.PullRequestPayload:
		fmt.Printf("%+v", payload)
	default:
		c.logger.Warnw("missing handler", "payload", payload)
	}

	response.WriteHeader(http.StatusOK)
}
