package actions

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"errors"

	"github.com/Masterminds/semver/v3"
	"github.com/atedja/go-multilock"
	v3 "github.com/google/go-github/v57/github"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/git"
	filepatchers "github.com/metal-stack/metal-robot/pkg/webhooks/modifiers/file-patchers"
	"github.com/mitchellh/mapstructure"
	"go.uber.org/zap"
)

type AggregateReleases struct {
	logger                *zap.SugaredLogger
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

type AggregateReleaseParams struct {
	RepositoryName string
	TagName        string
	Sender         string
}

func NewAggregateReleases(logger *zap.SugaredLogger, client *clients.Github, rawConfig map[string]any) (*AggregateReleases, error) {
	var (
		branch                = "develop"
		branchBase            = "master"
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
		branchBase:            branchBase,
		commitMessageTemplate: commitMessageTemplate,
		patchMap:              patchMap,
		repoURL:               typedConfig.TargetRepositoryURL,
		repoName:              typedConfig.TargetRepositoryName,
		pullRequestTitle:      pullRequestTitle,
		lock:                  multilock.New(typedConfig.TargetRepositoryName),
	}, nil
}

// AggregateRelease applies the given actions after push and release trigger of a given list of source repositories to a target repository
func (r *AggregateReleases) AggregateRelease(ctx context.Context, p *AggregateReleaseParams) error {
	log := r.logger.With("target-repo", r.repoName, "source-repo", p.RepositoryName, "tag", p.TagName)

	patches, ok := r.patchMap[p.RepositoryName]
	if !ok {
		log.Debugw("skip applying release actions to aggregation repo, not in list of source repositories")
		return nil
	}

	tag := p.TagName
	trimmed := strings.TrimPrefix(tag, "v")
	_, err := semver.NewVersion(trimmed)
	if err != nil {
		log.Infow("skip applying release actions to aggregation repo because not a valid semver release tag", "error", err)
		return nil
	}

	openPR, err := findOpenReleasePR(ctx, r.client, r.client.Organization(), r.repoName, r.branch, r.branchBase)
	if err != nil {
		return err
	}

	if openPR != nil {
		frozen, err := isReleaseFreeze(ctx, r.client, openPR)
		if err != nil {
			return err
		}

		if frozen {
			log.Infow("skip applying release actions to aggregation repo because release is currently frozen")

			_, _, err = r.client.GetV3AppClient().PullRequests.CreateComment(ctx, r.client.Organization(), r.repoName, *openPR.Number, &v3.PullRequestComment{
				Body: v3.String(fmt.Sprintf("Release `%v` in repository %q (issued by @%s) was rejected because release is currently frozen. Please re-issue the release hook once this branch was merged or unfrozen.",
					p.TagName,
					p.RepositoryName,
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
		return fmt.Errorf("error creating git token %w", err)
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
			return fmt.Errorf("error applying release updates %w", err)
		}
	}

	commitMessage := fmt.Sprintf(r.commitMessageTemplate, p.RepositoryName, tag)
	hash, err := git.CommitAndPush(repository, commitMessage)
	if err != nil {
		if errors.Is(err, git.NoChangesError) {
			log.Debugw("skip push to target repository because nothing changed")
		} else {
			return fmt.Errorf("error pushing to target repository %w", err)
		}
	} else {
		log.Infow("pushed to aggregate target repo", "branch", r.branch, "hash", hash)

		once.Do(func() { r.lock.Unlock() })
	}

	pr, _, err := r.client.GetV3Client().PullRequests.Create(ctx, r.client.Organization(), r.repoName, &v3.NewPullRequest{
		Title:               v3.String("Next release"),
		Head:                v3.String(r.branch),
		Base:                v3.String(r.branchBase),
		Body:                v3.String(r.pullRequestTitle),
		MaintainerCanModify: v3.Bool(true),
	})
	if err != nil {
		if !strings.Contains(err.Error(), "A pull request already exists") {
			return err
		}
	} else {
		log.Infow("created pull request", "url", pr.GetURL())
	}

	return nil
}

func findOpenReleasePR(ctx context.Context, client *clients.Github, owner, repo, branch, base string) (*v3.PullRequest, error) {
	prs, _, err := client.GetV3AppClient().PullRequests.List(ctx, owner, repo, &v3.PullRequestListOptions{
		State: "open",
		Head:  branch,
		Base:  base,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to list pull requests: %w", err)
	}

	if len(prs) == 1 {
		return prs[0], nil
	}

	return nil, nil
}

func isReleaseFreeze(ctx context.Context, client *clients.Github, pr *v3.PullRequest) (bool, error) {
	comments, _, err := client.GetV3AppClient().PullRequests.ListComments(ctx, *pr.Base.Repo.Owner.Name, *pr.Base.Repo.Name, pointer.SafeDeref(pr.Number), &v3.PullRequestListCommentsOptions{
		Direction: "desc",
	})
	if err != nil {
		return true, fmt.Errorf("unable to list pull request comments: %w", err)
	}

	for _, comment := range comments {
		comment := comment

		if ok := searchForCommandInComment(pointer.SafeDeref(comment.Body), IssueCommentReleaseFreeze); ok {
			return true, nil
		}

		if ok := searchForCommandInComment(pointer.SafeDeref(comment.Body), IssueCommentReleaseUnfreeze); ok {
			return false, nil
		}
	}

	return false, nil
}
