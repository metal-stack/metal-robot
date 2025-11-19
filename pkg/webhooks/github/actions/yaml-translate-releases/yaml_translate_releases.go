package yaml_translate_releases

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
	"github.com/metal-stack/metal-robot/pkg/webhooks/github/actions"
	filepatchers "github.com/metal-stack/metal-robot/pkg/webhooks/modifiers/file-patchers"
	"github.com/mitchellh/mapstructure"
)

type yamlTranslateReleases struct {
	logger                *slog.Logger
	client                *clients.Github
	branch                string
	branchBase            string
	commitMessageTemplate string
	translationMap        map[string][]yamlTranslation
	repoURL               string
	repoName              string
	pullRequestTitle      string

	lock *multilock.Lock
}

type yamlTranslation struct {
	from yamlFrom
	to   []filepatchers.Patcher
}

type yamlFrom struct {
	file     string
	yamlPath string
}

type Params struct {
	RepositoryName string
	RepositoryURL  string
	TagName        string
}

func New(logger *slog.Logger, client *clients.Github, rawConfig map[string]any) (actions.WebhookHandler[*Params], error) {
	var (
		branch                = "develop"
		branchBase            = "master"
		commitMessageTemplate = "Bump %s to version %s"
		pullRequestTitle      = "Next release"
	)

	var typedConfig config.YAMLTranslateReleasesConfig
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
	if typedConfig.BranchBase != nil {
		branchBase = *typedConfig.BranchBase
	}
	if typedConfig.CommitMsgTemplate != nil {
		commitMessageTemplate = *typedConfig.CommitMsgTemplate
	}
	if typedConfig.PullRequestTitle != nil {
		pullRequestTitle = *typedConfig.PullRequestTitle
	}

	translationMap := make(map[string][]yamlTranslation)
	for n, translations := range typedConfig.SourceRepos {
		for _, t := range translations {
			from := yamlFrom{
				file:     t.From.File,
				yamlPath: t.From.YAMLPath,
			}

			yt := yamlTranslation{
				from: from,
				to:   []filepatchers.Patcher{},
			}

			for _, m := range t.To {
				to, err := filepatchers.InitPatcher(m)
				if err != nil {
					return nil, err
				}
				yt.to = append(yt.to, to)
			}

			ts, ok := translationMap[n]
			if !ok {
				ts = []yamlTranslation{}
			}
			ts = append(ts, yt)
			translationMap[n] = ts
		}
	}

	return &yamlTranslateReleases{
		logger:                logger,
		client:                client,
		branch:                branch,
		branchBase:            branchBase,
		commitMessageTemplate: commitMessageTemplate,
		translationMap:        translationMap,
		repoURL:               typedConfig.TargetRepositoryURL,
		repoName:              typedConfig.TargetRepositoryName,
		pullRequestTitle:      pullRequestTitle,
		lock:                  multilock.New(typedConfig.TargetRepositoryName),
	}, nil
}

// TranslateRelease translates contents from one repository to another repository
func (r *yamlTranslateReleases) Handle(ctx context.Context, p *Params) error {
	translations, ok := r.translationMap[p.RepositoryName]
	if !ok {
		r.logger.Debug("skip applying translate release actions to aggregation repo, not in list of source repositories", "target-repo", r.repoName, "source-repo", p.RepositoryName, "tag", p.TagName)
		return nil
	}

	tag := p.TagName
	trimmed := strings.TrimPrefix(tag, "v")
	_, err := semver.NewVersion(trimmed)
	if err != nil {
		r.logger.Info("skip applying translate release actions to aggregation repo because not a valid semver release tag", "target-repo", r.repoName, "source-repo", p.RepositoryName, "tag", p.TagName)
		return nil //nolint:nilerr
	}

	// preventing concurrent git repo modifications
	var once sync.Once
	r.lock.Lock()
	defer once.Do(func() { r.lock.Unlock() })

	token, err := r.client.GitToken(ctx)
	if err != nil {
		return fmt.Errorf("error creating git token %w", err)
	}

	sourceRepoURL, err := url.Parse(p.RepositoryURL)
	if err != nil {
		return err
	}
	sourceRepoURL.User = url.UserPassword("x-access-token", token)

	sourceRepository, err := git.ShallowClone(sourceRepoURL.String(), r.branch, 1)
	if err != nil {
		return err
	}

	targetRepoURL, err := url.Parse(r.repoURL)
	if err != nil {
		return err
	}
	targetRepoURL.User = url.UserPassword("x-access-token", token)

	targetRepository, err := git.ShallowClone(targetRepoURL.String(), r.branch, 1)
	if err != nil {
		return err
	}

	reader := func(file string) ([]byte, error) {
		return git.ReadRepoFile(targetRepository, file)
	}

	writer := func(file string, content []byte) error {
		return git.WriteRepoFile(targetRepository, file, content)
	}

	for _, translation := range translations {
		content, err := git.ReadRepoFile(sourceRepository, translation.from.file)
		if err != nil {
			return fmt.Errorf("error reading content from source repository file %w", err)
		}

		value, err := filepatchers.GetYAML(content, translation.from.yamlPath)
		if err != nil {
			return fmt.Errorf("error reading value from source repository file %w", err)
		}

		for _, patch := range translation.to {
			err = patch.Apply(reader, writer, value)
			if err != nil {
				return fmt.Errorf("error applying translate updates %w", err)
			}
		}
	}

	commitMessage := fmt.Sprintf(r.commitMessageTemplate, p.RepositoryName, tag)
	hash, err := git.CommitAndPush(targetRepository, commitMessage)
	if err != nil {
		if errors.Is(err, git.ErrNoChanges) {
			r.logger.Debug("skip push to target repository because nothing changed", "target-repo", p.RepositoryName, "source-repo", p.RepositoryName, "release", tag)
			return nil
		}
		return fmt.Errorf("error pushing to target repository %w", err)
	}

	r.logger.Info("pushed to translate target repo", "target-repo", p.RepositoryName, "source-repo", p.RepositoryName, "release", tag, "branch", r.branch, "hash", hash)

	once.Do(func() { r.lock.Unlock() })

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
		r.logger.Info("created pull request", "target-repo", p.RepositoryName, "url", pr.GetURL())
	}

	return nil
}
