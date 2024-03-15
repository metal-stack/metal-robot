package gitlab

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	glwebhooks "github.com/go-playground/webhooks/v6/gitlab"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/gitlab/actions"
)

var (
	listenEvents = []glwebhooks.Event{
		glwebhooks.TagEvents,
	}
)

type Webhook struct {
	logger *slog.Logger
	cs     clients.ClientMap
	hook   *glwebhooks.Webhook
	a      *actions.WebhookActions
}

// NewGitlabWebhook returns a new webhook controller
func NewGitlabWebhook(logger *slog.Logger, w config.Webhook, cs clients.ClientMap) (*Webhook, error) {
	hook, err := glwebhooks.New(glwebhooks.Options.Secret(w.Secret))
	if err != nil {
		return nil, err
	}

	a, err := actions.InitActions(logger, cs, w.Actions)
	if err != nil {
		return nil, err
	}

	controller := &Webhook{
		logger: logger,
		cs:     cs,
		hook:   hook,
		a:      a,
	}

	return controller, nil
}

// GitlabWebhooks handles gitlab webhook events
func (w *Webhook) Handle(response http.ResponseWriter, request *http.Request) {
	payload, err := w.hook.Parse(request, listenEvents...)
	if err != nil {
		if errors.Is(err, glwebhooks.ErrEventNotFound) {
			w.logger.Warn("received unregistered gitlab event", "error", err)
			response.WriteHeader(http.StatusOK)
		} else {
			w.logger.Error("received malformed gitlab event", "error", err)
			response.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	ctx := context.Background()
	switch payload := payload.(type) {
	case glwebhooks.TagEventPayload:
		w.logger.Debug("received tag push event")
		// nolint:contextcheck
		w.a.ProcessTagEvent(ctx, &payload)
	default:
		w.logger.Warn("missing handler", "payload", payload)
	}

	response.WriteHeader(http.StatusOK)
}
