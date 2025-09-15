package github

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/google/go-github/v74/github"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/github/actions"
)

type Webhook struct {
	logger *slog.Logger
	secret string
}

// NewGithubWebhook returns a new webhook controller
func NewGithubWebhook(logger *slog.Logger, config config.Webhook, clients clients.ClientMap) (*Webhook, error) {
	err := initHandlers(logger, clients, config.Actions)
	if err != nil {
		return nil, err
	}

	controller := &Webhook{
		logger: logger,
		secret: config.Secret,
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

	logger := w.logger.With("github-event-type", fmt.Sprintf("%T", event))

	// as we need to fulfill the time constraint for webhooks, we run all actions async
	go func() {
		switch event := event.(type) {
		case *github.ReleaseEvent:
			logger = logger.With(
				"github-event-action", pointer.SafeDeref(event.Action),
				"github-user", pointer.SafeDeref(event.Sender.Login),
				"github-organization-name", pointer.SafeDeref(event.Org.Name),
				"github-repository-url", pointer.SafeDeref(event.Repo.URL),
				"github-release-name", pointer.SafeDeref(event.Release.Name),
			)

			actions.Run(logger, event)

		case *github.PullRequestEvent:
			logger = logger.With(
				"github-event-action", pointer.SafeDeref(event.Action),
				"github-user", pointer.SafeDeref(event.Sender.Login),
				"github-organization-name", pointer.SafeDeref(event.Organization.Name),
				"github-repository-url", pointer.SafeDeref(event.Repo.URL),
				"github-pull-request-url", pointer.SafeDeref(event.PullRequest.URL),
			)

			actions.Run(logger, event)

		case *github.PushEvent:
			logger = logger.With(
				"github-event-action", pointer.SafeDeref(event.Action),
				"github-user", pointer.SafeDeref(event.Sender.Login),
				"github-organization-name", pointer.SafeDeref(event.Organization.Name),
				"github-repository-url", pointer.SafeDeref(event.Repo.URL),
				"github-ref", pointer.SafeDeref(event.Ref),
			)

			actions.Run(logger, event)

		case *github.IssuesEvent:
			logger = logger.With(
				"github-event-action", pointer.SafeDeref(event.Action),
				"github-user", pointer.SafeDeref(event.Sender.Login),
				"github-organization-name", pointer.SafeDeref(event.Org.Name),
				"github-repository-url", pointer.SafeDeref(event.Repo.URL),
				"github-issue-number", pointer.SafeDeref(event.Issue.Number),
			)

			actions.Run(logger, event)

		case *github.IssueCommentEvent:
			logger = logger.With(
				"github-event-action", pointer.SafeDeref(event.Action),
				"github-user", pointer.SafeDeref(event.Sender.Login),
				"github-organization-name", pointer.SafeDeref(event.Organization.Name),
				"github-repository-url", pointer.SafeDeref(event.Repo.URL),
				"github-issue-number", pointer.SafeDeref(event.Issue.Number),
			)

			actions.Run(logger, event)

		case *github.RepositoryEvent:
			logger = logger.With(
				"github-event-action", pointer.SafeDeref(event.Action),
				"github-user", pointer.SafeDeref(event.Sender.Login),
				"github-organization-name", pointer.SafeDeref(event.Org.Name),
				"github-repository-url", pointer.SafeDeref(event.Repo.URL),
			)

			actions.Run(logger, event)

		case *github.ProjectV2ItemEvent:
			logger = logger.With(
				"github-event-action", pointer.SafeDeref(event.Action),
				"github-user", pointer.SafeDeref(event.Sender.Login),
				"github-organization-name", pointer.SafeDeref(event.Org.Name),
				"github-v2-item-content-type", pointer.SafeDeref(event.ProjectV2Item.ContentType),
			)

			actions.Run(logger, event)

		default:
			w.logger.Warn("missing handler for webhook event", "event-type", github.WebHookType(request))
		}
	}()

	response.WriteHeader(http.StatusOK)
}
