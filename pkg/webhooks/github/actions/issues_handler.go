package actions

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/mitchellh/mapstructure"
	"github.com/shurcooL/githubv4"
)

type IssuesAction struct {
	logger  *slog.Logger
	client  *clients.Github
	graphql *githubv4.Client
}

type IssuesActionParams struct {
	RepositoryName string
	RepositoryURL  string
	URL            string
}

func NewIssuesAction(logger *slog.Logger, client *clients.Github, rawConfig map[string]any) (*IssuesAction, error) {
	var typedConfig config.IssueHandlerConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	return &IssuesAction{
		logger:  logger,
		client:  client,
		graphql: client.GetGraphQLClient(),
	}, nil
}

func (r *IssuesAction) HandleIssue(ctx context.Context, p *IssuesActionParams) error {
	if err := r.addToProject(ctx, p); err != nil {
		return fmt.Errorf("unable to add issue to project: %w", err)
	}

	return nil
}

func (r *IssuesAction) addToProject(ctx context.Context, p *IssuesActionParams) error {
	var query struct {
		Viewer struct {
			Login     githubv4.String
			CreatedAt githubv4.DateTime
		}
	}

	err := r.graphql.Query(ctx, &query, nil)
	if err != nil {
		return fmt.Errorf("error querying graphql: %w", err)
	}

	fmt.Println("    Login:", query.Viewer.Login)
	fmt.Println("CreatedAt:", query.Viewer.CreatedAt)

	return nil
}
