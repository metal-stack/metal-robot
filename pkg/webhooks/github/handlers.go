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

func initHandlers(logger *slog.Logger, cs clients.ClientMap, cfg config.WebhookActions) error {
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

			actions.Append(func(ctx context.Context, log *slog.Logger, event *github.RepositoryEvent) error {
				var (
					action = pointer.SafeDeref(event.Action)
					repo   = pointer.SafeDeref(event.Repo)
					sender = pointer.SafeDeref(event.Sender)

					repoName = pointer.SafeDeref(repo.Name)
					login    = pointer.SafeDeref(sender.Login)
				)

				if action != githubActionCreated {
					return nil
				}

				return h.Handle(ctx, log, &repository_maintainers.Params{
					RepositoryName: repoName,
					Creator:        login,
				})
			})

		case config.ActionDocsPreviewComment:
			h, err := docs_preview_comment.New(client, spec.Args)
			if err != nil {
				return err
			}

			actions.Append(func(ctx context.Context, log *slog.Logger, event *github.PullRequestEvent) error {
				var (
					action      = pointer.SafeDeref(event.Action)
					repo        = pointer.SafeDeref(event.Repo)
					pullRequest = pointer.SafeDeref(event.PullRequest)

					repoName          = pointer.SafeDeref(repo.Name)
					pullRequestNumber = pointer.SafeDeref(pullRequest.Number)
				)

				if action != githubActionOpened {
					return nil
				}
				if repoName != "docs" { // FIXME: this is a weird convention, this should come from configuration
					return nil
				}

				return h.Handle(ctx, log, &docs_preview_comment.Params{
					PullRequestNumber: int(pullRequestNumber),
				})
			})

		case config.ActionLabelsOnIssueCreation:
			h, err := issue_labels_on_creation.New(client, spec.Args)
			if err != nil {
				return err
			}

			actions.Append(func(ctx context.Context, log *slog.Logger, event *github.PullRequestEvent) error {
				var (
					action      = pointer.SafeDeref(event.Action)
					repo        = pointer.SafeDeref(event.Repo)
					pullRequest = pointer.SafeDeref(event.PullRequest)

					repoName          = pointer.SafeDeref(repo.Name)
					pullRequestNodeID = pointer.SafeDeref(pullRequest.NodeID)
					pullRequestURL    = pointer.SafeDeref(pullRequest.HTMLURL)
				)

				if action != githubActionOpened {
					return nil
				}

				return h.Handle(ctx, log, &issue_labels_on_creation.Params{
					RepositoryName: repoName,
					URL:            pullRequestURL,
					ContentNodeID:  pullRequestNodeID,
				})
			})

			actions.Append(func(ctx context.Context, log *slog.Logger, event *github.IssuesEvent) error {
				var (
					action = pointer.SafeDeref(event.Action)
					repo   = pointer.SafeDeref(event.Repo)
					issue  = pointer.SafeDeref(event.Issue)

					repoName = pointer.SafeDeref(repo.Name)
					nodeID   = pointer.SafeDeref(issue.NodeID)
					url      = pointer.SafeDeref(issue.URL)
				)

				if action != githubActionOpened {
					return nil
				}

				return h.Handle(ctx, log, &issue_labels_on_creation.Params{
					RepositoryName: repoName,
					URL:            url,
					ContentNodeID:  nodeID,
				})
			})

		case config.ActionAggregateReleases:
			h, err := aggregate_releases.New(client, spec.Args)
			if err != nil {
				return err
			}

			actions.Append(func(ctx context.Context, log *slog.Logger, event *github.ReleaseEvent) error {
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
					return nil
				}

				return h.Handle(ctx, log, &aggregate_releases.Params{
					RepositoryName: repoName,
					RepositoryURL:  repoURL,
					TagName:        tagName,
					Sender:         login,
				})
			})

			actions.Append(func(ctx context.Context, log *slog.Logger, event *github.PushEvent) error {
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

				if !created || !strings.HasPrefix(ref, "refs/tags/v") {
					return nil
				}

				return h.Handle(ctx, log, &aggregate_releases.Params{
					RepositoryName: repoName,
					RepositoryURL:  repoURL,
					TagName:        tagName,
					Sender:         login,
				})
			})

		case config.ActionDistributeReleases:
			h, err := distribute_releases.New(client, spec.Args)
			if err != nil {
				return err
			}

			actions.Append(func(ctx context.Context, log *slog.Logger, event *github.PushEvent) error {
				var (
					created = pointer.SafeDeref(event.Created)
					ref     = pointer.SafeDeref(event.Ref)

					repo = pointer.SafeDeref(event.Repo)

					repoName = pointer.SafeDeref(repo.Name)

					tagName = extractTag(event)
				)

				if !created || !strings.HasPrefix(ref, "refs/tags/v") {
					return nil
				}

				return h.Handle(ctx, log, &distribute_releases.Params{
					RepositoryName: repoName,
					TagName:        tagName,
				})
			})

		case config.ActionReleaseDraft:
			h, err := release_drafter.New(client, spec.Args)
			if err != nil {
				return err
			}

			actions.Append(func(ctx context.Context, log *slog.Logger, event *github.ReleaseEvent) error {
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
					return nil
				}

				return h.Handle(ctx, log, &release_drafter.Params{
					RepositoryName:       repoName,
					TagName:              tagName,
					ComponentReleaseInfo: releaseBody,
					ReleaseURL:           releaseURL,
				})
			})

			h2, err := release_drafter.NewAppendMergedPRs(logger, client, spec.Args)
			if err != nil {
				return err
			}

			actions.Append(func(ctx context.Context, log *slog.Logger, event *github.PullRequestEvent) error {
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
					return nil
				}
				if privateRepo {
					return nil
				}
				if !merged {
					return nil
				}

				return h2.Handle(ctx, log, &release_drafter.AppendMergedPrParams{
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
			})

		case config.ActionYAMLTranslateReleases:
			h, err := yaml_translate_releases.New(client, spec.Args)
			if err != nil {
				return err
			}

			actions.Append(func(ctx context.Context, log *slog.Logger, event *github.ReleaseEvent) error {
				var (
					action  = pointer.SafeDeref(event.Action)
					repo    = pointer.SafeDeref(event.Repo)
					release = pointer.SafeDeref(event.Release)

					repoName = pointer.SafeDeref(repo.Name)
					cloneURL = pointer.SafeDeref(repo.CloneURL)
					tagName  = pointer.SafeDeref(release.TagName)
				)

				if action != githubActionReleased {
					return nil
				}

				return h.Handle(ctx, log, &yaml_translate_releases.Params{
					RepositoryName: repoName,
					RepositoryURL:  cloneURL,
					TagName:        tagName,
				})
			})

		case config.ActionProjectItemAddHandler:
			h, err := project_item_add.New(client, spec.Args)
			if err != nil {
				return err
			}

			actions.Append(func(ctx context.Context, log *slog.Logger, event *github.PullRequestEvent) error {
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
					return nil
				}

				return h.Handle(ctx, log, &project_item_add.Params{
					RepositoryName: repoName,
					NodeID:         pullRequestNodeID,
					ID:             pullRequestID,
					URL:            pullRequestURL,
				})
			})

			actions.Append(func(ctx context.Context, log *slog.Logger, event *github.IssuesEvent) error {
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
					return nil
				}

				return h.Handle(ctx, log, &project_item_add.Params{
					RepositoryName: repoName,
					NodeID:         nodeID,
					ID:             id,
					URL:            url,
				})
			})

		case config.ActionProjectV2ItemHandler:
			h, err := project_v2_item.New(client, spec.Args)
			if err != nil {
				return err
			}

			actions.Append(func(ctx context.Context, log *slog.Logger, event *github.ProjectV2ItemEvent) error {
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
					return nil
				}

				if fieldName != "Status" || len(fieldValue.To) == 0 || len(fieldValue.From) == 0 {
					return nil
				}

				var from any
				err := json.Unmarshal(fieldValue.From, &from)
				if err != nil {
					return err
				}

				if from != nil {
					return nil
				}

				return h.Handle(ctx, log, &project_v2_item.Params{
					ProjectNumber: projectNumber,
					ProjectID:     projectNodeID,
					ContentNodeID: contentNodeID,
				})
			})
		case config.ActionIssueCommentsHandler:
			h, err := issue_comments.New(client, spec.Args)
			if err != nil {
				return err
			}

			actions.Append(func(ctx context.Context, log *slog.Logger, event *github.IssueCommentEvent) error {
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
					return nil
				}
				if event.Issue.PullRequestLinks == nil {
					return nil
				}

				parts := strings.Split(pullRequestURL, "/")
				pullRequestNumberString := parts[len(parts)-1]
				pullRequestNumber, err := strconv.ParseInt(pullRequestNumberString, 10, 64)
				if err != nil {
					return err
				}

				return h.Handle(ctx, log, &issue_comments.Params{
					RepositoryName:    repoName,
					RepositoryURL:     repoCloneURL,
					Comment:           commentBody,
					CommentID:         commentID,
					User:              commentlogin,
					PullRequestNumber: int(pullRequestNumber),
				})
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
