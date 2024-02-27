package actions

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
	v3 "github.com/google/go-github/v57/github"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/git"
	filepatchers "github.com/metal-stack/metal-robot/pkg/webhooks/modifiers/file-patchers"
	"github.com/mitchellh/mapstructure"
	"golang.org/x/sync/errgroup"
)

type distributeReleaseParams struct {
	RepositoryName string
	TagName        string
}

type distributeReleases struct {
	logger                *slog.Logger
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
	url     string
}

func newDistributeReleases(logger *slog.Logger, client *clients.Github, rawConfig map[string]any) (*distributeReleases, error) {
	var (
		commitMessageTemplate = "Bump %s to version %s"
		branchTemplate        = "auto-generate/%s"
		pullRequestTitle      = "Bump version"
	)

	var typedConfig config.DistributeReleasesConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
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

		targetRepos[t.RepositoryName] = targetRepo{
			url:     t.RepositoryURL,
			patches: patches,
		}
	}

	return &distributeReleases{
		logger:                logger,
		client:                client,
		branchTemplate:        branchTemplate,
		commitMessageTemplate: commitMessageTemplate,
		repoURL:               typedConfig.SourceRepositoryURL,
		repoName:              typedConfig.SourceRepositoryName,
		targetRepos:           targetRepos,
		pullRequestTitle:      pullRequestTitle,
	}, nil
}

// DistributeRelease applies the actions to a given list of target repositories after a push or release trigger on the source repository
func (d *distributeReleases) DistributeRelease(ctx context.Context, p *distributeReleaseParams) error {
	if p.RepositoryName != d.repoName {
		d.logger.Debug("skip applying release actions to target repos, not triggered by source repo", "source-repo", d.repoName, "trigger-repo", p.RepositoryName, "tag", p.TagName)
		return nil
	}

	tag := p.TagName
	if !strings.HasPrefix(tag, "v") {
		d.logger.Debug("skip applying release actions to target repos because release tag not starting with v", "source-repo", p.RepositoryName, "tag", p.TagName)
		return nil
	}

	trimmed := strings.TrimPrefix(tag, "v")

	parsedVersion, err := semver.NewVersion(trimmed)
	if err != nil {
		d.logger.Info("skip applying release actions to target repos because not a valid semver release tag", "source-repo", d.repoName, "trigger-repo", p.RepositoryName, "tag", p.TagName)
		return nil //nolint:nilerr
	}

	if parsedVersion.Prerelease() != "" {
		d.logger.Info("skip applying release actions to target repos because is a pre-release", "source-repo", d.repoName, "trigger-repo", p.RepositoryName, "tag", p.TagName)
		return nil //nolint:nilerr
	}

	token, err := d.client.GitToken(ctx)
	if err != nil {
		return fmt.Errorf("error creating git token %w", err)
	}

	var targetRepos []string
	for targetRepoName := range d.targetRepos {
		targetRepos = append(targetRepos, targetRepoName)
	}
	lock := multilock.New(targetRepos...)

	g, _ := errgroup.WithContext(ctx)
	for targetRepoName, targetRepo := range d.targetRepos {
		targetRepoName := targetRepoName
		targetRepo := targetRepo
		g.Go(func() error {
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
					return fmt.Errorf("error applying repo updates %w", err)
				}
			}

			commitMessage := fmt.Sprintf(d.commitMessageTemplate, p.RepositoryName, tag)
			hash, err := git.CommitAndPush(r, commitMessage)
			if err != nil {
				if errors.Is(err, git.NoChangesError) {
					d.logger.Debug("skip applying release actions to target repo because nothing changed", "source-repo", p.RepositoryName, "target-repo", targetRepoName, "tag", p.TagName)
					return nil
				}
				return fmt.Errorf("error applying release updates %w", err)
			}

			d.logger.Info("pushed to target repo", "source-repo", p.RepositoryName, "target-repo", targetRepoName, "release", tag, "branch", prBranch, "hash", hash)

			once.Do(func() { lock.Unlock() })

			pr, _, err := d.client.GetV3Client().PullRequests.Create(ctx, d.client.Organization(), targetRepoName, &v3.NewPullRequest{
				Title:               v3.String(commitMessage),
				Head:                v3.String(prBranch),
				Base:                v3.String("master"),
				Body:                v3.String(d.pullRequestTitle),
				MaintainerCanModify: v3.Bool(true),
			})
			if err != nil {
				if !strings.Contains(err.Error(), "A pull request already exists") {
					return err
				}
			} else {
				d.logger.Info("created pull request for target repo", "source-repo", p.RepositoryName, "target-repo", targetRepoName, "release", tag, "url", pr.GetURL())
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		d.logger.Error("errors occurred while applying release actions to target repos", "error", err)
	}

	return nil
}
