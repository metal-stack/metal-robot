package actions

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/constants"
	"golang.org/x/sync/errgroup"

	ghwebhooks "github.com/go-playground/webhooks/v6/github"
)

const (
	ActionAggregateReleases           string = "aggregate-releases"
	ActionYAMLTranslateReleases       string = "yaml-translate-releases"
	ActionDocsPreviewComment          string = "docs-preview-comment"
	ActionCreateRepositoryMaintainers string = "create-repository-maintainers"
	ActionDistributeReleases          string = "distribute-releases"
	ActionReleaseDraft                string = "release-draft"
	ActionIssueCommentsHandler        string = "issue-handling"
	ActionProjectItemAddHandler       string = "add-items-to-project"
)

type WebhookActions struct {
	logger *slog.Logger
	rm     []*repositoryMaintainers
	dp     []*docsPreviewComment
	ar     []*AggregateReleases
	dr     []*distributeReleases
	rd     []*releaseDrafter
	pa     []*projectItemAdd
	ih     []*IssueCommentsAction
	yr     []*yamlTranslateReleases
}

func InitActions(logger *slog.Logger, cs clients.ClientMap, config config.WebhookActions) (*WebhookActions, error) {
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
		case ActionDocsPreviewComment:
			h, err := newDocsPreviewComment(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.dp = append(actions.dp, h)
		case ActionAggregateReleases:
			h, err := NewAggregateReleases(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.ar = append(actions.ar, h)
		case ActionDistributeReleases:
			h, err := newDistributeReleases(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.dr = append(actions.dr, h)
		case ActionReleaseDraft:
			h, err := newReleaseDrafter(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.rd = append(actions.rd, h)
		case ActionYAMLTranslateReleases:
			h, err := newYAMLTranslateReleases(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.yr = append(actions.yr, h)
		case ActionProjectItemAddHandler:
			h, err := newProjectItemAdd(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.pa = append(actions.pa, h)
		case ActionIssueCommentsHandler:
			h, err := newIssueCommentsAction(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.ih = append(actions.ih, h)
		default:
			return nil, fmt.Errorf("handler type not supported: %s", t)
		}

		logger.Debug("initialized github webhook action", "name", spec.Type)
	}

	return &actions, nil
}

func (w *WebhookActions) ProcessReleaseEvent(ctx context.Context, payload *ghwebhooks.ReleasePayload) {
	ctx, cancel := context.WithTimeout(ctx, constants.WebhookHandleTimeout)
	defer cancel()
	g, _ := errgroup.WithContext(ctx)

	for _, a := range w.ar {
		g.Go(func() error {
			if payload.Action != "released" {
				return nil
			}
			params := &AggregateReleaseParams{
				RepositoryName: payload.Repository.Name,
				RepositoryURL:  payload.Repository.HTMLURL,
				TagName:        payload.Release.TagName,
				Sender:         payload.Sender.Login,
			}
			err := a.AggregateRelease(ctx, params)
			if err != nil {
				w.logger.Error("error in aggregate release action", "source-repo", params.RepositoryName, "target-repo", a.repoName, "tag", params.TagName, "error", err)
				return err
			}

			return nil
		})
	}

	for _, a := range w.rd {
		g.Go(func() error {
			if payload.Action != "released" {
				return nil
			}
			params := &releaseDrafterParams{
				RepositoryName:       payload.Repository.Name,
				TagName:              payload.Release.TagName,
				ComponentReleaseInfo: payload.Release.Body,
				ReleaseURL:           payload.Release.HTMLURL,
			}
			err := a.draft(ctx, params)
			if err != nil {
				w.logger.Error("error creating release draft", "repo", a.repoName, "tag", params.TagName, "error", err)
				return err
			}

			return nil
		})
	}

	for _, a := range w.yr {
		g.Go(func() error {
			if payload.Action != "released" {
				return nil
			}
			params := &yamlTranslateReleaseParams{
				RepositoryName: payload.Repository.Name,
				RepositoryURL:  payload.Repository.CloneURL,
				TagName:        payload.Release.TagName,
			}
			err := a.translateRelease(ctx, params)
			if err != nil {
				w.logger.Error("error creating translating release", "repo", a.repoName, "tag", params.TagName, "error", err)
				return err
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		w.logger.Error("errors processing event", "error", err)
	}
}

func (w *WebhookActions) ProcessPullRequestEvent(ctx context.Context, payload *ghwebhooks.PullRequestPayload) {
	ctx, cancel := context.WithTimeout(ctx, constants.WebhookHandleTimeout)
	defer cancel()
	g, _ := errgroup.WithContext(ctx)

	for _, a := range w.dp {
		g.Go(func() error {
			if payload.Action != "opened" || payload.Repository.Name != "docs" {
				return nil
			}
			params := &docsPreviewCommentParams{
				PullRequestNumber: int(payload.PullRequest.Number),
			}

			err := a.AddDocsPreviewComment(ctx, params)
			if err != nil {
				w.logger.Error("error adding docs preview comment to docs", "repo", payload.Repository.Name, "pull_request", params.PullRequestNumber, "error", err)
				return err
			}

			return nil
		})
	}

	for _, a := range w.rd {
		g.Go(func() error {
			if payload.Action != "closed" {
				return nil
			}
			if payload.Repository.Private {
				return nil
			}
			if !payload.PullRequest.Merged {
				return nil
			}

			params := &releaseDrafterParams{
				RepositoryName:       payload.Repository.Name,
				ComponentReleaseInfo: &payload.PullRequest.Body,
			}
			err := a.appendMergedPR(ctx, payload.PullRequest.Title, payload.PullRequest.Number, payload.PullRequest.User.Login, params)
			if err != nil {
				w.logger.Error("error append merged PR to release draft", "repo", a.repoName, "pr", payload.PullRequest.Title, "error", err)
				return err
			}

			return nil
		})
	}

	for _, i := range w.pa {
		g.Go(func() error {
			if payload.Action != "opened" {
				return nil
			}

			params := &projectItemAddParams{
				RepositoryName: payload.Repository.Name,
				RepositoryURL:  payload.Repository.CloneURL,
				NodeID:         payload.PullRequest.NodeID,
				ID:             payload.PullRequest.ID,
				URL:            payload.PullRequest.URL,
			}

			err := i.Handle(ctx, params)
			if err != nil {
				w.logger.Error("error in project item add handler action", "source-repo", params.RepositoryName, "error", err)
				return err
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		w.logger.Error("errors processing event", "error", err)
	}
}

func (w *WebhookActions) ProcessPushEvent(ctx context.Context, payload *ghwebhooks.PushPayload) {
	ctx, cancel := context.WithTimeout(ctx, constants.WebhookHandleTimeout)
	defer cancel()
	g, _ := errgroup.WithContext(ctx)

	for _, a := range w.ar {
		g.Go(func() error {
			if !payload.Created || !strings.HasPrefix(payload.Ref, "refs/tags/v") {
				return nil
			}
			params := &AggregateReleaseParams{
				RepositoryName: payload.Repository.Name,
				RepositoryURL:  payload.Repository.HTMLURL,
				TagName:        extractTag(payload),
				Sender:         payload.Sender.Login,
			}

			err := a.AggregateRelease(ctx, params)
			if err != nil {
				w.logger.Error("error in aggregate release action", "source-repo", params.RepositoryName, "target-repo", a.repoName, "tag", params.TagName, "error", err)
				return err
			}

			return nil
		})
	}

	for _, a := range w.dr {
		g.Go(func() error {
			if !payload.Created || !strings.HasPrefix(payload.Ref, "refs/tags/v") {
				return nil
			}

			params := &distributeReleaseParams{
				RepositoryName: payload.Repository.Name,
				TagName:        extractTag(payload),
			}

			err := a.DistributeRelease(ctx, params)
			if err != nil {
				w.logger.Error("error in distribute release action", "source-repo", params.RepositoryName, "tag", params.TagName, "error", err)
				return err
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		w.logger.Error("errors processing event", "error", err)
	}
}

func (w *WebhookActions) ProcessRepositoryEvent(ctx context.Context, payload *ghwebhooks.RepositoryPayload) {
	ctx, cancel := context.WithTimeout(ctx, constants.WebhookHandleTimeout)
	defer cancel()
	g, _ := errgroup.WithContext(ctx)

	for _, a := range w.rm {
		g.Go(func() error {
			if payload.Action != "created" {
				return nil
			}

			params := &repositoryMaintainersParams{
				RepositoryName: payload.Repository.Name,
				Creator:        payload.Sender.Login,
			}
			err := a.CreateRepositoryMaintainers(ctx, params)
			if err != nil {
				w.logger.Error("error creating repository maintainers team", "repo", params.RepositoryName, "error", err)
				return err
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		w.logger.Error("errors processing event", "error", err)
	}
}

func (w *WebhookActions) ProcessIssuesEvent(ctx context.Context, payload *ghwebhooks.IssuesPayload) {
	ctx, cancel := context.WithTimeout(ctx, constants.WebhookHandleTimeout)
	defer cancel()
	g, _ := errgroup.WithContext(ctx)

	for _, i := range w.pa {
		g.Go(func() error {
			if payload.Action != "opened" {
				return nil
			}

			params := &projectItemAddParams{
				RepositoryName: payload.Repository.Name,
				RepositoryURL:  payload.Repository.CloneURL,
				NodeID:         payload.Issue.NodeID,
				ID:             payload.Issue.ID,
				URL:            payload.Issue.URL,
			}

			err := i.Handle(ctx, params)
			if err != nil {
				w.logger.Error("error in project item add handler action", "source-repo", params.RepositoryName, "error", err)
				return err
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		w.logger.Error("errors processing event", "error", err)
	}
}

func (w *WebhookActions) ProcessIssueCommentEvent(ctx context.Context, payload *ghwebhooks.IssueCommentPayload) {
	ctx, cancel := context.WithTimeout(ctx, constants.WebhookHandleTimeout)
	defer cancel()
	g, _ := errgroup.WithContext(ctx)

	for _, i := range w.ih {
		g.Go(func() error {
			if payload.Action != "created" {
				return nil
			}
			if payload.Issue.PullRequest == nil {
				return nil
			}

			parts := strings.Split(payload.Issue.PullRequest.URL, "/")
			pullRequestNumberString := parts[len(parts)-1]
			pullRequestNumber, err := strconv.ParseInt(pullRequestNumberString, 10, 64)
			if err != nil {
				return err
			}

			params := &IssueCommentsActionParams{
				RepositoryName:    payload.Repository.Name,
				RepositoryURL:     payload.Repository.CloneURL,
				Comment:           payload.Comment.Body,
				CommentID:         payload.Comment.ID,
				User:              payload.Comment.User.Login,
				PullRequestNumber: int(pullRequestNumber),
			}

			err = i.HandleIssueComment(ctx, params)
			if err != nil {
				w.logger.Error("error in issue comment handler action", "source-repo", params.RepositoryName, "error", err)
				return err
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		w.logger.Error("errors processing event", "error", err)
	}
}

func extractTag(payload *ghwebhooks.PushPayload) string {
	return strings.Replace(payload.Ref, "refs/tags/", "", 1)
}
