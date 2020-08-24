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

	glwebhooks "gopkg.in/go-playground/webhooks.v5/gitlab"
)

type WebhookActions struct {
	logger *zap.SugaredLogger
	rv     []*ghactions.ReleaseVector
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
		case ghactions.ActionAddToReleaseVector:
			typedClient, ok := c.(*clients.Github)
			if !ok {
				return nil, fmt.Errorf("action %s only supports github clients", spec.Type)
			}
			h, err := ghactions.NewReleaseVector(logger, typedClient, spec.Args)
			if err != nil {
				return nil, err
			}
			actions.rv = append(actions.rv, h)
			logger.Debugw("initialized github webhook action", "name", ghactions.ActionAddToReleaseVector)
		default:
			return nil, fmt.Errorf("handler type not supported: %s", t)
		}
	}

	return &actions, nil
}

func (w *WebhookActions) ProcessTagEvent(payload *glwebhooks.TagEventPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.WebhookHandleTimeout)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	for _, a := range w.rv {
		a := a
		g.Go(func() error {
			p := &ghactions.ReleaseVectorParams{
				RepositoryName: payload.Repository.Name,
				TagName:        extractTag(payload),
			}
			err := a.AddToRelaseVector(ctx, p)
			if err != nil {
				w.logger.Errorw("error adding release to release vector", "repo", p.RepositoryName, "tag", extractTag(payload), "error", err)
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
