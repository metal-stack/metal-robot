package actions

import (
	"context"
	"fmt"
	"strings"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/constants"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	ghwebhooks "gopkg.in/go-playground/webhooks.v5/github"
)

const (
	ActionAddToReleaseVector          string = "release-vector"
	ActionDocsPreviewComment          string = "docs-preview-comment"
	ActionCreateRepositoryMaintainers string = "create-repository-maintainers"
	ActionUpdateSwaggerClients        string = "swagger-clients"
)

type WebhookActions struct {
	logger *zap.SugaredLogger
	rm     []*repositoryMaintainers
	dp     []*docsPreviewComment
	rv     []*ReleaseVector
	sc     []*swaggerClient
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

		switch clientType := c.(type) {
		case *clients.Github:
		default:
			return nil, fmt.Errorf("action %s only supports github clients, not: %s", spec.Type, clientType)
		}

		switch t := spec.Type; t {
		case ActionCreateRepositoryMaintainers:
			h, err := newCreateRepositoryMaintainers(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.rm = append(actions.rm, h)
			logger.Debugw("initialized github webhook action", "name", ActionCreateRepositoryMaintainers)
		case ActionDocsPreviewComment:
			h, err := newDocsPreviewComment(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.dp = append(actions.dp, h)
			logger.Debugw("initialized github webhook action", "name", ActionDocsPreviewComment)
		case ActionAddToReleaseVector:
			h, err := NewReleaseVector(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.rv = append(actions.rv, h)
			logger.Debugw("initialized github webhook action", "name", ActionAddToReleaseVector)
		case ActionUpdateSwaggerClients:
			h, err := newSwaggerClient(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.sc = append(actions.sc, h)
			logger.Debugw("initialized github webhook action", "name", ActionUpdateSwaggerClients)
		default:
			return nil, fmt.Errorf("handler type not supported: %s", t)
		}
	}

	return &actions, nil
}

func (w *WebhookActions) ProcessReleaseEvent(payload *ghwebhooks.ReleasePayload) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.WebhookHandleTimeout)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	for _, a := range w.rv {
		a := a
		g.Go(func() error {
			if payload.Action != "released" {
				return nil
			}
			p := &ReleaseVectorParams{
				RepositoryName: payload.Repository.Name,
				TagName:        payload.Release.TagName,
			}
			err := a.AddToRelaseVector(ctx, p)
			if err != nil {
				w.logger.Errorw("error adding release to release vector", "release-repo", a.repoURL, "repo", p.RepositoryName, "tag", p.TagName, "error", err)
				return err
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		w.logger.Errorw("errors processing event", "error", err)
	}
}

func (w *WebhookActions) ProcessPullRequestEvent(payload *ghwebhooks.PullRequestPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.WebhookHandleTimeout)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	for _, a := range w.dp {
		a := a
		g.Go(func() error {
			if payload.Action != "opened" || payload.Repository.Name != "docs" {
				return nil
			}
			p := &docsPreviewCommentParams{
				PullRequestNumber: int(payload.PullRequest.Number),
			}
			err := a.AddDocsPreviewComment(ctx, p)
			if err != nil {
				w.logger.Errorw("error adding docs preview comment to docs", "repo", payload.Repository.Name, "pull_request", p.PullRequestNumber, "error", err)
				return err
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		w.logger.Errorw("errors processing event", "error", err)
	}
}

func (w *WebhookActions) ProcessPushEvent(payload *ghwebhooks.PushPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.WebhookHandleTimeout)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	for _, a := range w.rv {
		a := a
		g.Go(func() error {
			if !payload.Created || !strings.HasPrefix(payload.Ref, "refs/tags/v") {
				return nil
			}
			releaseParams := &ReleaseVectorParams{
				RepositoryName: payload.Repository.Name,
				TagName:        extractTag(payload),
			}

			err := a.AddToRelaseVector(ctx, releaseParams)
			if err != nil {
				w.logger.Errorw("error adding new tag to release vector", "release-repo", a.repoURL, "repo", releaseParams.RepositoryName, "tag", releaseParams.TagName, "error", err)
				return err
			}

			return nil
		})
	}

	for _, a := range w.sc {
		a := a
		g.Go(func() error {
			if !payload.Created || !strings.HasPrefix(payload.Ref, "refs/tags/v") {
				return nil
			}

			swaggerParams := &swaggerParams{
				RepositoryName: payload.Repository.Name,
				TagName:        extractTag(payload),
			}

			err := a.GenerateSwaggerClients(ctx, swaggerParams)
			if err != nil {
				w.logger.Errorw("error creating branches for swagger client repositories", "repo", swaggerParams.RepositoryName, "tag", swaggerParams.TagName, "error", err)
				return err
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		w.logger.Errorw("errors processing event", "error", err)
	}
}

func (w *WebhookActions) ProcessRepositoryEvent(payload *ghwebhooks.RepositoryPayload) {
	ctx, cancel := context.WithTimeout(context.Background(), constants.WebhookHandleTimeout)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	for _, a := range w.rm {
		a := a
		g.Go(func() error {
			if payload.Action != "created" {
				return nil
			}

			p := &repositoryMaintainersParams{
				RepositoryName: payload.Repository.Name,
				Creator:        payload.Sender.Login,
			}
			err := a.CreateRepositoryMaintainers(ctx, p)
			if err != nil {
				w.logger.Errorw("error creating repository maintainers team", "repo", p.RepositoryName, "error", err)
				return err
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		w.logger.Errorw("errors processing event", "error", err)
	}
}

func extractTag(payload *ghwebhooks.PushPayload) string {
	return strings.Replace(payload.Ref, "refs/tags/", "", 1)
}
