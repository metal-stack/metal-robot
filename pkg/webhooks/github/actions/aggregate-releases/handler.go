package aggregate_releases

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
	"github.com/google/go-github/v79/github"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/git"
	"github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/common"
	"github.com/metal-stack/metal-robot/pkg/webhooks/handlers"
	handlerrors "github.com/metal-stack/metal-robot/pkg/webhooks/handlers/errors"
	filepatchers "github.com/metal-stack/metal-robot/pkg/webhooks/modifiers/file-patchers"
	"github.com/mitchellh/mapstructure"
)

type aggregateReleases struct {
	client                *clients.Github
	branch                string
	branchBase            string
	commitMessageTemplate string
	patchMap              map[string][]filepatchers.Patcher
	repoURL               string
	repoName              string
	pullRequestTitle      string

	lock *multilock.Lock
}

type Params struct {
	RepositoryName string
	RepositoryURL  string
	TagName        string
	Sender         string
}

func New(client *clients.Github, rawConfig map[string]any) (handlers.WebhookHandler[*Params], error) {
	var (
		branch                = "develop"
		branchBase            = "master"
		commitMessageTemplate = "Bump %s to version %s"
		pullRequestTitle      = "Next release"
	)

	var typedConfig config.AggregateReleasesConfig
	err := mapstructure.Decode(rawConfig, &typedConfig) // nolint:musttag
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
	if typedConfig.BranchBase != nil && *typedConfig.BranchBase != "" {
		branchBase = *typedConfig.BranchBase
	}
	if typedConfig.CommitMsgTemplate != nil {
		commitMessageTemplate = *typedConfig.CommitMsgTemplate
	}
	if typedConfig.PullRequestTitle != nil && *typedConfig.PullRequestTitle != "" {
		pullRequestTitle = *typedConfig.PullRequestTitle
	}

	patchMap := make(map[string][]filepatchers.Patcher)
	for n, actions := range typedConfig.SourceRepos {
		for _, m := range actions.Modifiers {
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

	return &aggregateReleases{
		client:                client,
		branch:                branch,
		branchBase:            branchBase,
		commitMessageTemplate: commitMessageTemplate,
		patchMap:              patchMap,
		repoURL:               typedConfig.TargetRepositoryURL,
		repoName:              typedConfig.TargetRepositoryName,
		pullRequestTitle:      pullRequestTitle,
		lock:                  multilock.New(typedConfig.TargetRepositoryName),
	}, nil
}

// Handle adds a repository release to a release vector repository.
func (r *aggregateReleases) Handle(ctx context.Context, log *slog.Logger, p *Params) error {
	log = log.With("target-repo", r.repoName, "tag", p.TagName)

	patches, ok := r.patchMap[p.RepositoryName]
	if !ok {
		return handlerrors.Skip("not adding to release vector because repository is not configured as a release repository in the metal-robot configuration")
	}

	var (
		tag     = p.TagName
		trimmed = strings.TrimPrefix(tag, "v")
	)

	_, err := semver.NewVersion(trimmed)
	if err != nil {
		return handlerrors.Skip("not adding to release vector because not a valid semver release tag: %w", err)
	}

	openPR, err := common.FindOpenReleasePR(ctx, r.client.GetV3Client(), r.client.Organization(), r.repoName, r.branch, r.branchBase)
	if err != nil {
		return fmt.Errorf("unable to find open release pull requests: %w", err)
	}

	if openPR != nil {
		frozen, err := common.IsReleaseFreeze(ctx, r.client.GetV3Client(), *openPR.Number, r.client.Organization(), r.repoName)
		if err != nil {
			return fmt.Errorf("unable to find out if release is frozen: %w", err)
		}

		if frozen {
			log.Info("not adding to release vector because release is currently frozen")

			_, _, err = r.client.GetV3Client().Issues.CreateComment(ctx, r.client.Organization(), r.repoName, *openPR.Number, &github.IssueComment{
				Body: new(fmt.Sprintf(":warning: Release `%v` in repository %s (issued by @%s) was rejected because release is currently frozen. Please re-issue the release hook once this branch was merged or unfrozen.",
					p.TagName,
					p.RepositoryURL,
					p.Sender,
				)),
			})
			if err != nil {
				return fmt.Errorf("unable to create comment for rejected release aggregation: %w", err)
			}

			return nil
		}
	}

	// preventing concurrent git repo modifications
	var once sync.Once
	r.lock.Lock()
	defer once.Do(func() { r.lock.Unlock() })

	token, err := r.client.GitToken(ctx)
	if err != nil {
		return fmt.Errorf("error creating git token: %w", err)
	}

	repoURL, err := url.Parse(r.repoURL)
	if err != nil {
		return fmt.Errorf("unable to parse repository url: %w", err)
	}
	repoURL.User = url.UserPassword("x-access-token", token)

	repository, err := git.ShallowClone(repoURL.String(), r.branch, 1)
	if err != nil {
		return fmt.Errorf("unable to shallow clone repository: %w", err)
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
			return fmt.Errorf("error applying release updates: %w", err)
		}
	}

	commitMessage := fmt.Sprintf(r.commitMessageTemplate, p.RepositoryName, tag)
	hash, err := git.CommitAndPush(repository, commitMessage)
	if err != nil {
		if errors.Is(err, git.ErrNoChanges) {
			log.Debug("skip push to target repository because nothing changed")
		} else {
			return fmt.Errorf("error pushing to target repository: %w", err)
		}
	} else {
		log.Info("pushed to aggregate target repo", "branch", r.branch, "hash", hash)

		once.Do(func() { r.lock.Unlock() })
	}

	pr, _, err := r.client.GetV3Client().PullRequests.Create(ctx, r.client.Organization(), r.repoName, &github.NewPullRequest{
		Title:               new("Next release"),
		Head:                new(r.branch),
		Base:                new(r.branchBase),
		Body:                new(r.pullRequestTitle),
		MaintainerCanModify: new(true),
	})
	if err != nil {
		if !strings.Contains(err.Error(), "A pull request already exists") {
			return fmt.Errorf("unable to create pull request: %w", err)
		}
	} else {
		log.Info("created pull request", "url", pr.GetURL())
	}

	return nil
}
