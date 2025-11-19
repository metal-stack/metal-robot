package github

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
	"github.com/metal-stack/metal-robot/pkg/webhooks/github/actions"

	aggregate_releases "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/aggregate-releases"
	distribute_releases "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/distribute-releases"
	docs_preview_comment "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/docs-preview-comment"
	issue_comments "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/issue-comments"
	issue_labels_on_creation "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/issue-labels-on-creation"
	project_item_add "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/project-item-add"
	project_v2_item "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/project-v2-item"
	release_drafter "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/release-drafter"
	repository_maintainers "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/repository-maintainers"
	yaml_translate_releases "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/yaml-translate-releases"

	"github.com/google/go-github/v74/github"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"golang.org/x/sync/errgroup"
)

const (
	githubActionReleased string = "released"
	githubActionOpened   string = "opened"
	githubActionClosed   string = "closed"
	githubActionCreated  string = "created"
	githubActionEdited   string = "edited"
)

type WebhookActions struct {
	logger *slog.Logger

	aggregateReleasesHandlers     []actions.WebhookHandler[*aggregate_releases.Params]
	distributeReleasesHandlers    []actions.WebhookHandler[*distribute_releases.Params]
	docsPreviewCommentHandlers    []actions.WebhookHandler[*docs_preview_comment.Params]
	issueCommentsHandlers         []actions.WebhookHandler[*issue_comments.Params]
	labelsOnCreationHandlers      []actions.WebhookHandler[*issue_labels_on_creation.Params]
	projectItemAddHandlers        []actions.WebhookHandler[*project_item_add.Params]
	projectV2ItemHandlers         []actions.WebhookHandler[*project_v2_item.Params]
	releaseDrafterHandlers        []actions.WebhookHandler[*release_drafter.Params]
	appendMergedPRsHandlers       []actions.WebhookHandler[*release_drafter.AppendMergedPrParams]
	repositoryMaintainersHandlers []actions.WebhookHandler[*repository_maintainers.Params]
	yamlTranslateReleasesHandlers []actions.WebhookHandler[*yaml_translate_releases.Params]
}

