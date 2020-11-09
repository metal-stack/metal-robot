package actions

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/atedja/go-multilock"
	"github.com/blang/semver"
	v3 "github.com/google/go-github/v32/github"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/git"
	filepatchers "github.com/metal-stack/metal-robot/pkg/webhooks/modifiers/file-patchers"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type AggregateReleases struct {
	logger                *zap.SugaredLogger
	client                *clients.Github
	branch                string
	commitMessageTemplate string
	patchMap              map[string][]filepatchers.Patcher
	repoURL               string
	repoName              string
	pullRequestTitle      string
}

type AggregateReleaseParams struct {
	RepositoryName string
	TagName        string
}

func NewAggregateReleases(logger *zap.SugaredLogger, client *clients.Github, rawConfig map[string]interface{}) (*AggregateReleases, error) {
	var (
		branch                = "develop"
		commitMessageTemplate = "Bump %s to version %s"
		pullRequestTitle      = "Next release"
	)

	var typedConfig config.AggregateReleasesConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	if typedConfig.TargetRepositoryName == "" {
		return nil, fmt.Errorf("target repository name must be specified")
	}
	if typedConfig.TargetRepositoryURL == "" {
		return nil, fmt.Errorf("target repository-url must be specified")
	}
	if typedConfig.Branch != nil {
		branch = *typedConfig.Branch
	}
	if typedConfig.CommitMsgTemplate != nil {
		commitMessageTemplate = *typedConfig.CommitMsgTemplate
	}
	if typedConfig.PullRequestTitle != nil {
		pullRequestTitle = *typedConfig.PullRequestTitle
	}

	patchMap := make(map[string][]filepatchers.Patcher)
	for n, modifiers := range typedConfig.SourceRepos {
		for _, m := range modifiers {
			patcher, err := filepatchers.InitPatcher(m)
			if err != nil {
				return nil, err
			}

			patches, ok := patchMap[n]
			if !ok {
				patches = []filepatchers.Patcher{}
			}
			patches = append(patches, patcher)
			patchMap[n] = patches
		}
	}

	return &AggregateReleases{
		logger:                logger,
		client:                client,
		branch:                branch,
		commitMessageTemplate: commitMessageTemplate,
		patchMap:              patchMap,
		repoURL:               typedConfig.TargetRepositoryURL,
		repoName:              typedConfig.TargetRepositoryName,
		pullRequestTitle:      pullRequestTitle,
	}, nil
}

// AggregateRelease applies the given actions after push and release trigger of a given list of source repositories to a target repository
func (r *AggregateReleases) AggregateRelease(ctx context.Context, p *AggregateReleaseParams) error {
	patches, ok := r.patchMap[p.RepositoryName]
	if !ok {
		r.logger.Debugw("skip applying release actions to aggregation repo, not in list of source repositories", "target-repo", r.repoName, "source-repo", p.RepositoryName, "tag", p.TagName)
		return nil
	}

	tag := p.TagName
	trimmed := strings.TrimPrefix(tag, "v")
	_, err := semver.Make(trimmed)
	if err != nil {
		r.logger.Infow("skip applying release actions to aggregation repo because not a valid semver release tag", "target-repo", r.repoName, "source-repo", p.RepositoryName, "tag", p.TagName)
		return nil
	}

	// preventing concurrent git repo modifications
	var once sync.Once
	lock := multilock.New(r.repoName)
	lock.Lock()
	defer once.Do(func() { lock.Unlock() })

	token, err := r.client.GitToken(ctx)
	if err != nil {
		return errors.Wrap(err, "error creating git token")
	}

	repoURL, err := url.Parse(r.repoURL)
	if err != nil {
		return err
	}
	repoURL.User = url.UserPassword("x-access-token", token)

	repository, err := git.ShallowClone(repoURL.String(), r.branch, 1)
	if err != nil {
		return err
	}

	reader := func(file string) ([]byte, error) {
		return git.ReadRepoFile(repository, file)
	}

	writer := func(file string, content []byte) error {
		return git.WriteRepoFile(repository, file, content)
	}

	for _, patch := range patches {
		err = patch.Apply(reader, writer, tag)
		if err != nil {
			return errors.Wrap(err, "error applying release updates")
		}
	}

	commitMessage := fmt.Sprintf(r.commitMessageTemplate, p.RepositoryName, tag)
	hash, err := git.CommitAndPush(repository, commitMessage)
	if err != nil {
		if err == git.NoChangesError {
			r.logger.Debugw("skip push to target repository because nothing changed", "target-repo", p.RepositoryName, "source-repo", p.RepositoryName, "release", tag)
			return nil
		}
		return errors.Wrap(err, "error pushing to target repository")
	}

	r.logger.Infow("pushed to aggregate target repo", "target-repo", p.RepositoryName, "source-repo", p.RepositoryName, "release", tag, "branch", r.branch, "hash", hash)

	once.Do(func() { lock.Unlock() })

	pr, _, err := r.client.GetV3Client().PullRequests.Create(ctx, r.client.Organization(), r.repoName, &v3.NewPullRequest{
		Title:               v3.String("Next release"),
		Head:                v3.String(r.branch),
		Base:                v3.String("master"),
		Body:                v3.String(r.pullRequestTitle),
		MaintainerCanModify: v3.Bool(true),
	})
	if err != nil {
		if !strings.Contains(err.Error(), "A pull request already exists") {
			return err
		}
	} else {
		r.logger.Infow("created pull request", "target-repo", p.RepositoryName, "url", pr.GetURL())
	}

	return nil
}