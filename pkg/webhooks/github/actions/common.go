package actions

import (
	"context"
	"encoding/json"
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
			var (
				action  = pointer.SafeDeref(payload.Action)
				repo    = pointer.SafeDeref(payload.Repo)
				sender  = pointer.SafeDeref(payload.Sender)
				release = pointer.SafeDeref(payload.Release)

				repoName = pointer.SafeDeref(repo.Name)
				repoURL  = pointer.SafeDeref(repo.HTMLURL)
				tagName  = pointer.SafeDeref(release.TagName)
				login    = pointer.SafeDeref(sender.Login)
			)

			if action != "released" {
				return nil
			}

			err := a.AggregateRelease(ctx, &AggregateReleaseParams{
				RepositoryName: repoName,
				RepositoryURL:  repoURL,
				TagName:        tagName,
				Sender:         login,
			})
			if err != nil {
				w.logger.Error("error in aggregate release action", "source-repo", repoName, "target-repo", a.repoName, "tag", tagName, "error", err)
				return err
			}

			return nil
		})
	}

	for _, a := range w.rd {
		g.Go(func() error {
			var (
				action  = pointer.SafeDeref(payload.Action)
				repo    = pointer.SafeDeref(payload.Repo)
				release = pointer.SafeDeref(payload.Release)

				repoName    = pointer.SafeDeref(repo.Name)
				tagName     = pointer.SafeDeref(release.TagName)
				releaseURL  = pointer.SafeDeref(release.HTMLURL)
				releaseBody = release.Body
			)

			if action != "released" {
				return nil
			}

			err := a.draft(ctx, &releaseDrafterParams{
				RepositoryName:       repoName,
				TagName:              tagName,
				ComponentReleaseInfo: releaseBody,
				ReleaseURL:           releaseURL,
			})
			if err != nil {
				w.logger.Error("error creating release draft", "repo", a.repoName, "tag", tagName, "error", err)
				return err
			}

			return nil
		})
	}

	for _, a := range w.yr {
		g.Go(func() error {
			var (
				action  = pointer.SafeDeref(payload.Action)
				repo    = pointer.SafeDeref(payload.Repo)
				release = pointer.SafeDeref(payload.Release)

				repoName = pointer.SafeDeref(repo.Name)
				cloneURL = pointer.SafeDeref(repo.CloneURL)
				tagName  = pointer.SafeDeref(release.TagName)
			)

			if action != "released" {
				return nil
			}

			err := a.translateRelease(ctx, &yamlTranslateReleaseParams{
				RepositoryName: repoName,
				RepositoryURL:  cloneURL,
				TagName:        tagName,
			})
			if err != nil {
				w.logger.Error("error creating translating release", "repo", a.repoName, "tag", tagName, "error", err)
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
			var (
				action      = pointer.SafeDeref(payload.Action)
				repo        = pointer.SafeDeref(payload.Repo)
				pullRequest = pointer.SafeDeref(payload.PullRequest)

				repoName          = pointer.SafeDeref(repo.Name)
				pullRequestNumber = pointer.SafeDeref(pullRequest.Number)
			)

			if action != "opened" {
				return nil
			}
			if repoName != "docs" { // FIXME: this is a weird convention, this should come from configuration
				return nil
			}

			err := a.AddDocsPreviewComment(ctx, &docsPreviewCommentParams{
				PullRequestNumber: int(pullRequestNumber),
			})
			if err != nil {
				w.logger.Error("error adding docs preview comment to docs", "repo", repoName, "pull_request", pullRequestNumber, "error", err)
				return err
			}

			return nil
		})
	}

	for _, a := range w.rd {
		g.Go(func() error {
			var (
				action      = pointer.SafeDeref(payload.Action)
				repo        = pointer.SafeDeref(payload.Repo)
				pullRequest = pointer.SafeDeref(payload.PullRequest)

				repoName    = pointer.SafeDeref(repo.Name)
				privateRepo = pointer.SafeDeref(repo.Private)
				merged      = pointer.SafeDeref(pullRequest.Merged)

				pullRequestBody   = payload.PullRequest.Body
				pullRequestTitle  = pointer.SafeDeref(payload.PullRequest.Title)
				pullRequestNumber = pointer.SafeDeref(payload.PullRequest.Number)
				pullRequestLogin  = pointer.SafeDeref(payload.PullRequest.User.Login)
			)

			if action != "closed" {
				return nil
			}
			if privateRepo {
				return nil
			}
			if !merged {
				return nil
			}

			err := a.appendMergedPR(ctx,
				pullRequestTitle,
				pullRequestNumber,
				pullRequestLogin,
				&releaseDrafterParams{
					RepositoryName:       repoName,
					ComponentReleaseInfo: pullRequestBody,
				},
			)
			if err != nil {
				w.logger.Error("error append merged PR to release draft", "repo", a.repoName, "pr", pullRequestTitle, "error", err)
				return err
			}

			return nil
		})
	}

	for _, i := range w.pa {
		g.Go(func() error {
			var (
				action      = pointer.SafeDeref(payload.Action)
				repo        = pointer.SafeDeref(payload.Repo)
				pullRequest = pointer.SafeDeref(payload.PullRequest)

				repoName = pointer.SafeDeref(repo.Name)

				pullRequestNodeID = pointer.SafeDeref(pullRequest.NodeID)
				pullRequestID     = pointer.SafeDeref(pullRequest.ID)
				pullRequestURL    = pointer.SafeDeref(pullRequest.HTMLURL)
			)

			if action != "opened" {
				return nil
			}

			err := i.Handle(ctx, &projectItemAddParams{
				RepositoryName: repoName,
				NodeID:         pullRequestNodeID,
				ID:             pullRequestID,
				URL:            pullRequestURL,
			})
			if err != nil {
				w.logger.Error("error in project item add handler action", "source-repo", repoName, "error", err)
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
			var (
				created = pointer.SafeDeref(payload.Created)
				ref     = pointer.SafeDeref(payload.Ref)

				repo   = pointer.SafeDeref(payload.Repo)
				sender = pointer.SafeDeref(payload.Sender)

				repoName = pointer.SafeDeref(repo.Name)
				repoURL  = pointer.SafeDeref(repo.HTMLURL)

				login = pointer.SafeDeref(sender.Login)

				tagName = extractTag(payload)
			)

			if !created || !strings.HasPrefix(ref, "refs/tags/v") {
				return nil
			}

			err := a.AggregateRelease(ctx, &AggregateReleaseParams{
				RepositoryName: repoName,
				RepositoryURL:  repoURL,
				TagName:        tagName,
				Sender:         login,
			})
			if err != nil {
				w.logger.Error("error in aggregate release action", "source-repo", repoName, "target-repo", a.repoName, "tag", tagName, "error", err)
				return err
			}

			return nil
		})
	}

	for _, a := range w.dr {
		g.Go(func() error {
			var (
				created = pointer.SafeDeref(payload.Created)
				ref     = pointer.SafeDeref(payload.Ref)

				repo = pointer.SafeDeref(payload.Repo)

				repoName = pointer.SafeDeref(repo.Name)

				tagName = extractTag(payload)
			)

			if !created || !strings.HasPrefix(ref, "refs/tags/v") {
				return nil
			}

			params := &distributeReleaseParams{
				RepositoryName: repoName,
				TagName:        tagName,
			}

			err := a.DistributeRelease(ctx, params)
			if err != nil {
				w.logger.Error("error in distribute release action", "source-repo", repoName, "tag", tagName, "error", err)
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
			var (
				action = pointer.SafeDeref(payload.Action)
				repo   = pointer.SafeDeref(payload.Repo)
				sender = pointer.SafeDeref(payload.Sender)

				repoName = pointer.SafeDeref(repo.Name)
				login    = pointer.SafeDeref(sender.Login)
			)

			if action != "created" {
				return nil
			}

			params := &repositoryMaintainersParams{
				RepositoryName: repoName,
				Creator:        login,
			}
			err := a.CreateRepositoryMaintainers(ctx, params)
			if err != nil {
				w.logger.Error("error creating repository maintainers team", "repo", repoName, "error", err)
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
			var (
				action  = pointer.SafeDeref(payload.Action)
				changes = pointer.SafeDeref(payload.Changes)
				project = pointer.SafeDeref(payload.ProjectV2Item)

				fieldValue    = pointer.SafeDeref(changes.FieldValue)
				fieldName     = pointer.SafeDeref(fieldValue.FieldName)
				projectNumber = pointer.SafeDeref(fieldValue.ProjectNumber)

				projectNodeID = pointer.SafeDeref(project.ProjectNodeID)
				contentNodeID = pointer.SafeDeref(project.ContentNodeID)
			)

			if action != "edited" {
				return nil
			}

			if fieldName != "Status" || len(fieldValue.To) == 0 || len(fieldValue.From) == 0 {
				return nil
			}

			var from any
			err := json.Unmarshal(fieldValue.From, &from)
			if err != nil {
				w.logger.Error("unable to parse from", "error", err)
				return err
			}

			if from != nil {
				return nil
			}

			err = a.Handle(ctx, &projectV2ItemHandlerParams{
				ProjectNumber: projectNumber,
				ProjectID:     projectNodeID,
				ContentNodeID: contentNodeID,
			})
			if err != nil {
				w.logger.Error("error handling project v2 item", "project-number", projectNumber, "error", err)
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
			var (
				action = pointer.SafeDeref(payload.Action)
				repo   = pointer.SafeDeref(payload.Repo)
				issue  = pointer.SafeDeref(payload.Issue)

				repoName = pointer.SafeDeref(repo.Name)
				nodeID   = pointer.SafeDeref(issue.NodeID)
				id       = pointer.SafeDeref(issue.ID)
				url      = pointer.SafeDeref(issue.URL)
			)

			if action != "opened" {
				return nil
			}

			err := i.Handle(ctx, &projectItemAddParams{
				RepositoryName: repoName,
				NodeID:         nodeID,
				ID:             id,
				URL:            url,
			})
			if err != nil {
				w.logger.Error("error in project item add handler action", "source-repo", repoName, "error", err)
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
			var (
				action  = pointer.SafeDeref(payload.Action)
				repo    = pointer.SafeDeref(payload.Repo)
				comment = pointer.SafeDeref(payload.Comment)
				user    = pointer.SafeDeref(payload.Comment.User)
				issue   = pointer.SafeDeref(payload.Issue)

				repoName     = pointer.SafeDeref(repo.Name)
				repoCloneURL = pointer.SafeDeref(repo.CloneURL)

				commentBody  = pointer.SafeDeref(comment.Body)
				commentID    = pointer.SafeDeref(comment.ID)
				commentlogin = pointer.SafeDeref(user.Login)

				pullRequestLinks = pointer.SafeDeref(issue.PullRequestLinks)
				pullRequestURL   = pointer.SafeDeref(pullRequestLinks.URL)
			)

			if action != "created" {
				return nil
			}
			if payload.Issue.PullRequestLinks == nil {
				return nil
			}

			parts := strings.Split(pullRequestURL, "/")
			pullRequestNumberString := parts[len(parts)-1]
			pullRequestNumber, err := strconv.ParseInt(pullRequestNumberString, 10, 64)
			if err != nil {
				return err
			}

			err = i.HandleIssueComment(ctx, &IssueCommentsActionParams{
				RepositoryName:    repoName,
				RepositoryURL:     repoCloneURL,
				Comment:           commentBody,
				CommentID:         commentID,
				User:              commentlogin,
				PullRequestNumber: int(pullRequestNumber),
			})
			if err != nil {
				w.logger.Error("error in issue comment handler action", "source-repo", repoName, "error", err)
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
