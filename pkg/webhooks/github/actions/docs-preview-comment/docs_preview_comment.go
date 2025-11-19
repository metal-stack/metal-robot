package docs_preview_comment

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/go-github/v74/github"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/github/actions"
	"github.com/mitchellh/mapstructure"
)

type docsPreviewComment struct {
	logger          *slog.Logger
	client          *clients.Github
	commentTemplate string
	repositoryName  string
}

type Params struct {
	PullRequestNumber int
}

func New(logger *slog.Logger, client *clients.Github, rawConfig map[string]any) (actions.WebhookHandler[*Params], error) {
	var (
		commentTemplate = "#%d"
	)

	var typedConfig config.DocsPreviewCommentConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	if typedConfig.CommentTemplate != nil {
		commentTemplate = *typedConfig.CommentTemplate
	}
	if typedConfig.RepositoryName == "" {
		return nil, fmt.Errorf("repository must be specified")
	}

	return &docsPreviewComment{
		logger:          logger,
		client:          client,
		commentTemplate: commentTemplate,
		repositoryName:  typedConfig.RepositoryName,
	}, nil
}

// AddDocsPreviewComment adds a comment to a pull request in the docs repository
func (d *docsPreviewComment) Handle(ctx context.Context, p *Params) error {
	b := fmt.Sprintf(d.commentTemplate, p.PullRequestNumber)
	c, _, err := d.client.GetV3Client().Issues.CreateComment(
		ctx,
		d.client.Organization(),
		d.repositoryName,
		p.PullRequestNumber,
		&github.IssueComment{
			Body: github.Ptr(b),
		},
	)
	if err != nil {
		return fmt.Errorf("error creating pull request comment in docs repo %w", err)
	}

	d.logger.Info("added preview comment in docs repo", "url", c.GetURL())

	return nil
}
