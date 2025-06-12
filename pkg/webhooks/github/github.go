package github

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	ghwebhooks "github.com/go-playground/webhooks/v6/github"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/github/actions"
)

var listenEvents = []ghwebhooks.Event{
	ghwebhooks.ReleaseEvent,
	ghwebhooks.PullRequestEvent,
	ghwebhooks.PushEvent,
	ghwebhooks.IssuesEvent,
	ghwebhooks.IssueCommentEvent,
	ghwebhooks.RepositoryEvent,
}

type Webhook struct {
	logger *slog.Logger
	cs     clients.ClientMap
	hook   *ghwebhooks.Webhook
	a      *actions.WebhookActions
}

// NewGithubWebhook returns a new webhook controller
func NewGithubWebhook(logger *slog.Logger, w config.Webhook, cs clients.ClientMap) (*Webhook, error) {
	hook, err := ghwebhooks.New(ghwebhooks.Options.Secret(w.Secret))
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

// Handle handles github webhook events
func (w *Webhook) Handle(response http.ResponseWriter, request *http.Request) {
	payload, err := w.hook.Parse(request, listenEvents...)
	if err != nil {
		if errors.Is(err, ghwebhooks.ErrEventNotFound) {
			w.logger.Warn("received unregistered github event", "error", err)
			response.WriteHeader(http.StatusOK)
		} else {
			w.logger.Error("received malformed github event", "error", err)
			response.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	ctx := context.Background()
	switch payload := payload.(type) {
	case ghwebhooks.ReleasePayload:
		w.logger.Debug("received release event")
		// nolint:contextcheck
		go w.a.ProcessReleaseEvent(ctx, &payload)
	case ghwebhooks.PullRequestPayload:
		w.logger.Debug("received pull request event")
		// nolint:contextcheck
		go w.a.ProcessPullRequestEvent(ctx, &payload)
	case ghwebhooks.PushPayload:
		w.logger.Debug("received push event")
		// nolint:contextcheck
		go w.a.ProcessPushEvent(ctx, &payload)
	case ghwebhooks.IssuesPayload:
		w.logger.Debug("received issues event")
		// nolint:contextcheck
		go w.a.ProcessIssuesEvent(ctx, &payload)
	case ghwebhooks.IssueCommentPayload:
		w.logger.Debug("received issue comment event")
		// nolint:contextcheck
		go w.a.ProcessIssueCommentEvent(ctx, &payload)
	case ghwebhooks.RepositoryPayload:
		w.logger.Debug("received repository event")
		// nolint:contextcheck
		go w.a.ProcessRepositoryEvent(ctx, &payload)
	default:
		w.logger.Warn("missing handler", "payload", payload)
	}

	response.WriteHeader(http.StatusOK)
}
