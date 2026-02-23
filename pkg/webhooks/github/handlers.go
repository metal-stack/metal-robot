package github

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"

	aggregate_releases "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/aggregate-releases"
	distribute_releases "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/distribute-releases"
	issue_comments "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/issue-comments"
	issue_labels_on_creation "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/issue-labels-on-creation"
	project_item_add "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/project-item-add"
	project_v2_item "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/project-v2-item"
	release_drafter "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/release-drafter"
	repository_maintainers "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/repository-maintainers"
	yaml_translate_releases "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/yaml-translate-releases"
	"github.com/metal-stack/metal-robot/pkg/webhooks/handlers"
	handlerrors "github.com/metal-stack/metal-robot/pkg/webhooks/handlers/errors"

	"github.com/google/go-github/v79/github"
	"github.com/metal-stack/metal-lib/pkg/pointer"
)

const (
	githubActionReleased string = "released"
	githubActionOpened   string = "opened"
	githubActionClosed   string = "closed"
	githubActionCreated  string = "created"
	githubActionEdited   string = "edited"
)

func initHandlers(logger *slog.Logger, cs clients.ClientMap, path string, cfg config.WebhookActions) error {
	for _, spec := range cfg {
		c, ok := cs[spec.Client]
		if !ok {
			return fmt.Errorf("webhook action client not found: %s", spec.Client)
		}

		switch clientType := c.(type) {
		case *clients.Github:
		default:
			return fmt.Errorf("action %s only supports github clients, not: %s", spec.Type, clientType)
		}

		client := c.(*clients.Github)

		switch t := spec.Type; t {
		case config.ActionCreateRepositoryMaintainers:
			h, err := repository_maintainers.New(client, spec.Args)
			if err != nil {
				return err
			}

			handlers.Register(string(t), path, h, func(event *github.RepositoryEvent) (*repository_maintainers.Params, error) {
				var (
					action = pointer.SafeDeref(event.Action)
					repo   = pointer.SafeDeref(event.Repo)
					sender = pointer.SafeDeref(event.Sender)

					repoName = pointer.SafeDeref(repo.Name)
					login    = pointer.SafeDeref(sender.Login)
				)

				if action != githubActionCreated {
					return nil, handlerrors.SkipOnlyActions(githubActionCreated)
				}

				return &repository_maintainers.Params{
					RepositoryName: repoName,
					Creator:        login,
				}, nil
			})

		case config.ActionLabelsOnIssueCreation:
			h, err := issue_labels_on_creation.New(client, spec.Args)
			if err != nil {
				return err
			}

			handlers.Register(string(t), path, h, func(event *github.PullRequestEvent) (*issue_labels_on_creation.Params, error) {
				var (
					action      = pointer.SafeDeref(event.Action)
					repo        = pointer.SafeDeref(event.Repo)
					pullRequest = pointer.SafeDeref(event.PullRequest)

					repoName          = pointer.SafeDeref(repo.Name)
					pullRequestNodeID = pointer.SafeDeref(pullRequest.NodeID)
					pullRequestURL    = pointer.SafeDeref(pullRequest.HTMLURL)
				)

				if action != githubActionOpened {
					return nil, handlerrors.SkipOnlyActions(githubActionOpened)
				}

				return &issue_labels_on_creation.Params{
					RepositoryName: repoName,
					URL:            pullRequestURL,
					ContentNodeID:  pullRequestNodeID,
				}, nil
			})

			handlers.Register(string(t), path, h, func(event *github.IssuesEvent) (*issue_labels_on_creation.Params, error) {
				var (
					action = pointer.SafeDeref(event.Action)
					repo   = pointer.SafeDeref(event.Repo)
					issue  = pointer.SafeDeref(event.Issue)

					repoName = pointer.SafeDeref(repo.Name)
					nodeID   = pointer.SafeDeref(issue.NodeID)
					url      = pointer.SafeDeref(issue.URL)
				)

				if action != githubActionOpened {
					return nil, handlerrors.SkipOnlyActions(githubActionOpened)
				}

				return &issue_labels_on_creation.Params{
					RepositoryName: repoName,
					URL:            url,
					ContentNodeID:  nodeID,
				}, nil
			})

		case config.ActionAggregateReleases:
			h, err := aggregate_releases.New(client, spec.Args)
			if err != nil {
				return err
			}

			handlers.Register(string(t), path, h, func(event *github.ReleaseEvent) (*aggregate_releases.Params, error) {
				var (
					action  = pointer.SafeDeref(event.Action)
					repo    = pointer.SafeDeref(event.Repo)
					sender  = pointer.SafeDeref(event.Sender)
					release = pointer.SafeDeref(event.Release)

					repoName = pointer.SafeDeref(repo.Name)
					repoURL  = pointer.SafeDeref(repo.HTMLURL)
					tagName  = pointer.SafeDeref(release.TagName)
					login    = pointer.SafeDeref(sender.Login)
				)

				if action != githubActionReleased {
					return nil, handlerrors.SkipOnlyActions(githubActionReleased)
				}

				return &aggregate_releases.Params{
					RepositoryName: repoName,
					RepositoryURL:  repoURL,
					TagName:        tagName,
					Sender:         login,
				}, nil
			})

			handlers.Register(string(t), path, h, func(event *github.PushEvent) (*aggregate_releases.Params, error) {
				var (
					created = pointer.SafeDeref(event.Created)
					ref     = pointer.SafeDeref(event.Ref)

					repo   = pointer.SafeDeref(event.Repo)
					sender = pointer.SafeDeref(event.Sender)

					repoName = pointer.SafeDeref(repo.Name)
					repoURL  = pointer.SafeDeref(repo.HTMLURL)

					login = pointer.SafeDeref(sender.Login)

					tagName = extractTag(event)
				)

				if !created {
					return nil, handlerrors.Skip("only reacting on created event")
				}

				if !strings.HasPrefix(ref, "refs/tags/v") {
					return nil, handlerrors.Skip("only reacting if ref starts with /refs/tags/v, but has %s", ref)
				}

				return &aggregate_releases.Params{
					RepositoryName: repoName,
					RepositoryURL:  repoURL,
					TagName:        tagName,
					Sender:         login,
				}, nil
			})

		case config.ActionDistributeReleases:
			h, err := distribute_releases.New(client, spec.Args)
			if err != nil {
				return err
			}

			handlers.Register(string(t), path, h, func(event *github.PushEvent) (*distribute_releases.Params, error) {
				var (
					created = pointer.SafeDeref(event.Created)
					ref     = pointer.SafeDeref(event.Ref)

					repo = pointer.SafeDeref(event.Repo)

					repoName = pointer.SafeDeref(repo.Name)

					tagName = extractTag(event)
				)

				if !created {
					return nil, handlerrors.Skip("only reacting on created event")
				}

				if !strings.HasPrefix(ref, "refs/tags/v") {
					return nil, handlerrors.Skip("only reacting if ref starts with /refs/tags/v, but has %s", ref)
				}

				return &distribute_releases.Params{
					RepositoryName: repoName,
					TagName:        tagName,
				}, nil
			})

		case config.ActionReleaseDraft:
			h, err := release_drafter.New(client, spec.Args)
			if err != nil {
				return err
			}

			handlers.Register(string(t), path, h, func(event *github.ReleaseEvent) (*release_drafter.Params, error) {
				var (
					action  = pointer.SafeDeref(event.Action)
					repo    = pointer.SafeDeref(event.Repo)
					release = pointer.SafeDeref(event.Release)

					repoName    = pointer.SafeDeref(repo.Name)
					tagName     = pointer.SafeDeref(release.TagName)
					releaseURL  = pointer.SafeDeref(release.HTMLURL)
					releaseBody = release.Body
				)

				if action != githubActionReleased {
					return nil, handlerrors.SkipOnlyActions(githubActionReleased)
				}

				return &release_drafter.Params{
					RepositoryName:       repoName,
					TagName:              tagName,
					ComponentReleaseInfo: releaseBody,
					ReleaseURL:           releaseURL,
				}, nil
			})

			h2, err := release_drafter.NewAppendMergedPRs(logger, client, spec.Args)
			if err != nil {
				return err
			}

			handlers.Register(string(t), path, h2, func(event *github.PullRequestEvent) (*release_drafter.AppendMergedPrParams, error) {
				var (
					action      = pointer.SafeDeref(event.Action)
					repo        = pointer.SafeDeref(event.Repo)
					pullRequest = pointer.SafeDeref(event.PullRequest)

					repoName    = pointer.SafeDeref(repo.Name)
					privateRepo = pointer.SafeDeref(repo.Private)
					merged      = pointer.SafeDeref(pullRequest.Merged)

					pullRequestBody   = event.PullRequest.Body
					pullRequestTitle  = pointer.SafeDeref(event.PullRequest.Title)
					pullRequestNumber = pointer.SafeDeref(event.PullRequest.Number)
					pullRequestLogin  = pointer.SafeDeref(event.PullRequest.User.Login)
				)

				if action != githubActionClosed {
					return nil, handlerrors.SkipOnlyActions(githubActionClosed)
				}
				if privateRepo {
					return nil, handlerrors.Skip("not reacting on private repos")
				}
				if !merged {
					return nil, handlerrors.Skip("only reacting on merged pull requests")
				}

				return &release_drafter.AppendMergedPrParams{
					Params: release_drafter.Params{
						RepositoryName:       repoName,
						ComponentReleaseInfo: pullRequestBody,
						TagName:              "",
						ReleaseURL:           "",
					},
					Title:  pullRequestTitle,
					Number: pullRequestNumber,
					Author: pullRequestLogin,
				}, nil
			})

		case config.ActionYAMLTranslateReleases:
			h, err := yaml_translate_releases.New(client, spec.Args)
			if err != nil {
				return err
			}

			handlers.Register(string(t), path, h, func(event *github.ReleaseEvent) (*yaml_translate_releases.Params, error) {
				var (
					action  = pointer.SafeDeref(event.Action)
					repo    = pointer.SafeDeref(event.Repo)
					release = pointer.SafeDeref(event.Release)
					sender  = pointer.SafeDeref(event.Sender)

					repoName = pointer.SafeDeref(repo.Name)
					cloneURL = pointer.SafeDeref(repo.CloneURL)
					tagName  = pointer.SafeDeref(release.TagName)

					login = pointer.SafeDeref(sender.Login)
				)

				if action != githubActionReleased {
					return nil, handlerrors.SkipOnlyActions(githubActionReleased)
				}

				return &yaml_translate_releases.Params{
					RepositoryName: repoName,
					RepositoryURL:  cloneURL,
					TagName:        tagName,
					Sender:         login,
				}, nil
			})

		case config.ActionProjectItemAddHandler:
			h, err := project_item_add.New(client, spec.Args)
			if err != nil {
				return err
			}

			handlers.Register(string(t), path, h, func(event *github.PullRequestEvent) (*project_item_add.Params, error) {
				var (
					action      = pointer.SafeDeref(event.Action)
					repo        = pointer.SafeDeref(event.Repo)
					pullRequest = pointer.SafeDeref(event.PullRequest)

					repoName = pointer.SafeDeref(repo.Name)

					pullRequestNodeID = pointer.SafeDeref(pullRequest.NodeID)
					pullRequestID     = pointer.SafeDeref(pullRequest.ID)
					pullRequestURL    = pointer.SafeDeref(pullRequest.HTMLURL)
				)

				if action != githubActionOpened {
					return nil, handlerrors.SkipOnlyActions(githubActionOpened)
				}

				return &project_item_add.Params{
					RepositoryName: repoName,
					NodeID:         pullRequestNodeID,
					ID:             pullRequestID,
					URL:            pullRequestURL,
				}, nil
			})

			handlers.Register(string(t), path, h, func(event *github.IssuesEvent) (*project_item_add.Params, error) {
				var (
					action = pointer.SafeDeref(event.Action)
					repo   = pointer.SafeDeref(event.Repo)
					issue  = pointer.SafeDeref(event.Issue)

					repoName = pointer.SafeDeref(repo.Name)
					nodeID   = pointer.SafeDeref(issue.NodeID)
					id       = pointer.SafeDeref(issue.ID)
					url      = pointer.SafeDeref(issue.URL)
				)

				if action != githubActionOpened {
					return nil, handlerrors.SkipOnlyActions(githubActionOpened)
				}

				return &project_item_add.Params{
					RepositoryName: repoName,
					NodeID:         nodeID,
					ID:             id,
					URL:            url,
				}, nil
			})

		case config.ActionProjectV2ItemHandler:
			h, err := project_v2_item.New(client, spec.Args)
			if err != nil {
				return err
			}

			handlers.Register(string(t), path, h, func(event *github.ProjectV2ItemEvent) (*project_v2_item.Params, error) {
				var (
					action  = pointer.SafeDeref(event.Action)
					changes = pointer.SafeDeref(event.Changes)
					project = pointer.SafeDeref(event.ProjectV2Item)

					fieldValue    = pointer.SafeDeref(changes.FieldValue)
					fieldName     = pointer.SafeDeref(fieldValue.FieldName)
					projectNumber = pointer.SafeDeref(fieldValue.ProjectNumber)

					projectNodeID = pointer.SafeDeref(project.ProjectNodeID)
					contentNodeID = pointer.SafeDeref(project.ContentNodeID)
				)

				if action != githubActionEdited {
					return nil, handlerrors.SkipOnlyActions(githubActionEdited)
				}

				if fieldName != "Status" || len(fieldValue.To) == 0 || len(fieldValue.From) == 0 {
					return nil, handlerrors.Skip("only reacting to changes in status field (that contain contents)")
				}

				var from any
				err := json.Unmarshal(fieldValue.From, &from)
				if err != nil {
					return nil, fmt.Errorf("unable to unmarshal field value: %w", err)
				}

				if from != nil {
					return nil, handlerrors.Skip("from field is nil")
				}

				return &project_v2_item.Params{
					ProjectNumber: projectNumber,
					ProjectID:     projectNodeID,
					ContentNodeID: contentNodeID,
				}, nil
			})
		case config.ActionIssueCommentsHandler:
			h, err := issue_comments.New(client, spec.Args)
			if err != nil {
				return err
			}

			handlers.Register(string(t), path, h, func(event *github.IssueCommentEvent) (*issue_comments.Params, error) {
				var (
					action  = pointer.SafeDeref(event.Action)
					repo    = pointer.SafeDeref(event.Repo)
					comment = pointer.SafeDeref(event.Comment)
					user    = pointer.SafeDeref(event.Comment.User)
					issue   = pointer.SafeDeref(event.Issue)

					repoName     = pointer.SafeDeref(repo.Name)
					repoCloneURL = pointer.SafeDeref(repo.CloneURL)

					commentBody  = pointer.SafeDeref(comment.Body)
					commentID    = pointer.SafeDeref(comment.ID)
					commentlogin = pointer.SafeDeref(user.Login)

					pullRequestLinks = pointer.SafeDeref(issue.PullRequestLinks)
					pullRequestURL   = pointer.SafeDeref(pullRequestLinks.URL)
				)

				if action != githubActionCreated {
					return nil, handlerrors.SkipOnlyActions(githubActionCreated)
				}

				parts := strings.Split(pullRequestURL, "/")
				pullRequestNumberString := parts[len(parts)-1]
				pullRequestNumber, err := strconv.ParseInt(pullRequestNumberString, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("unable to parse pull request number: %w", err)
				}

				return &issue_comments.Params{
					RepositoryName:    repoName,
					RepositoryURL:     repoCloneURL,
					Comment:           commentBody,
					CommentID:         commentID,
					User:              commentlogin,
					PullRequestNumber: int(pullRequestNumber),
				}, nil
			})
		default:
			return fmt.Errorf("handler type not supported: %s", t)
		}

		logger.Debug("initialized github webhook action", "name", spec.Type)
	}

	return nil
}

func extractTag(payload *github.PushEvent) string {
	return strings.Replace(pointer.SafeDeref(payload.Ref), "refs/tags/", "", 1)
}
