package handlers

import (
	"context"
	"fmt"

	v3 "github.com/google/go-github/v32/github"
	"github.com/metal-stack/metal-robot/pkg/controllers"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type DocsPreviewCommentParams struct {
	Logger            *zap.SugaredLogger
	RepositoryName    string
	PullRequestNumber int
	Client            *v3.Client
}

// AddDocsPreviewComment adds a comment to a pull request in the docs repository
func AddDocsPreviewComment(p *DocsPreviewCommentParams) error {
	b := fmt.Sprintf("Thanks for contributing a pull request to the metal-stack docs!\n\nA rendered preview of your changes will be available at: https://docs.metal-stack.io/previews/PR%d", p.PullRequestNumber)
	_, _, err := p.Client.Issues.CreateComment(
		context.TODO(),
		controllers.GithubOrganisation,
		p.RepositoryName,
		p.PullRequestNumber,
		&v3.IssueComment{
			Body: v3.String(b),
		},
	)
	if err != nil {
		return errors.Wrap(err, "error creating pull request comment in docs repo")
	}

	p.Logger.Infow("added preview comment in docs repo")

	return nil
}
