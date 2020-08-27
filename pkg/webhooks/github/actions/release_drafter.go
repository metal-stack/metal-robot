package actions

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/google/go-github/v32/github"
	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	v3 "github.com/google/go-github/v32/github"
)

type ReleaseDrafter struct {
	logger   *zap.SugaredLogger
	client   *clients.Github
	repoMap  map[string]bool
	repoName string
}

type ReleaseDrafterParams struct {
	RepositoryName       string
	TagName              string
	ComponentReleaseInfo *string
}

func NewReleaseDrafter(logger *zap.SugaredLogger, client *clients.Github, rawConfig map[string]interface{}) (*ReleaseDrafter, error) {

	var typedConfig config.ReleaseDraftConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	if typedConfig.RepositoryName == "" {
		return nil, fmt.Errorf("repository must be specified")
	}

	repos := make(map[string]bool)
	for _, name := range typedConfig.Repos {
		_, ok := repos[name]
		if ok {
			return nil, fmt.Errorf("repository defined twice: %v", name)
		}
		repos[name] = true
	}

	return &ReleaseDrafter{
		logger:   logger,
		client:   client,
		repoMap:  repos,
		repoName: typedConfig.RepositoryName,
	}, nil
}

// UpdateReleaseDraft updates a release draft in a release repository
func (r *ReleaseDrafter) UpdateReleaseDraft(ctx context.Context, p *ReleaseDrafterParams) error {
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

	body := r.updateReleaseBody(releaseTag, priorBody, p.RepositoryName, componentSemver, p.ComponentReleaseInfo)

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

func (r *ReleaseDrafter) guessNextVersionFromLatestRelease(ctx context.Context) (string, error) {
	latest, _, err := r.client.GetV3Client().Repositories.GetLatestRelease(ctx, r.client.Organization(), r.repoName)
	if err != nil {
		return "", errors.Wrap(err, "unable to find latest release")
	}
	if latest != nil && latest.TagName != nil {
		latestTag, err := semver.Parse(*latest.TagName)
		if err != nil {
			r.logger.Warnw("latest release of repository was not a semver tag", "repository", r.repoName, "latest-tag", *latest.TagName)
		} else {
			latestTag.Minor = latestTag.Minor + 1
			return "v" + latestTag.String(), nil
		}
	}
	return "v0.0.1", nil
}

func (r *ReleaseDrafter) updateReleaseBody(version string, priorBody string, component string, componentVersion semver.Version, componentBody *string) string {
	m := parseMarkdown(priorBody)

	m.EnsureSection(1, nil, version, "")
	body := ""
	if componentBody != nil {
		body = *componentBody
	}
	m.EnsureSection(2, &component, fmt.Sprintf("%s v%s", component, componentVersion.String()), body)

	return m.String()
}

type markdown struct {
	sections []*markdownSection
}

type markdownSection struct {
	Level   int
	Heading string
	Content string
}

func parseMarkdown(content string) *markdown {
	var sections []*markdownSection
	lines := strings.Split(content, "\n")

	var currentSection *markdownSection
	for _, l := range lines {
		if strings.HasPrefix(l, "#") {
			level := 0
			for _, char := range l {
				if char != '#' {
					break
				}
				level++
			}
			currentSection = &markdownSection{
				Level:   level,
				Heading: strings.TrimSpace(l[level:]),
			}
			sections = append(sections, currentSection)
			continue
		}

		if currentSection == nil {
			currentSection = &markdownSection{}
			sections = append(sections, currentSection)
		}

		if currentSection.Content == "" {
			currentSection.Content = l
		} else {
			currentSection.Content = currentSection.Content + "\n" + l
		}
	}

	return &markdown{
		sections: sections,
	}
}

func (m *markdown) EnsureSection(level int, headlinePrefix *string, headline string, content string) *markdownSection {
	for _, s := range m.sections {
		if s.Level == level {
			if headlinePrefix == nil {
				return s
			}
			if strings.HasPrefix(s.Heading, *headlinePrefix) {
				return s
			}
		}
	}
	s := &markdownSection{
		Level:   level,
		Heading: headline,
		Content: content,
	}
	m.sections = append(m.sections, s)
	return s
}

func (m *markdown) String() string {
	result := ""
	for _, s := range m.sections {
		if s.Level > 0 {
			result += "\n"
			for i := 0; i < s.Level; i++ {
				result += "#"
			}
			result += " " + s.Heading + "\n"
		}
		result += s.Content
	}
	return result
}
