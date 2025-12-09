package project_v2_item

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/handlers"
	handlerrors "github.com/metal-stack/metal-robot/pkg/webhooks/handlers/errors"
	"github.com/mitchellh/mapstructure"
	"github.com/shurcooL/githubv4"
)

type ProjectV2ItemHandler struct {
	graphql      *githubv4.Client
	projectID    string
	removeLabels []string
}

type Params struct {
	ProjectNumber int64
	ProjectID     string
	ContentNodeID string
}

func New(client *clients.Github, rawConfig map[string]any) (handlers.WebhookHandler[*Params], error) {
	var typedConfig config.ProjectV2ItemHandlerConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	return &ProjectV2ItemHandler{
		graphql:      client.GetGraphQLClient(),
		projectID:    typedConfig.ProjectID,
		removeLabels: typedConfig.RemoveLabels,
	}, nil
}

// Handle removes a label from an issue or pull request when being moved to a certain project column
func (r *ProjectV2ItemHandler) Handle(ctx context.Context, log *slog.Logger, p *Params) error {
	if p.ProjectID != r.projectID {
		return handlerrors.Skip("skip removing labels from project v2 item, only acting on project-id %q", r.projectID)
	}

	var q struct {
		Node struct {
			PullRequest struct {
				URL    string `graphql:"url"`
				Labels struct {
					Nodes []struct {
						Name string `graphql:"name"`
						ID   string `graphql:"id"`
					} `graphql:"nodes"`
				} `graphql:"labels(first: 20)"`
			} `graphql:"... on PullRequest"`
			Issue struct {
				URL    string `graphql:"url"`
				Labels struct {
					Nodes []struct {
						Name string `graphql:"name"`
						ID   string `graphql:"id"`
					} `graphql:"nodes"`
				} `graphql:"labels(first: 20)"`
			} `graphql:"... on Issue"`
		} `graphql:"node(id: $node_id)"`
	}

	variables := map[string]any{
		"node_id": githubv4.ID(p.ContentNodeID),
	}

	err := r.graphql.Query(ctx, &q, variables)
	if err != nil {
		return fmt.Errorf("error querying graphql: %w", err)
	}

	var (
		labelIDs []githubv4.ID
		url      = q.Node.PullRequest.URL
	)

	if q.Node.Issue.URL != "" {
		url = q.Node.Issue.URL
	}

	for _, n := range q.Node.PullRequest.Labels.Nodes {
		if slices.Contains(r.removeLabels, n.Name) {
			labelIDs = append(labelIDs, n.ID)
		}
	}
	for _, n := range q.Node.Issue.Labels.Nodes {
		if slices.Contains(r.removeLabels, n.Name) {
			labelIDs = append(labelIDs, n.ID)
		}
	}

	if len(labelIDs) > 0 {
		var m struct {
			RemoveLabelsFromLabelable struct {
				ClientMutationId string `graphql:"clientMutationId"`
			} `graphql:"removeLabelsFromLabelable(input: $input)"`
		}

		input := githubv4.RemoveLabelsFromLabelableInput{
			LabelableID: p.ContentNodeID,
			LabelIDs:    labelIDs,
		}

		err = r.graphql.Mutate(ctx, &m, input, nil)
		if err != nil {
			return fmt.Errorf("error mutating graphql: %w", err)
		}

		log.Info("removed labels from project v2 item", "labels", r.removeLabels, "project-number", r.projectID, "item-url", url)
	} else {
		log.Info("no need to remove labels from project v2 item, none of them are attached", "labels", r.removeLabels, "project-number", r.projectID, "item-url", url)
	}

	return nil
}
