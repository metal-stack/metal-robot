package handlers

import (
	"context"
	"fmt"
	"strings"

	v3 "github.com/google/go-github/v32/github"
	"github.com/metal-stack/metal-robot/pkg/controllers"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

var (
	MaintainersTeamPostfix = "maintainers"
)

type RepositoryMaintainersParams struct {
	Logger         *zap.SugaredLogger
	RepositoryName string
	Creator        string
	Client         *v3.Client
}

func CreateRepositoryMaintainersTeam(ctx context.Context, p *RepositoryMaintainersParams) error {
	var (
		name        = fmt.Sprintf("%s-%s", p.RepositoryName, MaintainersTeamPostfix)
		description = fmt.Sprintf("Maintainers of %s", p.RepositoryName)
	)

	_, _, err := p.Client.Teams.CreateTeam(ctx, controllers.GithubOrganisation, v3.NewTeam{
		Name:        name,
		Description: v3.String(description),
		Maintainers: []string{p.Creator},
		RepoNames:   []string{p.RepositoryName},
		Privacy:     v3.String("closed"),
	})
	if err != nil {
		// Could be that team already exists...
		if !strings.Contains(err.Error(), "Name must be unique for this org") {
			return errors.Wrap(err, "error creating maintainers team")
		}
	} else {
		p.Logger.Infow("created new maintainers team for repository", "repository", p.RepositoryName, "team", name)
	}

	return nil
}
