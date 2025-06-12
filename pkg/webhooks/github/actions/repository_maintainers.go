package actions

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/go-github/v72/github"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/mitchellh/mapstructure"
)

type repositoryMaintainers struct {
	logger          *slog.Logger
	client          *clients.Github
	suffix          string
	additionalTeams repositoryAdditionalMemberships
}

type repositoryAdditionalMemberships []repositoryTeamMembership

type repositoryTeamMembership struct {
	teamSlug   string
	permission string
}

type repositoryMaintainersParams struct {
	RepositoryName string
	Creator        string
}

func newCreateRepositoryMaintainers(logger *slog.Logger, client *clients.Github, rawConfig map[string]any) (*repositoryMaintainers, error) {
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
		team := team
		additionalTeams = append(additionalTeams, repositoryTeamMembership{
			teamSlug:   team.TeamSlug,
			permission: team.Permission,
		})
	}

	return &repositoryMaintainers{
		logger:          logger,
		client:          client,
		suffix:          suffix,
		additionalTeams: additionalTeams,
	}, nil
}

func (r *repositoryMaintainers) CreateRepositoryMaintainers(ctx context.Context, p *repositoryMaintainersParams) error {
	var (
		name        = fmt.Sprintf("%s%s", p.RepositoryName, r.suffix)
		description = fmt.Sprintf("Maintainers of %s", p.RepositoryName)
	)

	_, _, err := r.client.GetV3Client().Teams.CreateTeam(ctx, r.client.Organization(), github.NewTeam{
		Name:        name,
		Description: github.Ptr(description),
		Maintainers: []string{p.Creator},
		RepoNames:   []string{p.RepositoryName},
		Privacy:     github.Ptr("closed"),
	})
	if err != nil {
		if strings.Contains(err.Error(), "Name must be unique for this org") {
			r.logger.Info("maintainers team for repository already exists", "repository", p.RepositoryName, "team", name)
		} else {
			return fmt.Errorf("error creating maintainers team %w", err)
		}
	} else {
		r.logger.Info("created new maintainers team for repository", "repository", p.RepositoryName, "team", name)
	}

	memberships := []repositoryTeamMembership{
		{
			teamSlug:   name,
			permission: "maintain",
		},
	}
	memberships = append(memberships, r.additionalTeams...)

	for _, team := range memberships {
		team := team

		_, err := r.client.GetV3Client().Teams.AddTeamRepoBySlug(ctx, r.client.Organization(), team.teamSlug, r.client.Organization(), p.RepositoryName, &github.TeamAddTeamRepoOptions{
			Permission: team.permission,
		})
		if err != nil {
			return fmt.Errorf("error adding team membership: %w", err)
		} else {
			r.logger.Info("added team to repository", "repository", p.RepositoryName, "team", team.teamSlug, "permission", team.permission)
		}
	}

	return nil
}
