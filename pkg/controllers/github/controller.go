package github

import (
	"context"
	"net/http"

	"github.com/metal-stack/metal-robot/pkg/controllers/github/webhooks"
	"go.uber.org/zap"
	ghwebhooks "gopkg.in/go-playground/webhooks.v5/github"
)

// Controller that retrieves and handles github webhook events
type Controller struct {
	logger    *zap.SugaredLogger
	auth      *Auth
	hook      *ghwebhooks.Webhook
	events    []ghwebhooks.Event
	installID int64
}

// NewController returns a new webhook controller
func NewController(logger *zap.SugaredLogger, auth *Auth, webhookSecret string) (*Controller, error) {
	hook, err := ghwebhooks.New(ghwebhooks.Options.Secret(webhookSecret))
	if err != nil {
		return nil, err
	}

	installation, _, err := auth.GetV3AppClient().Apps.FindOrganizationInstallation(context.TODO(), organisation)
	if err != nil {
		return nil, err
	}

	var events []ghwebhooks.Event
	for _, e := range installation.Events {
		events = append(events, ghwebhooks.Event(e))
	}

	logger.Infow("figured out installation id", "id", installation.GetID(), "listen-events", events)

	controller := &Controller{
		logger:    logger,
		auth:      auth,
		hook:      hook,
		events:    events,
		installID: installation.GetID(),
	}
	return controller, nil
}

// Webhook handles a webhook event
func (c *Controller) Webhook(response http.ResponseWriter, request *http.Request) {
	payload, err := c.hook.Parse(request, c.events...)
	if err != nil {
		if err == ghwebhooks.ErrEventNotFound {
			c.logger.Warnw("received unexpected github event", "error", err)
			response.WriteHeader(http.StatusOK)
		} else {
			c.logger.Errorw("unable to handle github event", "error", err)
			response.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	switch payload := payload.(type) {
	case ghwebhooks.ReleasePayload:
		c.logger.Debugw("received release event")
		p := &webhooks.ReleaseProcessor{
			Logger:    c.logger.Named("releases-webhook"),
			Payload:   &payload,
			Client:    c.auth.GetV3Client(),
			AppClient: c.auth.GetV3AppClient(),
			InstallID: c.installID,
		}
		err = webhooks.ProcessReleaseEvent(p)
		if err != nil {
			c.logger.Errorw("error processing release event", "error", err)
			response.WriteHeader(http.StatusInternalServerError)
			_, err = response.Write([]byte(err.Error()))
			if err != nil {
				c.logger.Errorw("could not write error to http response", "error", err)
			}
			return
		}
	case ghwebhooks.PullRequestPayload:
		c.logger.Debugw("received pull request event")
	case ghwebhooks.PushPayload:
		c.logger.Debugw("received push event")
		p := &webhooks.PushProcessor{
			Logger:    c.logger.Named("releases-webhook"),
			Payload:   &payload,
			Client:    c.auth.GetV3Client(),
			InstallID: c.installID,
		}
		err = webhooks.ProcessPushEvent(p)
		if err != nil {
			c.logger.Errorw("error processing push event", "error", err)
			response.WriteHeader(http.StatusInternalServerError)
			_, err = response.Write([]byte(err.Error()))
			if err != nil {
				c.logger.Errorw("could not write error to http response", "error", err)
			}
			return
		}
	default:
		c.logger.Warnw("missing handler", "payload", payload)
	}

	response.WriteHeader(http.StatusOK)
}
