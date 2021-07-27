package actions

import (
	"context"
	"fmt"

	v3 "github.com/google/go-github/v37/github"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

const ()

type docsPreviewComment struct {
	logger          *zap.SugaredLogger
	client          *clients.Github
	commentTemplate string
	repositoryName  string
}

type docsPreviewCommentParams struct {
	PullRequestNumber int
}

func newDocsPreviewComment(logger *zap.SugaredLogger, client *clients.Github, rawConfig map[string]interface{}) (*docsPreviewComment, error) {
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
func (d *docsPreviewComment) AddDocsPreviewComment(ctx context.Context, p *docsPreviewCommentParams) error {
	b := fmt.Sprintf(d.commentTemplate, p.PullRequestNumber)
	c, _, err := d.client.GetV3Client().Issues.CreateComment(
		ctx,
		d.client.Organization(),
		d.repositoryName,
		p.PullRequestNumber,
		&v3.IssueComment{
			Body: v3.String(b),
		},
	)
	if err != nil {
		return errors.Wrap(err, "error creating pull request comment in docs repo")
	}

	d.logger.Infow("added preview comment in docs repo", "url", c.GetURL())

	return nil
}
