package gitlab

import (
	"net/http"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"go.uber.org/zap"
	glwebhooks "gopkg.in/go-playground/webhooks.v5/gitlab"
)

var (
	listenEvents = []glwebhooks.Event{
		glwebhooks.PushEvents,
	}
)

type Webhook struct {
	logger *zap.SugaredLogger
	cs     clients.ClientMap
	hook   *glwebhooks.Webhook
}

// NewGitlabWebhook returns a new webhook controller
func NewGitlabWebhook(logger *zap.SugaredLogger, w config.Webhook, cs clients.ClientMap) (*Webhook, error) {
	hook, err := glwebhooks.New(glwebhooks.Options.Secret(w.Secret))
	if err != nil {
		return nil, err
	}

	controller := &Webhook{
		logger: logger,
		cs:     cs,
		hook:   hook,
	}

	return controller, nil
}

// GitlabWebhooks handles gitlab webhook events
func (c *Webhook) Handle(response http.ResponseWriter, request *http.Request) {
	payload, err := c.hook.Parse(request, listenEvents...)
	if err != nil {
		if err == glwebhooks.ErrEventNotFound {
			c.logger.Warnw("received unregistered gitlab event", "error", err)
			response.WriteHeader(http.StatusOK)
		} else {
			c.logger.Errorw("received malformed gitlab event", "error", err)
			response.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	switch payload := payload.(type) {
	case glwebhooks.PushEventPayload:
		c.logger.Debugw("received push event")
	default:
		c.logger.Warnw("missing handler", "payload", payload)
	}

	response.WriteHeader(http.StatusOK)
}
