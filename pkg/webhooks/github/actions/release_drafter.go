package actions

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/google/go-github/v32/github"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/utils"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	v3 "github.com/google/go-github/v32/github"
)

type releaseDrafter struct {
	logger   *zap.SugaredLogger
	client   *clients.Github
	repoMap  map[string]bool
	repoName string
}

type releaseDrafterParams struct {
	RepositoryName       string
	TagName              string
	ComponentReleaseInfo *string
}

func newReleaseDrafter(logger *zap.SugaredLogger, client *clients.Github, rawConfig map[string]interface{}) (*releaseDrafter, error) {
	var typedConfig config.ReleaseDraftConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	if typedConfig.RepositoryName == "" {
		return nil, fmt.Errorf("repository must be specified")
	}

	repos := make(map[string]bool)
	for name := range typedConfig.Repos {
		_, ok := repos[name]
		if ok {
			return nil, fmt.Errorf("repository defined twice: %v", name)
		}
		repos[name] = true
	}

	return &releaseDrafter{
		logger:   logger,
		client:   client,
		repoMap:  repos,
		repoName: typedConfig.RepositoryName,
	}, nil
}

// UpdateReleaseDraft updates a release draft in a release repository
func (r *releaseDrafter) draft(ctx context.Context, p *releaseDrafterParams) error {
	_, ok := r.repoMap[p.RepositoryName]
	if !ok {
		r.logger.Debugw("skip adding release draft because not a release vector repo", "repo", p.RepositoryName, "release", p.TagName)
		return nil
	}
	componentTag := p.TagName
	if !strings.HasPrefix(componentTag, "v") {
		r.logger.Debugw("skip adding release draft because tag not starting with v", "repo", p.RepositoryName, "release", componentTag)
		return nil
	}
	trimmedVersion := strings.TrimPrefix(componentTag, "v")
	componentSemver, err := semver.Parse(trimmedVersion)
	if err != nil {
		r.logger.Debugw("skip adding release draft because tag is not semver compatible", "repo", p.RepositoryName, "release", componentTag)
		return nil
	}

	opt := &github.ListOptions{
		PerPage: 100,
	}
	var existingDraft *github.RepositoryRelease
	for {
		releases, resp, err := r.client.GetV3Client().Repositories.ListReleases(ctx, r.client.Organization(), r.repoName, opt)
		if err != nil {
			return errors.Wrap(err, "error retrieving releases")
		}

		for _, release := range releases {
			if release.Draft != nil && *release.Draft {
				existingDraft = release
				break
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	var releaseTag string
	if existingDraft != nil && existingDraft.TagName != nil {
		releaseTag = *existingDraft.TagName
	} else {
		releaseTag, err = r.guessNextVersionFromLatestRelease(ctx)
		if err != nil {
			return err
		}
	}

	var priorBody string
	if existingDraft != nil && existingDraft.Body != nil {
		priorBody = *existingDraft.Body
	}

	body := r.updateReleaseBody("General", priorBody, p.RepositoryName, componentSemver, p.ComponentReleaseInfo)

	if existingDraft != nil {
		existingDraft.Body = &body
		_, _, err := r.client.GetV3Client().Repositories.EditRelease(ctx, r.client.Organization(), r.repoName, existingDraft.GetID(), existingDraft)
		if err != nil {
			return errors.Wrap(err, "unable to update release draft")
		}
		r.logger.Infow("release draft updated", "repository", r.repoName, "trigger-component", p.RepositoryName, "version", p.TagName)
	} else {
		newDraft := &github.RepositoryRelease{
			TagName: v3.String(releaseTag),
			Name:    v3.String(releaseTag),
			Body:    &body,
			Draft:   v3.Bool(true),
		}
		_, _, err := r.client.GetV3Client().Repositories.CreateRelease(ctx, r.client.Organization(), r.repoName, newDraft)
		if err != nil {
			return errors.Wrap(err, "unable to create release draft")
		}
		r.logger.Infow("new release draft created", "repository", r.repoName, "trigger-component", p.RepositoryName, "version", p.TagName)
	}

	return nil
}

func (r *releaseDrafter) guessNextVersionFromLatestRelease(ctx context.Context) (string, error) {
	latest, _, err := r.client.GetV3Client().Repositories.GetLatestRelease(ctx, r.client.Organization(), r.repoName)
	if err != nil {
		return "", errors.Wrap(err, "unable to find latest release")
	}
	if latest != nil && latest.TagName != nil {
		groups := utils.RegexCapture(utils.SemanticVersionMatcher, *latest.TagName)
		t := groups["full_match"]
		t = strings.TrimPrefix(t, "v")
		latestTag, err := semver.Parse(t)
		if err != nil {
			r.logger.Warnw("latest release of repository was not a semver tag", "repository", r.repoName, "latest-tag", *latest.TagName)
		} else {
			latestTag.Minor = latestTag.Minor + 1
			return "v" + latestTag.String(), nil
		}
	}
	return "v0.0.1", nil
}

func (r *releaseDrafter) updateReleaseBody(headline string, priorBody string, component string, componentVersion semver.Version, componentBody *string) string {
	m := utils.ParseMarkdown(priorBody)

	// ensure draft header
	m.EnsureSection(1, nil, headline, nil)

	// ensure component secftion
	var body []string
	if componentBody != nil {
		lines := strings.Split(*componentBody, "\n")
		body = append(body, lines...)
	}
	heading := fmt.Sprintf("%s v%s", component, componentVersion.String())
	section := m.EnsureSection(2, &component, heading, body)
	if section != nil {
		groups := utils.RegexCapture(utils.SemanticVersionMatcher, section.Heading)
		old := groups["full_match"]
		old = strings.TrimPrefix(old, "v")
		oldVersion, err := semver.Parse(old)
		if err == nil {
			if componentVersion.GT(oldVersion) {
				// indicates this section has been there before and the version was updated
				// in this case we need to merge contents together and update the headline
				section.Heading = heading
				section.ContentLines = append(section.ContentLines, body...)
			}
		}

	}

	return strings.Trim(strings.TrimSpace(m.String()), "\n")
}