func initHandlers(logger *slog.Logger, cs clients.ClientMap, cfg config.WebhookActions) (*WebhookActions, error) {
	actions := WebhookActions{
		logger: logger,
	}

	for _, spec := range cfg {
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
		case config.ActionCreateRepositoryMaintainers:
			h, err := repository_maintainers.New(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.repositoryMaintainersHandlers = append(actions.repositoryMaintainersHandlers, h)
		case config.ActionDocsPreviewComment:
			h, err := docs_preview_comment.New(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.docsPreviewCommentHandlers = append(actions.docsPreviewCommentHandlers, h)
		case config.ActionLabelsOnIssueCreation:
			h, err := issue_labels_on_creation.New(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.labelsOnCreationHandlers = append(actions.labelsOnCreationHandlers, h)
		case config.ActionAggregateReleases:
			h, err := aggregate_releases.New(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.aggregateReleasesHandlers = append(actions.aggregateReleasesHandlers, h)
		case config.ActionDistributeReleases:
			h, err := distribute_releases.New(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.distributeReleasesHandlers = append(actions.distributeReleasesHandlers, h)
		case config.ActionReleaseDraft:
			h, err := release_drafter.New(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.releaseDrafterHandlers = append(actions.releaseDrafterHandlers, h)

			h2, err := release_drafter.NewAppendMergedPRs(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.appendMergedPRsHandlers = append(actions.appendMergedPRsHandlers, h2)
		case config.ActionYAMLTranslateReleases:
			h, err := yaml_translate_releases.New(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.yamlTranslateReleasesHandlers = append(actions.yamlTranslateReleasesHandlers, h)
		case config.ActionProjectItemAddHandler:
			h, err := project_item_add.New(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.projectItemAddHandlers = append(actions.projectItemAddHandlers, h)
		case config.ActionProjectV2ItemHandler:
			h, err := project_v2_item.New(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.projectV2ItemHandlers = append(actions.projectV2ItemHandlers, h)
		case config.ActionIssueCommentsHandler:
			h, err := issue_comments.New(logger, c.(*clients.Github), spec.Args)
			if err != nil {
				return nil, err
			}
			actions.issueCommentsHandlers = append(actions.issueCommentsHandlers, h)
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

	for _, a := range w.aggregateReleasesHandlers {
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

			if action != githubActionReleased {
				return nil
			}

			err := a.Handle(ctx, &aggregate_releases.Params{
				RepositoryName: repoName,
				RepositoryURL:  repoURL,
				TagName:        tagName,
				Sender:         login,
			})
			if err != nil {
				return err
			}

			return nil
		})
	}

	for _, a := range w.releaseDrafterHandlers {
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

			if action != githubActionReleased {
				return nil
			}

			err := a.Handle(ctx, &release_drafter.Params{
				RepositoryName:       repoName,
				TagName:              tagName,
				ComponentReleaseInfo: releaseBody,
				ReleaseURL:           releaseURL,
			})
			if err != nil {
				return err
			}

			return nil
		})
	}

	for _, a := range w.yamlTranslateReleasesHandlers {
		g.Go(func() error {
			var (
				action  = pointer.SafeDeref(payload.Action)
				repo    = pointer.SafeDeref(payload.Repo)
				release = pointer.SafeDeref(payload.Release)

				repoName = pointer.SafeDeref(repo.Name)
				cloneURL = pointer.SafeDeref(repo.CloneURL)
				tagName  = pointer.SafeDeref(release.TagName)
			)

			if action != githubActionReleased {
				return nil
			}

			err := a.Handle(ctx, &yaml_translate_releases.Params{
				RepositoryName: repoName,
				RepositoryURL:  cloneURL,
				TagName:        tagName,
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

func (w *WebhookActions) ProcessPullRequestEvent(ctx context.Context, payload *github.PullRequestEvent) {
	ctx, cancel := context.WithTimeout(ctx, constants.WebhookHandleTimeout)
	defer cancel()
	g, _ := errgroup.WithContext(ctx)

	for _, a := range w.docsPreviewCommentHandlers {
		g.Go(func() error {
			var (
				action      = pointer.SafeDeref(payload.Action)
				repo        = pointer.SafeDeref(payload.Repo)
				pullRequest = pointer.SafeDeref(payload.PullRequest)

				repoName          = pointer.SafeDeref(repo.Name)
				pullRequestNumber = pointer.SafeDeref(pullRequest.Number)
			)

			if action != githubActionOpened {
				return nil
			}
			if repoName != "docs" { // FIXME: this is a weird convention, this should come from configuration
				return nil
			}

			err := a.Handle(ctx, &docs_preview_comment.Params{
				PullRequestNumber: int(pullRequestNumber),
			})
			if err != nil {
				w.logger.Error("error adding docs preview comment to docs", "repo", repoName, "pull_request", pullRequestNumber, "error", err)
				return err
			}

			return nil
		})
	}

	for _, a := range w.appendMergedPRsHandlers {
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

			if action != githubActionClosed {
				return nil
			}
			if privateRepo {
				return nil
			}
			if !merged {
				return nil
			}

			err := a.Handle(ctx, &release_drafter.AppendMergedPrParams{
				Params: release_drafter.Params{
					RepositoryName:       repoName,
					ComponentReleaseInfo: pullRequestBody,
					TagName:              "",
					ReleaseURL:           "",
				},
				Title:  pullRequestTitle,
				Number: pullRequestNumber,
				Author: pullRequestLogin,
			},
			)
			if err != nil {
				return err
			}

			return nil
		})
	}

	for _, i := range w.projectItemAddHandlers {
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

			if action != githubActionOpened {
				return nil
			}

			err := i.Handle(ctx, &project_item_add.Params{
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

	for _, i := range w.labelsOnCreationHandlers {
		g.Go(func() error {
			var (
				action      = pointer.SafeDeref(payload.Action)
				repo        = pointer.SafeDeref(payload.Repo)
				pullRequest = pointer.SafeDeref(payload.PullRequest)

				repoName          = pointer.SafeDeref(repo.Name)
				pullRequestNodeID = pointer.SafeDeref(pullRequest.NodeID)
				pullRequestURL    = pointer.SafeDeref(pullRequest.HTMLURL)
			)

			if action != githubActionOpened {
				return nil
			}

			err := i.Handle(ctx, &issue_labels_on_creation.Params{
				RepositoryName: repoName,
				URL:            pullRequestURL,
				ContentNodeID:  pullRequestNodeID,
			})
			if err != nil {
				w.logger.Error("error in label pull request on creation handler action", "source-repo", repoName, "error", err)
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

	for _, a := range w.aggregateReleasesHandlers {
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

			err := a.Handle(ctx, &aggregate_releases.Params{
				RepositoryName: repoName,
				RepositoryURL:  repoURL,
				TagName:        tagName,
				Sender:         login,
			})
			if err != nil {
				return err
			}

			return nil
		})
	}

	for _, a := range w.distributeReleasesHandlers {
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

			err := a.Handle(ctx, &distribute_releases.Params{
				RepositoryName: repoName,
				TagName:        tagName,
			})
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

	for _, a := range w.repositoryMaintainersHandlers {
		g.Go(func() error {
			var (
				action = pointer.SafeDeref(payload.Action)
				repo   = pointer.SafeDeref(payload.Repo)
				sender = pointer.SafeDeref(payload.Sender)

				repoName = pointer.SafeDeref(repo.Name)
				login    = pointer.SafeDeref(sender.Login)
			)

			if action != githubActionCreated {
				return nil
			}

			err := a.Handle(ctx, &repository_maintainers.Params{
				RepositoryName: repoName,
				Creator:        login,
			})
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

	for _, a := range w.projectV2ItemHandlers {
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

			if action != githubActionEdited {
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

			err = a.Handle(ctx, &project_v2_item.Params{
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

	for _, i := range w.projectItemAddHandlers {
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

			if action != githubActionOpened {
				return nil
			}

			err := i.Handle(ctx, &project_item_add.Params{
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

	for _, i := range w.labelsOnCreationHandlers {
		g.Go(func() error {
			var (
				action = pointer.SafeDeref(payload.Action)
				repo   = pointer.SafeDeref(payload.Repo)
				issue  = pointer.SafeDeref(payload.Issue)

				repoName = pointer.SafeDeref(repo.Name)
				nodeID   = pointer.SafeDeref(issue.NodeID)
				url      = pointer.SafeDeref(issue.URL)
			)

			if action != githubActionOpened {
				return nil
			}

			err := i.Handle(ctx, &issue_labels_on_creation.Params{
				RepositoryName: repoName,
				URL:            url,
				ContentNodeID:  nodeID,
			})
			if err != nil {
				w.logger.Error("error in label issue on creation handler action", "source-repo", repoName, "error", err)
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

	for _, i := range w.issueCommentsHandlers {
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

			if action != githubActionCreated {
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

			err = i.Handle(ctx, &issue_comments.Params{
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
