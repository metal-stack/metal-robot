package handlers

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	v3 "github.com/google/go-github/v32/github"
	"github.com/metal-stack/metal-robot/pkg/controllers"
	"github.com/metal-stack/metal-robot/pkg/controllers/webhooks/handlers/actions"
	"github.com/metal-stack/metal-robot/pkg/controllers/webhooks/handlers/git"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type GenerateSwaggerParams struct {
	Logger         *zap.SugaredLogger
	RepositoryName string
	TagName        string
	Client         *v3.Client
	AppClient      *v3.Client
	InstallID      int64
}

type SwaggerClientRepo struct {
	Name    string
	URL     string
	Patches actions.LinePatches
}

var (
	swaggerCommitMessage = "Bump to %s swagger spec version %s"
	swaggerPRBranch      = "auto-generate/%s"
	swaggerRepos         = map[string][]SwaggerClientRepo{
		"metal-api": {
			{
				Name: "metal-python",
				URL:  "https://github.com/metal-stack/metal-python",
				Patches: actions.LinePatches{
					{File: "metal_python/version.py", Line: 1, ReplaceTemplate: "VERSION = '%s'"},
				},
			},
		},
		// Just for testing
		// "metal-robot": {
		// 	{
		// 		Name: "metal-robot",
		// 		URL:  "https://github.com/metal-stack/metal-robot",
		// 		Patches: actions.LinePatches{
		// 			{File: "deploy/kubernetes.yaml", Line: 1, ReplaceTemplate: "VERSION = '%s'"},
		// 		},
		// 	},
		// },
	}
)

// GenerateSwaggerClients is triggered by repositories that release a swagger spec.
// It will create a PR for the client repositories of this repository.
func GenerateSwaggerClients(ctx context.Context, p *GenerateSwaggerParams) error {
	clientRepos, ok := swaggerRepos[p.RepositoryName]
	if !ok {
		p.Logger.Debugw("skip creating swagger client update branches because not a swagger repo", "repo", p.RepositoryName, "release", p.TagName)
		return nil
	}

	tag := p.TagName
	if !strings.HasPrefix(tag, "v") {
		p.Logger.Debugw("skip creating swagger client update branches because tag not starting with v", "repo", p.RepositoryName, "release", p.TagName)
		return nil
	}

	g, ctx := errgroup.WithContext(ctx)

	t, _, err := p.AppClient.Apps.CreateInstallationToken(ctx, p.InstallID, &v3.InstallationTokenOptions{})
	if err != nil {
		return errors.Wrap(err, "error creating installation token")
	}

	for _, clientRepo := range clientRepos {
		g.Go(func() error {
			repoURL, err := url.Parse(clientRepo.URL)
			if err != nil {
				return err
			}
			repoURL.User = url.UserPassword("x-access-token", t.GetToken())

			prBranch := fmt.Sprintf(swaggerPRBranch, tag)

			r, err := git.ShallowClone(repoURL.String(), prBranch, 1)
			if err != nil {
				return err
			}

			reader := func(file string) ([]byte, error) {
				return git.ReadRepoFile(r, file)
			}

			writer := func(file string, content []byte) error {
				return git.WriteRepoFile(r, file, content)
			}

			err = clientRepo.Patches.Apply(reader, writer, tag)
			if err != nil {
				return errors.Wrap(err, "error applying repo updates")
			}

			commitMessage := fmt.Sprintf(swaggerCommitMessage, p.RepositoryName, tag)
			hash, err := git.CommitAndPush(r, commitMessage)
			if err != nil {
				if err == git.NoChangesError {
					p.Logger.Debugw("skip creating swagger client update branch because nothing changed", "repo", p.RepositoryName, "release", tag)
					return nil
				}
				return errors.Wrap(err, "error pushing release file")
			}

			p.Logger.Infow("pushed to swagger client repo", "repo", p.RepositoryName, "release", tag, "branch", prBranch, "hash", hash)

			pr, _, err := p.Client.PullRequests.Create(ctx, controllers.GithubOrganisation, clientRepo.Name, &v3.NewPullRequest{
				Title:               v3.String(commitMessage),
				Head:                v3.String(prBranch),
				Base:                v3.String("master"),
				Body:                v3.String("Updating swagger client"),
				MaintainerCanModify: v3.Bool(true),
			})
			if err != nil {
				if !strings.Contains(err.Error(), "A pull request already exists") {
					return err
				}
			} else {
				p.Logger.Infow("created pull request", "url", pr.GetURL())
			}

			return nil
		})
	}

	return nil
}
