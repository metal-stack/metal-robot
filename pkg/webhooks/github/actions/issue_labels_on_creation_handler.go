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

type labelsOnCreationHandler struct {
	logger  *slog.Logger
	graphql *githubv4.Client
	repos   map[string]config.RepoActions
	owner   string
}

type labelsOnCreationHandlerParams struct {
	ContentNodeID  string
	RepositoryName string
	URL            string
}

func newLabelsOnCreationHandler(logger *slog.Logger, client *clients.Github, rawConfig map[string]any) (*labelsOnCreationHandler, error) {
	var typedConfig config.LabelsOnCreation
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	return &labelsOnCreationHandler{
		logger:  logger,
		graphql: client.GetGraphQLClient(),
		repos:   typedConfig.SourceRepos,
		owner:   client.Owner(),
	}, nil
}

func (r *labelsOnCreationHandler) Handle(ctx context.Context, p *labelsOnCreationHandlerParams) error {
	repo, ok := r.repos[p.RepositoryName]
	if !ok {
		r.logger.Debug("skip handling labels on creation action, not in list of defined repositories", "source-repo", p.RepositoryName)
		return nil
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

		r.logger.Info("added creation labels to target repo", "labels", repo.Labels, "repository", p.RepositoryName, "url", p.URL)
	} else {
		r.logger.Info("no need to add creation labels because there are none of them present in the target repo", "labels", repo.Labels, "repository", p.RepositoryName, "url", p.URL)
	}

	return nil
}
