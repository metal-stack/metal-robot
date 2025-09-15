package distribute_releases

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"

	"errors"

	"github.com/Masterminds/semver/v3"
	"github.com/atedja/go-multilock"
	"github.com/google/go-github/v74/github"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/git"
	"github.com/metal-stack/metal-robot/pkg/webhooks/github/actions"
	filepatchers "github.com/metal-stack/metal-robot/pkg/webhooks/modifiers/file-patchers"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/sync/errgroup"
)

type Params struct {
	RepositoryName string
	TagName        string
}

type distributeReleases struct {
	client                *clients.Github
	commitMessageTemplate string
	branchTemplate        string
	repoURL               string
	repoName              string
	targetRepos           map[string]targetRepo
	pullRequestTitle      string
}

type targetRepo struct {
	patches []filepatchers.Patcher
	branch  string
	url     string
}

func New(client *clients.Github, rawConfig map[string]any) (actions.WebhookHandler[*Params], error) {
	var (
		commitMessageTemplate = "Bump %s to version %s"
		branchTemplate        = "auto-generate/%s"
		pullRequestTitle      = "Bump version"
	)

	var typedConfig config.DistributeReleasesConfig
	err := mapstructure.Decode(rawConfig, &typedConfig) // nolint:musttag
	if err != nil {
		return nil, err
	}

	if typedConfig.SourceRepositoryName == "" {
		return nil, fmt.Errorf("source repository name must be specified")
	}
	if typedConfig.SourceRepositoryURL == "" {
		return nil, fmt.Errorf("source repository-url must be specified")
	}
	if typedConfig.BranchTemplate != nil && *typedConfig.BranchTemplate != "" {
		branchTemplate = *typedConfig.BranchTemplate
	}
	if typedConfig.CommitMsgTemplate != nil {
		commitMessageTemplate = *typedConfig.CommitMsgTemplate
	}
	if typedConfig.PullRequestTitle != nil && *typedConfig.PullRequestTitle != "" {
		pullRequestTitle = *typedConfig.PullRequestTitle
	}

	targetRepos := make(map[string]targetRepo)
	for _, t := range typedConfig.TargetRepos {
		patches := []filepatchers.Patcher{}
		for _, m := range t.Patches {
			patcher, err := filepatchers.InitPatcher(m)
			if err != nil {
				return nil, err
			}

			patches = append(patches, patcher)
		}

		branch := "master"
		if t.Branch != "" {
			branch = t.Branch
		}

		targetRepos[t.RepositoryName] = targetRepo{
			url:     t.RepositoryURL,
			branch:  branch,
			patches: patches,
		}
	}

	return &distributeReleases{
		client:                client,
		branchTemplate:        branchTemplate,
		commitMessageTemplate: commitMessageTemplate,
		repoURL:               typedConfig.SourceRepositoryURL,
		repoName:              typedConfig.SourceRepositoryName,
		targetRepos:           targetRepos,
		pullRequestTitle:      pullRequestTitle,
	}, nil
}

// Handle can apply file patches in other repositories, when a repository creates a release
func (d *distributeReleases) Handle(ctx context.Context, log *slog.Logger, p *Params) error {
	if p.RepositoryName != d.repoName {
		return nil
	}

	var (
		tag = p.TagName
	)

	log = log.With("tag", p.TagName)

	if !strings.HasPrefix(tag, "v") {
		log.Debug("skip distribute release action because release tag not starting with v")
		return nil
	}

	trimmed := strings.TrimPrefix(tag, "v")

	parsedVersion, err := semver.NewVersion(trimmed)
	if err != nil {
		log.Info("skip distribute release action because not a valid semver release tag")
		return nil //nolint:nilerr
	}

	if parsedVersion.Prerelease() != "" {
		log.Info("skip distribute release action because is a pre-release")
		return nil //nolint:nilerr
	}

	token, err := d.client.GitToken(ctx)
	if err != nil {
		return fmt.Errorf("error creating git token: %w", err)
	}

	var targetRepos []string
	for targetRepoName := range d.targetRepos {
		targetRepos = append(targetRepos, targetRepoName)
	}
	lock := multilock.New(targetRepos...)

	g, _ := errgroup.WithContext(ctx)

	for targetRepoName, targetRepo := range d.targetRepos {
		g.Go(func() error {
			log = log.With("target-repo", targetRepoName)

			log.Info("applying patch actions")

			repoURL, err := url.Parse(targetRepo.url)
			if err != nil {
				return err
			}
			repoURL.User = url.UserPassword("x-access-token", token)

			prBranch := fmt.Sprintf(d.branchTemplate, tag)

			// preventing concurrent git repo modifications
			var once sync.Once
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

			for _, patch := range targetRepo.patches {
				err = patch.Apply(reader, writer, tag)
				if err != nil {
					return fmt.Errorf("error applying repo updates: %w", err)
				}
			}

			commitMessage := fmt.Sprintf(d.commitMessageTemplate, p.RepositoryName, tag)
			hash, err := git.CommitAndPush(r, commitMessage)
			if err != nil {
				if errors.Is(err, git.ErrNoChanges) {
					log.Debug("skip pushing to target repo because nothing changed")
					return nil
				}
				return fmt.Errorf("error applying release updates %w", err)
			}

			log.Info("pushed to target repo", "branch", prBranch, "hash", hash)

			once.Do(func() { lock.Unlock() })

			pr, _, err := d.client.GetV3Client().PullRequests.Create(ctx, d.client.Organization(), targetRepoName, &github.NewPullRequest{
				Title:               github.Ptr(commitMessage),
				Head:                github.Ptr(prBranch),
				Base:                github.Ptr(targetRepo.branch),
				Body:                github.Ptr(d.pullRequestTitle),
				MaintainerCanModify: github.Ptr(true),
			})
			if err != nil {
				if !strings.Contains(err.Error(), "A pull request already exists") {
					return err
				}
			} else {
				log.Info("created pull request for target repo", "url", pr.GetURL())
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		log.Error("errors occurred while applying release actions to target repos", "error", err)
	}

	return nil
}
