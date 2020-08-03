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

type releaseVector struct {
	logger                *zap.SugaredLogger
	client                *clients.Github
	branch                string
	commitMessageTemplate string
	patchMap              map[string][]filepatchers.Patcher
	repoURL               string
	repoName              string
}

type releaseVectorParams struct {
	RepositoryName string
	TagName        string
}

func newReleaseVector(logger *zap.SugaredLogger, client *clients.Github, rawConfig map[string]interface{}) (*releaseVector, error) {
	var (
		branch                = "develop"
		commitMessageTemplate = "Bump %s to version %s"
	)

	var typedConfig config.ReleaseVectorConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	if typedConfig.RepositoryName == "" {
		return nil, fmt.Errorf("repository must be specified")
	}
	if typedConfig.RepositoryURL == "" {
		return nil, fmt.Errorf("repository-url must be specified")
	}
	if typedConfig.Branch != nil {
		branch = *typedConfig.Branch
	}
	if typedConfig.CommitMsgTemplate != nil {
		commitMessageTemplate = *typedConfig.CommitMsgTemplate
	}

	patchMap := make(map[string][]filepatchers.Patcher)
	for n, modifiers := range typedConfig.Repos {
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

	return &releaseVector{
		logger:                logger,
		client:                client,
		branch:                branch,
		commitMessageTemplate: commitMessageTemplate,
		patchMap:              patchMap,
		repoURL:               typedConfig.RepositoryURL,
		repoName:              typedConfig.RepositoryName,
	}, nil
}

// AddToRelaseVector adds a release to the release vector in a release repository
func (r *releaseVector) AddToRelaseVector(ctx context.Context, p *releaseVectorParams) error {
	patches, ok := r.patchMap[p.RepositoryName]
	if !ok {
		r.logger.Debugw("skip adding new version to release vector because not a release vector repo", "repo", p.RepositoryName, "release", p.TagName)
		return nil
	}
	tag := p.TagName
	if !strings.HasPrefix(tag, "v") {
		r.logger.Debugw("skip adding new version to release vector because not starting with v", "repo", p.RepositoryName, "release", tag)
		return nil
	}

	// preventing concurrent git repo modifications
	var once sync.Once
	lock := multilock.New(r.repoName)
	lock.Lock()
	defer once.Do(func() { lock.Unlock() })

	version, err := semver.Make(tag[1:])
	if err != nil {
		return errors.Wrap(err, "not a valid semver release tag")
	}

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
		err = patch.Apply(reader, writer, version.String())
		if err != nil {
			return errors.Wrap(err, "error applying release updates")
		}
	}

	commitMessage := fmt.Sprintf(r.commitMessageTemplate, p.RepositoryName, tag)
	hash, err := git.CommitAndPush(repository, commitMessage)
	if err != nil {
		if err == git.NoChangesError {
			r.logger.Debugw("skip adding new version to release vector because nothing changed", "repo", p.RepositoryName, "release", tag)
			return nil
		}
		return errors.Wrap(err, "error pushing release file")
	}

	r.logger.Infow("pushed to release repo", "repo", r.repoName, "release", tag, "branch", r.branch, "hash", hash)

	once.Do(func() { lock.Unlock() })

	pr, _, err := r.client.GetV3Client().PullRequests.Create(ctx, r.client.Organization(), r.repoName, &v3.NewPullRequest{
		Title:               v3.String("Next release"),
		Head:                v3.String(r.branch),
		Base:                v3.String("master"),
		Body:                v3.String("Next release of metal-stack"),
		MaintainerCanModify: v3.Bool(true),
	})
	if err != nil {
		if !strings.Contains(err.Error(), "A pull request already exists") {
			return err
		}
	} else {
		r.logger.Infow("created pull request", "url", pr.GetURL())
	}

	return nil
}
