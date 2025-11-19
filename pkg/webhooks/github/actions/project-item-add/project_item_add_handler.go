package project_item_add

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/github/actions"
	"github.com/mitchellh/mapstructure"
	"github.com/shurcooL/githubv4"
)

type projectItemAdd struct {
	logger    *slog.Logger
	client    *clients.Github
	graphql   *githubv4.Client
	projectID string
}

type Params struct {
	RepositoryName string
	NodeID         string
	ID             int64
	URL            string
}

func New(logger *slog.Logger, client *clients.Github, rawConfig map[string]any) (actions.WebhookHandler[*Params], error) {
	var typedConfig config.ProjectItemAddHandlerConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	return &projectItemAdd{
		logger:    logger,
		client:    client,
		graphql:   client.GetGraphQLClient(),
		projectID: typedConfig.ProjectID,
	}, nil
}

func (r *projectItemAdd) Handle(ctx context.Context, p *Params) error {
	if err := r.addToProject(ctx, p); err != nil {
		return fmt.Errorf("unable to add item to project: %w", err)
	}

	return nil
}

func (r *projectItemAdd) addToProject(ctx context.Context, p *Params) error {
	var m struct {
		AddProjectV2ItemById struct {
			Item struct {
				ID      githubv4.ID
				Project struct {
					Title  githubv4.String
					Number githubv4.Int
				}
			}
		} `graphql:"addProjectV2ItemById(input: $input)"`
	}

	input := githubv4.AddProjectV2ItemByIdInput{
		ProjectID: r.projectID,
		ContentID: p.NodeID,
	}

	err := r.graphql.Mutate(ctx, &m, input, nil)
	if err != nil {
		return fmt.Errorf("error mutating graphql: %w", err)
	}

	r.logger.Info("added item to project", "project-number", m.AddProjectV2ItemById.Item.Project.Number, "project-title", m.AddProjectV2ItemById.Item.Project.Title, "url", p.URL)

	return nil
}
