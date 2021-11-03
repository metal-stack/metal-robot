package github

import (
	"errors"
	"net/http"

	ghwebhooks "github.com/go-playground/webhooks/v6/github"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/github/actions"
	"go.uber.org/zap"
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
	logger *zap.SugaredLogger
	cs     clients.ClientMap
	hook   *ghwebhooks.Webhook
	a      *actions.WebhookActions
}

// NewGithubWebhook returns a new webhook controller
func NewGithubWebhook(logger *zap.SugaredLogger, w config.Webhook, cs clients.ClientMap) (*Webhook, error) {
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
			w.logger.Warnw("received unregistered github event", "error", err)
			response.WriteHeader(http.StatusOK)
		} else {
			w.logger.Errorw("received malformed github event", "error", err)
			response.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	switch payload := payload.(type) {
	case ghwebhooks.ReleasePayload:
		w.logger.Debugw("received release event")
		go w.a.ProcessReleaseEvent(&payload)
	case ghwebhooks.PullRequestPayload:
		w.logger.Debugw("received pull request event")
		go w.a.ProcessPullRequestEvent(&payload)
	case ghwebhooks.PushPayload:
		w.logger.Debugw("received push event")
		go w.a.ProcessPushEvent(&payload)
	case ghwebhooks.IssuesPayload:
		w.logger.Debugw("received issues event")
	case ghwebhooks.IssueCommentPayload:
		w.logger.Debugw("received issue comment event")
		go w.a.ProcessIssueCommentEvent(&payload)
	case ghwebhooks.RepositoryPayload:
		w.logger.Debugw("received repository event")
		go w.a.ProcessRepositoryEvent(&payload)
	default:
		w.logger.Warnw("missing handler", "payload", payload)
	}

	response.WriteHeader(http.StatusOK)
}
