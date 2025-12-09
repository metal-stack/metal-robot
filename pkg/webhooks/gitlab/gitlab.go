package gitlab

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	glwebhooks "github.com/go-playground/webhooks/v6/gitlab"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/handlers"
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
}

// NewGitlabWebhook returns a new webhook controller
func NewGitlabWebhook(logger *slog.Logger, cfg config.Webhook, clients clients.ClientMap) (*Webhook, error) {
	hook, err := glwebhooks.New(glwebhooks.Options.Secret(cfg.Secret))
	if err != nil {
		return nil, err
	}

	err = initHandlers(logger, clients, cfg.Actions)
	if err != nil {
		return nil, err
	}

	controller := &Webhook{
		logger: logger,
		hook:   hook,
	}

	return controller, nil
}

// Handle handles gitlab webhook events
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

	logger := w.logger.With("github-event-type", fmt.Sprintf("%T", payload))

	go func() {
		switch payload := payload.(type) {
		case glwebhooks.TagEventPayload:
			logger = logger.With(
				"gitlab-repository-url", payload.Repository.URL,
				"gitlab-project-name", payload.Project.Name,
				"gitlab-project-namespace", payload.Project.Namespace,
				"gitlab-username", payload.UserUsername,
			)

			handlers.Run(logger, &payload)
		default:
			w.logger.Warn("missing handler for webhook event", "event-type", payload)
		}
	}()

	response.WriteHeader(http.StatusOK)
}
