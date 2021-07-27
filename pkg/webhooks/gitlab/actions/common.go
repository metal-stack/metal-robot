package actions

import (
	"context"
	"fmt"
	"strings"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/constants"
	ghactions "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	glwebhooks "github.com/go-playground/webhooks/v6/gitlab"
)

type WebhookActions struct {
	logger *zap.SugaredLogger
	ar     []*ghactions.AggregateReleases
}

func InitActions(logger *zap.SugaredLogger, cs clients.ClientMap, config config.WebhookActions) (*WebhookActions, error) {
	actions := WebhookActions{
		logger: logger,
	}

	for _, spec := range config {
		c, ok := cs[spec.Client]
		if !ok {
			return nil, fmt.Errorf("webhook action client not found: %s", spec.Client)
		}

		switch t := spec.Type; t {
		case ghactions.ActionAggregateReleases:
			typedClient, ok := c.(*clients.Github)
			if !ok {
				return nil, fmt.Errorf("action %s only supports github clients", spec.Type)
			}
			h, err := ghactions.NewAggregateReleases(logger, typedClient, spec.Args)
			if err != nil {
				return nil, err
			}
			actions.ar = append(actions.ar, h)
		default:
			return nil, fmt.Errorf("handler type not supported: %s", t)
		}

		logger.Debugw("initialized github webhook action", "name", ghactions.ActionAggregateReleases)
	}

	return &actions, nil
}

func (w *WebhookActions) ProcessTagEvent(payload *glwebhooks.TagEventPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.WebhookHandleTimeout)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	for _, a := range w.ar {
		a := a
		g.Go(func() error {
			params := &ghactions.AggregateReleaseParams{
				RepositoryName: payload.Repository.Name,
				TagName:        extractTag(payload),
			}
			err := a.AggregateRelease(ctx, params)
			if err != nil {
				w.logger.Errorw("error in aggregate release action", "release-repo", params, "repo", params.RepositoryName, "tag", params.TagName, "error", err)
				w.logger.Errorw("error adding release to release vector", "repo", params.RepositoryName, "tag", extractTag(payload), "error", err)
				return err
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		w.logger.Errorw("errors processing event", "error", err)
	}
}

func extractTag(payload *glwebhooks.TagEventPayload) string {
	return strings.Replace(payload.Ref, "refs/tags/", "", 1)
}
