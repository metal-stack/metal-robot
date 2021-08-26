package actions

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/atedja/go-multilock"
	"github.com/blang/semver"
	v3 "github.com/google/go-github/v37/github"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/git"
	filepatchers "github.com/metal-stack/metal-robot/pkg/webhooks/modifiers/file-patchers"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type yamlTranslateReleases struct {
	logger                *zap.SugaredLogger
	client                *clients.Github
	branch                string
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

type yamlTranslateReleaseParams struct {
	RepositoryName string
	RepositoryURL  string
	TagName        string
}

func newYAMLTranslateReleases(logger *zap.SugaredLogger, client *clients.Github, rawConfig map[string]interface{}) (*yamlTranslateReleases, error) {
	var (
		branch                = "develop"
		commitMessageTemplate = "Bump %s to version %s"
		pullRequestTitle      = "Next release"
	)

	var typedConfig config.YAMLTranslateReleasesConfig
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
					fmt.Println("1")
					return nil, err
				}
				yt.to = append(yt.to, to)
			}
			fmt.Println("2")

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
		commitMessageTemplate: commitMessageTemplate,
		translationMap:        translationMap,
		repoURL:               typedConfig.TargetRepositoryURL,
		repoName:              typedConfig.TargetRepositoryName,
		pullRequestTitle:      pullRequestTitle,
		lock:                  multilock.New(typedConfig.TargetRepositoryName),
	}, nil
}

// TranslateRelease translates contents from one repository to another repository
func (r *yamlTranslateReleases) translateRelease(ctx context.Context, p *yamlTranslateReleaseParams) error {
	translations, ok := r.translationMap[p.RepositoryName]
	if !ok {
		r.logger.Debugw("skip applying translate release actions to aggregation repo, not in list of source repositories", "target-repo", r.repoName, "source-repo", p.RepositoryName, "tag", p.TagName)
		return nil
	}

	tag := p.TagName
	trimmed := strings.TrimPrefix(tag, "v")
	_, err := semver.Make(trimmed)
	if err != nil {
		r.logger.Infow("skip applying translate release actions to aggregation repo because not a valid semver release tag", "target-repo", r.repoName, "source-repo", p.RepositoryName, "tag", p.TagName)
		return nil //nolint:nilerr
	}

	// preventing concurrent git repo modifications
	var once sync.Once
	r.lock.Lock()
	defer once.Do(func() { r.lock.Unlock() })

	token, err := r.client.GitToken(ctx)
	if err != nil {
		return errors.Wrap(err, "error creating git token")
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
			return errors.Wrap(err, "error reading content from source repository file")
		}

		value, err := filepatchers.GetYAML(content, translation.from.yamlPath)
		if err != nil {
			return errors.Wrap(err, "error reading value from source repository file")
		}

		for _, patch := range translation.to {
			err = patch.Apply(reader, writer, value)
			if err != nil {
				return errors.Wrap(err, "error applying translate updates")
			}
		}
	}

	commitMessage := fmt.Sprintf(r.commitMessageTemplate, p.RepositoryName, tag)
	hash, err := git.CommitAndPush(targetRepository, commitMessage)
	if err != nil {
		if errors.Is(err, git.NoChangesError) {
			r.logger.Debugw("skip push to target repository because nothing changed", "target-repo", p.RepositoryName, "source-repo", p.RepositoryName, "release", tag)
			return nil
		}
		return errors.Wrap(err, "error pushing to target repository")
	}

	r.logger.Infow("pushed to translate target repo", "target-repo", p.RepositoryName, "source-repo", p.RepositoryName, "release", tag, "branch", r.branch, "hash", hash)

	once.Do(func() { r.lock.Unlock() })

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
