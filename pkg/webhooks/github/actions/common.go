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

	"github.com/metal-stack/metal-lib/pkg/pointer"

	"github.com/google/go-github/v74/github"
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
	ActionProjectV2ItemHandler        string = "project-v2-item"
)

type WebhookActions struct {
	logger *slog.Logger
	rm     []*repositoryMaintainers
	dp     []*docsPreviewComment
	ar     []*AggregateReleases
	dr     []*distributeReleases
	rd     []*releaseDrafter
	pa     []*projectItemAdd
	p2     []*projectV2ItemHandler
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
		case ActionProjectV2ItemHandler:
			h, err := newProjectV2ItemHandler(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.p2 = append(actions.p2, h)
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

func (w *WebhookActions) ProcessReleaseEvent(ctx context.Context, payload *github.ReleaseEvent) {
	ctx, cancel := context.WithTimeout(ctx, constants.WebhookHandleTimeout)
	defer cancel()
	g, _ := errgroup.WithContext(ctx)

	for _, a := range w.ar {
		g.Go(func() error {
			if pointer.SafeDeref(payload.Action) != "released" {
				return nil
			}
			params := &AggregateReleaseParams{
				RepositoryName: pointer.SafeDeref(payload.Repo.Name),
				RepositoryURL:  pointer.SafeDeref(payload.Repo.HTMLURL),
				TagName:        pointer.SafeDeref(payload.Release.TagName),
				Sender:         pointer.SafeDeref(payload.Sender.Login),
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
			if pointer.SafeDeref(payload.Action) != "released" {
				return nil
			}
			params := &releaseDrafterParams{
				RepositoryName:       pointer.SafeDeref(payload.Repo.Name),
				TagName:              pointer.SafeDeref(payload.Release.TagName),
				ComponentReleaseInfo: payload.Release.Body,
				ReleaseURL:           pointer.SafeDeref(payload.Release.HTMLURL),
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
			if pointer.SafeDeref(payload.Action) != "released" {
				return nil
			}
			params := &yamlTranslateReleaseParams{
				RepositoryName: pointer.SafeDeref(payload.Repo.Name),
				RepositoryURL:  pointer.SafeDeref(payload.Repo.CloneURL),
				TagName:        pointer.SafeDeref(payload.Release.TagName),
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

func (w *WebhookActions) ProcessPullRequestEvent(ctx context.Context, payload *github.PullRequestEvent) {
	ctx, cancel := context.WithTimeout(ctx, constants.WebhookHandleTimeout)
	defer cancel()
	g, _ := errgroup.WithContext(ctx)

	for _, a := range w.dp {
		g.Go(func() error {
			if pointer.SafeDeref(payload.Action) != "opened" || pointer.SafeDeref(payload.Repo.Name) != "docs" {
				return nil
			}
			params := &docsPreviewCommentParams{
				PullRequestNumber: int(pointer.SafeDeref(payload.PullRequest.Number)),
			}

			err := a.AddDocsPreviewComment(ctx, params)
			if err != nil {
				w.logger.Error("error adding docs preview comment to docs", "repo", *payload.Repo.Name, "pull_request", params.PullRequestNumber, "error", err)
				return err
			}

			return nil
		})
	}

	for _, a := range w.rd {
		g.Go(func() error {
			if pointer.SafeDeref(payload.Action) != "closed" {
				return nil
			}
			if pointer.SafeDeref(payload.Repo.Private) {
				return nil
			}
			if !pointer.SafeDeref(payload.PullRequest.Merged) {
				return nil
			}

			params := &releaseDrafterParams{
				RepositoryName:       pointer.SafeDeref(payload.Repo.Name),
				ComponentReleaseInfo: payload.PullRequest.Body,
			}
			err := a.appendMergedPR(ctx,
				pointer.SafeDeref(payload.PullRequest.Title),
				pointer.SafeDeref(payload.PullRequest.Number),
				pointer.SafeDeref(payload.PullRequest.User.Login),
				params,
			)
			if err != nil {
				w.logger.Error("error append merged PR to release draft", "repo", a.repoName, "pr", payload.PullRequest.Title, "error", err)
				return err
			}

			return nil
		})
	}

	for _, i := range w.pa {
		g.Go(func() error {
			if pointer.SafeDeref(payload.Action) != "opened" {
				return nil
			}

			params := &projectItemAddParams{
				RepositoryName: pointer.SafeDeref(payload.Repo.Name),
				RepositoryURL:  pointer.SafeDeref(payload.Repo.CloneURL),
				NodeID:         pointer.SafeDeref(payload.PullRequest.NodeID),
				ID:             pointer.SafeDeref(payload.PullRequest.ID),
				URL:            pointer.SafeDeref(payload.PullRequest.URL),
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

func (w *WebhookActions) ProcessPushEvent(ctx context.Context, payload *github.PushEvent) {
	ctx, cancel := context.WithTimeout(ctx, constants.WebhookHandleTimeout)
	defer cancel()
	g, _ := errgroup.WithContext(ctx)

	for _, a := range w.ar {
		g.Go(func() error {
			if !pointer.SafeDeref(payload.Created) || !strings.HasPrefix(pointer.SafeDeref(payload.Ref), "refs/tags/v") {
				return nil
			}
			params := &AggregateReleaseParams{
				RepositoryName: pointer.SafeDeref(payload.Repo.Name),
				RepositoryURL:  pointer.SafeDeref(payload.Repo.HTMLURL),
				TagName:        extractTag(payload),
				Sender:         pointer.SafeDeref(payload.Sender.Login),
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
			if !pointer.SafeDeref(payload.Created) || !strings.HasPrefix(pointer.SafeDeref(payload.Ref), "refs/tags/v") {
				return nil
			}

			params := &distributeReleaseParams{
				RepositoryName: pointer.SafeDeref(payload.Repo.Name),
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

func (w *WebhookActions) ProcessRepositoryEvent(ctx context.Context, payload *github.RepositoryEvent) {
	ctx, cancel := context.WithTimeout(ctx, constants.WebhookHandleTimeout)
	defer cancel()
	g, _ := errgroup.WithContext(ctx)

	for _, a := range w.rm {
		g.Go(func() error {
			if pointer.SafeDeref(payload.Action) != "created" {
				return nil
			}

			params := &repositoryMaintainersParams{
				RepositoryName: pointer.SafeDeref(payload.Repo.Name),
				Creator:        pointer.SafeDeref(payload.Sender.Login),
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

func (w *WebhookActions) ProcessProjectV2ItemEvent(ctx context.Context, payload *github.ProjectV2ItemEvent) {
	ctx, cancel := context.WithTimeout(ctx, constants.WebhookHandleTimeout)
	defer cancel()
	g, _ := errgroup.WithContext(ctx)

	for _, a := range w.p2 {
		g.Go(func() error {
			if pointer.SafeDeref(payload.Action) != "edited" {
				return nil
			}
			if payload.Changes == nil ||
				payload.Changes.FieldValue == nil ||
				pointer.SafeDeref(payload.Changes.FieldValue.FieldName) != "Status" ||
				payload.Changes.FieldValue.From != nil ||
				len(payload.Changes.FieldValue.To) == 0 {
				return nil
			}

			params := &projectV2ItemHandlerParams{
				ProjectNumber: pointer.SafeDeref(payload.Changes.FieldValue.ProjectNumber), // TODO use node id as in other handler
				ProjectID:     ActionProjectItemAddHandler,
				ContentNodeID: pointer.SafeDeref(payload.ProjectV2Item.ContentNodeID),
			}
			err := a.Handle(ctx, params)
			if err != nil {
				w.logger.Error("error removing labels from project v2 item", "project-number", params.ProjectNumber, "error", err)
				return err
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		w.logger.Error("errors processing event", "error", err)
	}
}

func (w *WebhookActions) ProcessIssuesEvent(ctx context.Context, payload *github.IssuesEvent) {
	ctx, cancel := context.WithTimeout(ctx, constants.WebhookHandleTimeout)
	defer cancel()
	g, _ := errgroup.WithContext(ctx)

	for _, i := range w.pa {
		g.Go(func() error {
			if pointer.SafeDeref(payload.Action) != "opened" {
				return nil
			}

			params := &projectItemAddParams{
				RepositoryName: pointer.SafeDeref(payload.Repo.Name),
				RepositoryURL:  pointer.SafeDeref(payload.Repo.CloneURL),
				NodeID:         pointer.SafeDeref(payload.Issue.NodeID),
				ID:             pointer.SafeDeref(payload.Issue.ID),
				URL:            pointer.SafeDeref(payload.Issue.URL),
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

func (w *WebhookActions) ProcessIssueCommentEvent(ctx context.Context, payload *github.IssueCommentEvent) {
	ctx, cancel := context.WithTimeout(ctx, constants.WebhookHandleTimeout)
	defer cancel()
	g, _ := errgroup.WithContext(ctx)

	for _, i := range w.ih {
		g.Go(func() error {
			if pointer.SafeDeref(payload.Action) != "created" {
				return nil
			}
			if payload.Issue.PullRequestLinks == nil {
				return nil
			}

			parts := strings.Split(*payload.Issue.PullRequestLinks.URL, "/")
			pullRequestNumberString := parts[len(parts)-1]
			pullRequestNumber, err := strconv.ParseInt(pullRequestNumberString, 10, 64)
			if err != nil {
				return err
			}

			params := &IssueCommentsActionParams{
				RepositoryName:    pointer.SafeDeref(payload.Repo.Name),
				RepositoryURL:     pointer.SafeDeref(payload.Repo.CloneURL),
				Comment:           pointer.SafeDeref(payload.Comment.Body),
				CommentID:         pointer.SafeDeref(payload.Comment.ID),
				User:              pointer.SafeDeref(payload.Comment.User.Login),
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

func extractTag(payload *github.PushEvent) string {
	return strings.Replace(pointer.SafeDeref(payload.Ref), "refs/tags/", "", 1)
}
