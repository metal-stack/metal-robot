package project_item_add

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/handlers"
	"github.com/mitchellh/mapstructure"
	"github.com/shurcooL/githubv4"
)

type projectItemAdd struct {
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

func New(client *clients.Github, rawConfig map[string]any) (handlers.WebhookHandler[*Params], error) {
	var typedConfig config.ProjectItemAddHandlerConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	return &projectItemAdd{
		client:    client,
		graphql:   client.GetGraphQLClient(),
		projectID: typedConfig.ProjectID,
	}, nil
}

// Handle automatically adds project items in an organization to a project
func (r *projectItemAdd) Handle(ctx context.Context, log *slog.Logger, p *Params) error {
	if err := r.addToProject(ctx, log, p); err != nil {
		return fmt.Errorf("unable to add item to project: %w", err)
	}

	return nil
}

func (r *projectItemAdd) addToProject(ctx context.Context, log *slog.Logger, p *Params) error {
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

	log.Info("added item to project", "project-number", m.AddProjectV2ItemById.Item.Project.Number, "project-title", m.AddProjectV2ItemById.Item.Project.Title)

	return nil
}
