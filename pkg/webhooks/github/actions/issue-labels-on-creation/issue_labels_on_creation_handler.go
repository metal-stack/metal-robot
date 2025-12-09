package issue_labels_on_creation

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/github/actions"
	handlerrors "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/common/errors"
	"github.com/mitchellh/mapstructure"
	"github.com/shurcooL/githubv4"
)

type labelsOnCreationHandler struct {
	graphql *githubv4.Client
	repos   map[string]config.RepoActions
	owner   string
}

type Params struct {
	ContentNodeID  string
	RepositoryName string
	URL            string
}

func New(client *clients.Github, rawConfig map[string]any) (actions.WebhookHandler[*Params], error) {
	var typedConfig config.LabelsOnCreation
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	return &labelsOnCreationHandler{
		graphql: client.GetGraphQLClient(),
		repos:   typedConfig.SourceRepos,
		owner:   client.Owner(),
	}, nil
}

// Handle adds configured labels to issues and pull requests for certain repositories
func (r *labelsOnCreationHandler) Handle(ctx context.Context, log *slog.Logger, p *Params) error {
	repo, ok := r.repos[p.RepositoryName]
	if !ok {
		return handlerrors.Skip("skip handling labels on creation action, repository is not configured in the metal-robot configuration")
	}

	if len(repo.Labels) == 0 {
		return nil
	}

	var q struct {
		Repository struct {
			Labels struct {
				Nodes []struct {
					Name string `graphql:"name"`
					ID   string `graphql:"id"`
				} `graphql:"nodes"`
			} `graphql:"labels(first: 50)"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]any{
		"owner": githubv4.String(r.owner),
		"name":  githubv4.String(p.RepositoryName),
	}

	err := r.graphql.Query(ctx, &q, variables)
	if err != nil {
		return fmt.Errorf("error querying graphql: %w", err)
	}

	var (
		labelIDs []githubv4.ID
	)

	for _, n := range repo.Labels {
		for _, l := range q.Repository.Labels.Nodes {
			if l.Name == n {
				labelIDs = append(labelIDs, l.ID)
				break
			}
		}
	}

	if len(labelIDs) > 0 {
		var m struct {
			AddLabelsToLabelable struct {
				ClientMutationId string `graphql:"clientMutationId"`
			} `graphql:"addLabelsToLabelable(input: $input)"`
		}

		input := githubv4.AddLabelsToLabelableInput{
			LabelableID: p.ContentNodeID,
			LabelIDs:    labelIDs,
		}

		err = r.graphql.Mutate(ctx, &m, input, nil)
		if err != nil {
			return fmt.Errorf("error mutating graphql: %w", err)
		}

		log.Info("added creation labels to target repo", "labels", repo.Labels)
	} else {
		log.Info("no need to add creation labels because there are none of them present in the target repo", "labels", repo.Labels)
	}

	return nil
}
