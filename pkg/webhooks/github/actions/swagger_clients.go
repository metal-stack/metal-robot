package actions

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/atedja/go-multilock"
	v3 "github.com/google/go-github/v32/github"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/git"
	filepatchers "github.com/metal-stack/metal-robot/pkg/webhooks/modifiers/file-patchers"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type swaggerParams struct {
	RepositoryName string
	TagName        string
}

type swaggerClient struct {
	logger                *zap.SugaredLogger
	client                *clients.Github
	commitMessageTemplate string
	branchTemplate        string
	repos                 map[string][]swaggerClientRepo
}

type swaggerClientRepo struct {
	patches []filepatchers.Patcher
	url     string
	name    string
}

func newSwaggerClient(logger *zap.SugaredLogger, client *clients.Github, rawConfig map[string]interface{}) (*swaggerClient, error) {
	var (
		commitMessageTemplate = "Bump to %s swagger spec version %s"
		branchTemplate        = "auto-generate/%s"
	)

	var typedConfig config.SwaggerClientsConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	if typedConfig.BranchTemplate != nil {
		branchTemplate = *typedConfig.BranchTemplate
	}
	if typedConfig.CommitMsgTemplate != nil {
		commitMessageTemplate = *typedConfig.CommitMsgTemplate
	}

	repos := make(map[string][]swaggerClientRepo)
	for n, clientRepos := range typedConfig.Repos {
		var cs []swaggerClientRepo
		for _, clientRepo := range clientRepos {
			patches := []filepatchers.Patcher{}
			for _, m := range clientRepo.Patches {
				patcher, err := filepatchers.InitPatcher(m)
				if err != nil {
					return nil, err
				}

				patches = append(patches, patcher)
			}

			cs = append(cs, swaggerClientRepo{
				url:     clientRepo.RepositoryURL,
				name:    clientRepo.RepositoryName,
				patches: patches,
			})
		}
		repos[n] = cs
	}

	return &swaggerClient{
		logger:                logger,
		client:                client,
		branchTemplate:        branchTemplate,
		commitMessageTemplate: commitMessageTemplate,
		repos:                 repos,
	}, nil
}

// GenerateSwaggerClients is triggered by repositories that release a swagger spec.
// It will create a PR for the client repositories of this repository.
func (s *swaggerClient) GenerateSwaggerClients(ctx context.Context, p *swaggerParams) error {
	clientRepos, ok := s.repos[p.RepositoryName]
	if !ok {
		s.logger.Debugw("skip creating swagger client update branches because not a swagger repo", "repo", p.RepositoryName, "release", p.TagName)
		return nil
	}

	tag := p.TagName
	if !strings.HasPrefix(tag, "v") {
		s.logger.Debugw("skip creating swagger client update branches because tag not starting with v", "repo", p.RepositoryName, "release", p.TagName)
		return nil
	}

	g, ctx := errgroup.WithContext(ctx)

	token, err := s.client.GitToken(ctx)
	if err != nil {
		return errors.Wrap(err, "error creating git token")
	}

	for _, clientRepo := range clientRepos {
		g.Go(func() error {
			repoURL, err := url.Parse(clientRepo.url)
			if err != nil {
				return err
			}
			repoURL.User = url.UserPassword("x-access-token", token)

			prBranch := fmt.Sprintf(s.branchTemplate, tag)

			// preventing concurrent git repo modifications
			var once sync.Once
			lock := multilock.New(clientRepo.name)
			lock.Lock()
			defer once.Do(func() { lock.Unlock() })

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

			for _, patch := range clientRepo.patches {
				err = patch.Apply(reader, writer, tag)
				if err != nil {
					return errors.Wrap(err, "error applying repo updates")
				}
			}

			commitMessage := fmt.Sprintf(s.commitMessageTemplate, p.RepositoryName, tag)
			hash, err := git.CommitAndPush(r, commitMessage)
			if err != nil {
				if err == git.NoChangesError {
					s.logger.Debugw("skip creating swagger client update branch because nothing changed", "repo", p.RepositoryName, "release", tag)
					return nil
				}
				return errors.Wrap(err, "error pushing release file")
			}

			s.logger.Infow("pushed to swagger client repo", "repo", p.RepositoryName, "release", tag, "branch", prBranch, "hash", hash)

			once.Do(func() { lock.Unlock() })

			pr, _, err := s.client.GetV3Client().PullRequests.Create(ctx, s.client.Organization(), clientRepo.name, &v3.NewPullRequest{
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
				s.logger.Infow("created pull request", "url", pr.GetURL())
			}

			return nil
		})
	}

	return nil
}
