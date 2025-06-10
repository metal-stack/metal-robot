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
	logger    *slog.Logger
	client    *clients.Github
	graphql   *githubv4.Client
	projectID int
}

type IssuesActionParams struct {
	RepositoryName string
	RepositoryURL  string
	ID             int64
}

func NewIssuesAction(logger *slog.Logger, client *clients.Github, rawConfig map[string]any) (*IssuesAction, error) {
	var typedConfig config.IssueHandlerConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	return &IssuesAction{
		logger:    logger,
		client:    client,
		graphql:   client.GetGraphQLClient(),
		projectID: typedConfig.ProjectID,
	}, nil
}

func (r *IssuesAction) HandleIssue(ctx context.Context, p *IssuesActionParams) error {
	if err := r.addToProject(ctx, p); err != nil {
		return fmt.Errorf("unable to add issue to project: %w", err)
	}

	return nil
}

func (r *IssuesAction) addToProject(ctx context.Context, p *IssuesActionParams) error {
	var m struct {
		Item struct {
			ID githubv4.ID
		} `graphql:"addProjectV2ItemById(input: $input)"`
	}

	input := githubv4.AddProjectV2ItemByIdInput{
		ProjectID: r.projectID,
		ContentID: p.ID,
	}

	err := r.graphql.Mutate(ctx, &m, input, nil)
	if err != nil {
		return fmt.Errorf("error mutating graphql: %w", err)
	}

	return nil
}
