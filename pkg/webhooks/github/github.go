package github

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/go-github/v74/github"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
)

type Webhook struct {
	logger *slog.Logger
	cs     clients.ClientMap
	a      *WebhookActions
	secret string
}

// NewGithubWebhook returns a new webhook controller
func NewGithubWebhook(logger *slog.Logger, w config.Webhook, cs clients.ClientMap) (*Webhook, error) {
	a, err := initHandlers(logger, cs, w.Actions)
	if err != nil {
		return nil, err
	}

	controller := &Webhook{
		logger: logger,
		cs:     cs,
		secret: w.Secret,
		a:      a,
	}

	return controller, nil
}

// Handle handles github webhook events
func (w *Webhook) Handle(response http.ResponseWriter, request *http.Request) {
	payload, err := github.ValidatePayload(request, []byte(w.secret))
	if err != nil {
		w.logger.Error("received invalid github event", "error", err)
		response.WriteHeader(http.StatusInternalServerError)
	}

	event, err := github.ParseWebHook(github.WebHookType(request), payload)
	if err != nil {
		w.logger.Error("received unrecognized github event type", "error", err)
		response.WriteHeader(http.StatusInternalServerError)
	}

	ctx := context.Background()
	switch event := event.(type) {
	case *github.ReleaseEvent:
		w.logger.Debug("received release event")
		// nolint:contextcheck
		go w.a.ProcessReleaseEvent(ctx, event)
	case *github.PullRequestEvent:
		w.logger.Debug("received pull request event")
		// nolint:contextcheck
		go w.a.ProcessPullRequestEvent(ctx, event)
	case *github.PushEvent:
		w.logger.Debug("received push event")
		// nolint:contextcheck
		go w.a.ProcessPushEvent(ctx, event)
	case *github.IssuesEvent:
		w.logger.Debug("received issues event")
		// nolint:contextcheck
		go w.a.ProcessIssuesEvent(ctx, event)
	case *github.IssueCommentEvent:
		w.logger.Debug("received issue comment event")
		// nolint:contextcheck
		go w.a.ProcessIssueCommentEvent(ctx, event)
	case *github.RepositoryEvent:
		w.logger.Debug("received repository event")
		// nolint:contextcheck
		go w.a.ProcessRepositoryEvent(ctx, event)
	case *github.ProjectV2ItemEvent:
		w.logger.Debug("received project v2 item event")
		// nolint:contextcheck
		go w.a.ProcessProjectV2ItemEvent(ctx, event)
	default:
		w.logger.Warn("missing handler for webhook event", "event-type", github.WebHookType(request))
	}

	response.WriteHeader(http.StatusOK)
}
