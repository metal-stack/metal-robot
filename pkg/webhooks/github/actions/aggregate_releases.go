package actions

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"sort"
	"strings"
	"sync"

	"errors"

	"github.com/Masterminds/semver/v3"
	"github.com/atedja/go-multilock"
	"github.com/google/go-github/v72/github"
	"github.com/metal-stack/metal-lib/pkg/pointer"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/git"
	filepatchers "github.com/metal-stack/metal-robot/pkg/webhooks/modifiers/file-patchers"
	"github.com/mitchellh/mapstructure"
)

type AggregateReleases struct {
	logger                *slog.Logger
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
	RepositoryURL  string
	TagName        string
	Sender         string
}

func NewAggregateReleases(logger *slog.Logger, client *clients.Github, rawConfig map[string]any) (*AggregateReleases, error) {
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
		log.Debug("skip applying release actions to aggregation repo, not in list of source repositories")
		return nil
	}

	tag := p.TagName
	trimmed := strings.TrimPrefix(tag, "v")
	_, err := semver.NewVersion(trimmed)
	if err != nil {
		log.Info("skip applying release actions to aggregation repo because not a valid semver release tag", "error", err)
		return nil
	}

	openPR, err := findOpenReleasePR(ctx, r.client.GetV3Client(), r.client.Organization(), r.repoName, r.branch, r.branchBase)
	if err != nil {
		return err
	}

	if openPR != nil {
		frozen, err := isReleaseFreeze(ctx, r.client.GetV3Client(), *openPR.Number, r.client.Organization(), r.repoName)
		if err != nil {
			return err
		}

		if frozen {
			log.Info("skip applying release actions to aggregation repo because release is currently frozen")

			_, _, err = r.client.GetV3Client().Issues.CreateComment(ctx, r.client.Organization(), r.repoName, *openPR.Number, &github.IssueComment{
				Body: github.Ptr(fmt.Sprintf(":warning: Release `%v` in repository %s (issued by @%s) was rejected because release is currently frozen. Please re-issue the release hook once this branch was merged or unfrozen.",
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
		if errors.Is(err, git.ErrNoChanges) {
			log.Debug("skip push to target repository because nothing changed")
		} else {
			return fmt.Errorf("error pushing to target repository %w", err)
		}
	} else {
		log.Info("pushed to aggregate target repo", "branch", r.branch, "hash", hash)

		once.Do(func() { r.lock.Unlock() })
	}

	pr, _, err := r.client.GetV3Client().PullRequests.Create(ctx, r.client.Organization(), r.repoName, &github.NewPullRequest{
		Title:               github.Ptr("Next release"),
		Head:                github.Ptr(r.branch),
		Base:                github.Ptr(r.branchBase),
		Body:                github.Ptr(r.pullRequestTitle),
		MaintainerCanModify: github.Ptr(true),
	})
	if err != nil {
		if !strings.Contains(err.Error(), "A pull request already exists") {
			return err
		}
	} else {
		log.Info("created pull request", "url", pr.GetURL())
	}

	return nil
}

func findOpenReleasePR(ctx context.Context, client *github.Client, owner, repo, branch, base string) (*github.PullRequest, error) {
	prs, _, err := client.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{
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

func isReleaseFreeze(ctx context.Context, client *github.Client, number int, owner, repo string) (bool, error) {
	comments, _, err := client.Issues.ListComments(ctx, owner, repo, number, &github.IssueListCommentsOptions{
		Direction: github.Ptr("desc"),
	})
	if err != nil {
		return true, fmt.Errorf("unable to list pull request comments: %w", err)
	}

	// somehow the direction parameter has no effect, it's always sorted in the same way?
	// therefore sorting manually:
	sort.Slice(comments, sortComments(comments))

	for _, comment := range comments {
		comment := comment

		if _, ok := searchForCommandInBody(pointer.SafeDeref(comment.Body), IssueCommentReleaseFreeze); ok {
			return true, nil
		}

		if _, ok := searchForCommandInBody(pointer.SafeDeref(comment.Body), IssueCommentReleaseUnfreeze); ok {
			return false, nil
		}
	}

	return false, nil
}

func sortComments(comments []*github.IssueComment) func(i, j int) bool {
	return func(i, j int) bool {
		return comments[j].CreatedAt.Before(comments[i].CreatedAt.Time)
	}
}
