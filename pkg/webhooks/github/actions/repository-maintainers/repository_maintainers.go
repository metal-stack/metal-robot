package repository_maintainers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/go-github/v79/github"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/handlers"
	"github.com/mitchellh/mapstructure"
)

type repositoryMaintainers struct {
	client          *clients.Github
	suffix          string
	additionalTeams repositoryAdditionalMemberships
}

type repositoryAdditionalMemberships []repositoryTeamMembership

type repositoryTeamMembership struct {
	teamSlug   string
	permission string
}

type Params struct {
	RepositoryName string
	Creator        string
}

func New(client *clients.Github, rawConfig map[string]any) (handlers.WebhookHandler[*Params], error) {
	var (
		suffix = "-maintainers"
	)

	var typedConfig config.RepositoryMaintainersConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	if typedConfig.Suffix != nil {
		suffix = *typedConfig.Suffix
	}

	var additionalTeams repositoryAdditionalMemberships
	for _, team := range typedConfig.AdditionalMemberships {
		additionalTeams = append(additionalTeams, repositoryTeamMembership{
			teamSlug:   team.TeamSlug,
			permission: team.Permission,
		})
	}

	return &repositoryMaintainers{
		client:          client,
		suffix:          suffix,
		additionalTeams: additionalTeams,
	}, nil
}

// Handle creates a maintainers team for a newly created repository
func (r *repositoryMaintainers) Handle(ctx context.Context, log *slog.Logger, p *Params) error {
	var (
		name        = fmt.Sprintf("%s%s", p.RepositoryName, r.suffix)
		description = fmt.Sprintf("Maintainers of %s", p.RepositoryName)
	)

	_, _, err := r.client.GetV3Client().Teams.CreateTeam(ctx, r.client.Organization(), github.NewTeam{
		Name:        name,
		Description: new(description),
		Maintainers: []string{p.Creator},
		RepoNames:   []string{p.RepositoryName},
		Privacy:     new("closed"),
	})
	if err != nil {
		if strings.Contains(err.Error(), "Name must be unique for this org") {
			log.Info("maintainers team for repository already exists", "team", name)
		} else {
			return fmt.Errorf("error creating maintainers team: %w", err)
		}
	} else {
		log.Info("created new maintainers team for repository", "team", name)
	}

	memberships := []repositoryTeamMembership{
		{
			teamSlug:   name,
			permission: "maintain",
		},
	}
	memberships = append(memberships, r.additionalTeams...)

	for _, team := range memberships {

		_, err := r.client.GetV3Client().Teams.AddTeamRepoBySlug(ctx, r.client.Organization(), team.teamSlug, r.client.Organization(), p.RepositoryName, &github.TeamAddTeamRepoOptions{
			Permission: team.permission,
		})
		if err != nil {
			return fmt.Errorf("error adding team membership: %w", err)
		} else {
			log.Info("added team to repository", "team", team.teamSlug, "permission", team.permission)
		}
	}

	return nil
}
