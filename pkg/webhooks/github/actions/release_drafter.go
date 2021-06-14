package actions

import (
	"context"
	"fmt"
	"regexp"
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

var (
	githubIssueRef = regexp.MustCompile(`\(#(?P<issue>[0-9]+)\)`)
)

type releaseDrafter struct {
	logger        *zap.SugaredLogger
	client        *clients.Github
	titleTemplate string
	draftHeadline string
	repoMap       map[string]bool
	repoName      string
	prHeadline    string
	prDescription *string
}

type releaseDrafterParams struct {
	RepositoryName       string
	TagName              string
	ComponentReleaseInfo *string
}

func newReleaseDrafter(logger *zap.SugaredLogger, client *clients.Github, rawConfig map[string]interface{}) (*releaseDrafter, error) {
	var (
		releaseTitleTemplate = "%s"
		draftHeadline        = "General"
		prHeadline           = "Merged Pull Requests"
	)

	var typedConfig config.ReleaseDraftConfig
	err := mapstructure.Decode(rawConfig, &typedConfig)
	if err != nil {
		return nil, err
	}

	if typedConfig.RepositoryName == "" {
		return nil, fmt.Errorf("repository must be specified")
	}
	if typedConfig.ReleaseTitleTemplate != nil {
		releaseTitleTemplate = *typedConfig.ReleaseTitleTemplate
	}
	if typedConfig.DraftHeadline != nil {
		draftHeadline = *typedConfig.DraftHeadline
	}
	if typedConfig.MergedPRsHeadline != nil {
		prHeadline = *typedConfig.MergedPRsHeadline
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
		logger:        logger,
		client:        client,
		repoMap:       repos,
		repoName:      typedConfig.RepositoryName,
		titleTemplate: releaseTitleTemplate,
		prHeadline:    prHeadline,
		prDescription: typedConfig.MergedPRsDescription,
		draftHeadline: draftHeadline,
	}, nil
}

// draft updates a release draft in a release repository
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

	existingDraft, err := findExistingReleaseDraft(ctx, r.client, r.repoName)
	if err != nil {
		return err
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

	body := r.updateReleaseBody(r.draftHeadline, r.client.Organization(), priorBody, p.RepositoryName, componentSemver, p.ComponentReleaseInfo)

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
			Name:    v3.String(fmt.Sprintf(r.titleTemplate, releaseTag)),
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

func findExistingReleaseDraft(ctx context.Context, client *clients.Github, repoName string) (*github.RepositoryRelease, error) {
	opt := &github.ListOptions{
		PerPage: 100,
	}

	for {
		releases, resp, err := client.GetV3Client().Repositories.ListReleases(ctx, client.Organization(), repoName, opt)
		if err != nil {
			return nil, errors.Wrap(err, "error retrieving releases")
		}

		for _, release := range releases {
			if release.Draft != nil && *release.Draft {
				return release, nil
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return nil, nil
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
			latestTag.Patch = latestTag.Patch + 1
			return "v" + latestTag.String(), nil
		}
	}
	return "v0.0.1", nil
}

func (r *releaseDrafter) updateReleaseBody(headline string, org string, priorBody string, component string, componentVersion semver.Version, componentBody *string) string {
	m := utils.ParseMarkdown(priorBody)

	// ensure draft header
	m.EnsureSection(1, nil, headline, nil, true)

	// ensure component section
	var body []string
	if componentBody != nil {
		lines := strings.Split(strings.Replace(*componentBody, `\r\n`, "\n", -1), "\n")
		for _, l := range lines {
			// TODO: we only add lines from bullet point list for now, but certainly we want to support more in the future.
			if !strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "*") {
				continue
			}

			groups := utils.RegexCapture(githubIssueRef, l)
			issue, ok := groups["issue"]
			if ok {
				l = strings.Replace(l, groups["full_match"], fmt.Sprintf("(%s/%s#%s)", org, component, issue), -1)
			}

			body = append(body, l)
		}
	}
	heading := fmt.Sprintf("%s v%s", component, componentVersion.String())
	section := m.EnsureSection(2, &component, heading, body, false)
	if section != nil {
		// indicates this section has been there before, maybe we need to update the contents
		groups := utils.RegexCapture(utils.SemanticVersionMatcher, section.Heading)
		old := groups["full_match"]
		old = strings.TrimPrefix(old, "v")
		oldVersion, err := semver.Parse(old)
		if err == nil {
			if componentVersion.GT(oldVersion) {
				// in this case we need to merge contents together and update the headline
				section.Heading = heading
				section.ContentLines = append(body, section.ContentLines...)
			}
		}
	}

	return strings.Trim(strings.TrimSpace(m.String()), "\n")
}

// appends a merged pull request to the release draft
func (r *releaseDrafter) appendMergedPR(ctx context.Context, title string, number int64, author string, p *releaseDrafterParams) error {
	_, ok := r.repoMap[p.RepositoryName]
	if ok {
		r.logger.Debugw("skip adding merged pull request to release draft because of special handling in release vector", "repo", p.RepositoryName)
		return nil
	}

	existingDraft, err := findExistingReleaseDraft(ctx, r.client, r.repoName)
	if err != nil {
		return err
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

	body := r.appendPullRequest(r.prHeadline, r.client.Organization(), priorBody, p.RepositoryName, title, number, author)

	if existingDraft != nil {
		existingDraft.Body = &body
		_, _, err := r.client.GetV3Client().Repositories.EditRelease(ctx, r.client.Organization(), r.repoName, existingDraft.GetID(), existingDraft)
		if err != nil {
			return errors.Wrap(err, "unable to update release draft")
		}
		r.logger.Infow("release draft updated", "repository", r.repoName, "trigger-component", p.RepositoryName, "pull-request", title)
	} else {
		newDraft := &github.RepositoryRelease{
			TagName: v3.String(releaseTag),
			Name:    v3.String(fmt.Sprintf(r.titleTemplate, releaseTag)),
			Body:    &body,
			Draft:   v3.Bool(true),
		}
		_, _, err := r.client.GetV3Client().Repositories.CreateRelease(ctx, r.client.Organization(), r.repoName, newDraft)
		if err != nil {
			return errors.Wrap(err, "unable to create release draft")
		}
		r.logger.Infow("new release draft created", "repository", r.repoName, "trigger-component", p.RepositoryName, "pull-request", title)
	}

	return nil
}

func (r *releaseDrafter) appendPullRequest(headline string, org string, priorBody string, repo string, title string, number int64, author string) string {
	m := utils.ParseMarkdown(priorBody)

	l := fmt.Sprintf("* %s (%s/%s#%d) @%s", title, org, repo, number, author)

	body := []string{l}
	if r.prDescription != nil {
		body = append([]string{*r.prDescription}, body...)
	}
	section := m.EnsureSection(1, &headline, headline, body, false)
	if section != nil {
		section.Heading = headline
		section.ContentLines = append([]string{l}, section.ContentLines...)
	}

	return strings.Trim(strings.TrimSpace(m.String()), "\n")
}
