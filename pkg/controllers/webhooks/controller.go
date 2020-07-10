package webhooks

import (
	"context"

	"github.com/metal-stack/metal-robot/pkg/controllers"
	"github.com/metal-stack/metal-robot/pkg/controllers/webhooks/github"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	ghwebhooks "gopkg.in/go-playground/webhooks.v5/github"
	glwebhooks "gopkg.in/go-playground/webhooks.v5/gitlab"
)

type gh struct {
	auth      *github.Auth
	hook      *ghwebhooks.Webhook
	events    []ghwebhooks.Event
	installID int64
}

type gl struct {
	hook   *glwebhooks.Webhook
	events []glwebhooks.Event
}

// Controller that retrieves and handles github webhook events
type Controller struct {
	logger *zap.SugaredLogger
	gh     *gh
	gl     *gl
}

// NewController returns a new webhook controller
func NewController(logger *zap.SugaredLogger, ghAuth *github.Auth, ghWebhookSecret string) (*Controller, error) {
	gh, err := initGithub(ghAuth, ghWebhookSecret)
	if err != nil {
		return nil, errors.Wrap(err, "error initializing Github")
	}

	logger.Infow("figured out github installation id", "id", gh.installID, "listen-events", gh.events)

	controller := &Controller{
		logger: logger,
		gh:     gh,
	}
	return controller, nil
}

func initGithub(auth *github.Auth, webhookSecret string) (*gh, error) {
	// set webhook secret
	hook, err := ghwebhooks.New(ghwebhooks.Options.Secret(webhookSecret))
	if err != nil {
		return nil, err
	}

	installation, _, err := auth.GetV3AppClient().Apps.FindOrganizationInstallation(context.TODO(), controllers.GithubOrganisation)
	if err != nil {
		return nil, err
	}

	var events []ghwebhooks.Event
	for _, e := range installation.Events {
		events = append(events, ghwebhooks.Event(e))
	}

	return &gh{
		auth:      auth,
		hook:      hook,
		events:    events,
		installID: installation.GetID(),
	}, nil
}
