package actions

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/constants"
	ghactions "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions"
	aggregate_releases "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/aggregate-releases"
	"golang.org/x/sync/errgroup"

	glwebhooks "github.com/go-playground/webhooks/v6/gitlab"
)

type WebhookActions struct {
	logger *slog.Logger
	ar     []ghactions.WebhookHandler[*aggregate_releases.Params]
}

func InitActions(logger *slog.Logger, cs clients.ClientMap, cfg config.WebhookActions) (*WebhookActions, error) {
	actions := WebhookActions{
		logger: logger,
	}

	for _, spec := range cfg {
		c, ok := cs[spec.Client]
		if !ok {
			return nil, fmt.Errorf("webhook action client not found: %s", spec.Client)
		}

		switch t := spec.Type; t {
		case config.ActionAggregateReleases:
			typedClient, ok := c.(*clients.Github)
			if !ok {
				return nil, fmt.Errorf("action %s only supports github clients", spec.Type)
			}
			h, err := aggregate_releases.New(typedClient, spec.Args)
			if err != nil {
				return nil, err
			}
			actions.ar = append(actions.ar, h)
		default:
			return nil, fmt.Errorf("handler type not supported: %s", t)
		}

		logger.Debug("initialized github webhook action", "name", config.ActionAggregateReleases)
	}

	return &actions, nil
}

func (w *WebhookActions) ProcessTagEvent(ctx context.Context, payload *glwebhooks.TagEventPayload) {
	ctx, cancel := context.WithTimeout(ctx, constants.WebhookHandleTimeout)
	defer cancel()
	g, _ := errgroup.WithContext(ctx)

	for _, a := range w.ar {
		a := a
		g.Go(func() error {
			err := a.Handle(ctx, w.logger, &aggregate_releases.Params{
				RepositoryName: payload.Repository.Name,
				RepositoryURL:  payload.Repository.URL,
				TagName:        extractTag(payload),
				Sender:         payload.UserUsername,
			})
			if err != nil {
				return err
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		w.logger.Error("errors processing event", "error", err)
	}
}

func extractTag(payload *glwebhooks.TagEventPayload) string {
	return strings.Replace(payload.Ref, "refs/tags/", "", 1)
}
