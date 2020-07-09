package gitlab

import (
	"net/http"

	"go.uber.org/zap"
	"gopkg.in/go-playground/webhooks.v5/gitlab"
)

var (
	gitlabEvents = []gitlab.Event{gitlab.PushEvents}
)

// Controller that retrieves and handles gitlab webhook events
type Controller struct {
	logger *zap.SugaredLogger
	hook   *gitlab.Webhook
}

// NewController returns a new webhook controller
func NewController(logger *zap.SugaredLogger, webhookSecret string) (*Controller, error) {
	hook, err := gitlab.New(gitlab.Options.Secret(webhookSecret))
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
	payload, err := c.hook.Parse(request, gitlabEvents...)
	if err != nil {
		if err == gitlab.ErrEventNotFound {
			c.logger.Warnw("received unexpected gitlab event", "error", err)
			response.WriteHeader(http.StatusOK)
		} else {
			c.logger.Errorw("unable to handle gitlab event", "error", err)
			response.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	switch payload := payload.(type) {
	case gitlab.PushEventPayload:
		c.logger.Debugw("received push event")
	default:
		c.logger.Warnw("missing handler", "payload", payload)
	}

	response.WriteHeader(http.StatusOK)
}
