package docs_preview_comment

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/go-github/v79/github"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/handlers"
	"github.com/mitchellh/mapstructure"
)

type docsPreviewComment struct {
	client          *clients.Github
	commentTemplate string
	repositoryName  string
}

type Params struct {
	PullRequestNumber int
}

func New(client *clients.Github, rawConfig map[string]any) (handlers.WebhookHandler[*Params], error) {
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
		client:          client,
		commentTemplate: commentTemplate,
		repositoryName:  typedConfig.RepositoryName,
	}, nil
}

// Handle adds a comment to a pull request in the docs repository
func (d *docsPreviewComment) Handle(ctx context.Context, log *slog.Logger, p *Params) error {
	c, _, err := d.client.GetV3Client().Issues.CreateComment(
		ctx,
		d.client.Organization(),
		d.repositoryName,
		p.PullRequestNumber,
		&github.IssueComment{
			Body: github.Ptr(fmt.Sprintf(d.commentTemplate, p.PullRequestNumber)),
		},
	)
	if err != nil {
		return fmt.Errorf("error creating pull request comment in docs repo: %w", err)
	}

	log.Info("added preview comment in docs repo", "url", c.GetURL())

	return nil
}
