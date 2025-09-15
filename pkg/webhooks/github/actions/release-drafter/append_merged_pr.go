package release_drafter

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/markdown"
	"github.com/metal-stack/metal-robot/pkg/webhooks/github/actions"
)

type appendMergedPR struct {
	rd *releaseDrafter
}

type AppendMergedPrParams struct {
	Params

	Title  string
	Number int
	Author string
}

func NewAppendMergedPRs(logger *slog.Logger, client *clients.Github, rawConfig map[string]any) (actions.WebhookHandler[*AppendMergedPrParams], error) {
	rd, err := New(logger, client, rawConfig)
	if err != nil {
		return nil, err
	}

	return &appendMergedPR{
		rd: rd.(*releaseDrafter),
	}, nil
}

func (r *appendMergedPR) Handle(ctx context.Context, p *AppendMergedPrParams) error {
	_, ok := r.rd.repoMap[p.RepositoryName]
	if ok {
		// if there is an ACTIONS_REQUIRED block, we want to add it (even when it's a release vector handled repository)

		if p.ComponentReleaseInfo == nil {
			r.rd.logger.Debug("skip adding merged pull request to release draft because of special handling in release vector", "repo", p.RepositoryName)
			return nil
		}

		infos, err := r.rd.releaseInfos(ctx)
		if err != nil {
			return err
		}

		m := markdown.Parse(infos.body)

		issueSuffix := fmt.Sprintf("(%s/%s#%d)", r.rd.client.Organization(), p.RepositoryName, p.Number)
		err = r.rd.prependCodeBlocks(m, *p.ComponentReleaseInfo, &issueSuffix)
		if err != nil {
			r.rd.logger.Debug("skip adding merged pull request to release draft", "reason", err, "repo", p.RepositoryName)
			return nil
		}

		body := m.String()

		return r.rd.createOrUpdateRelease(ctx, infos, body, &p.Params)
	}

	infos, err := r.rd.releaseInfos(ctx)
	if err != nil {
		return err
	}

	body := r.rd.appendPullRequest(r.rd.client.Organization(), infos.body, p.RepositoryName, p.Title, p.Number, p.Author, p.ComponentReleaseInfo)

	return r.rd.createOrUpdateRelease(ctx, infos, body, &p.Params)
}

func (r *releaseDrafter) appendPullRequest(org string, priorBody string, repo string, title string, number int, author string, prBody *string) string {
	m := markdown.Parse(priorBody)

	l := fmt.Sprintf("* %s (%s/%s#%d) @%s", title, org, repo, number, author)

	body := []string{l}

	section := m.FindSectionByHeading(1, r.prHeadline)
	if section == nil {
		if r.prDescription != nil {
			body = append([]string{*r.prDescription}, body...)
		}

		m.AppendSection(&markdown.MarkdownSection{
			Level:        1,
			Heading:      r.prHeadline,
			ContentLines: body,
		})
	} else {
		alreadyPresent := false
		for _, existingLine := range section.ContentLines {
			if existingLine == l {
				alreadyPresent = true
			}
		}
		if !alreadyPresent {
			section.AppendContent(body)
		}
	}

	if prBody != nil {
		issueSuffix := fmt.Sprintf("(%s/%s#%d)", org, repo, number)
		_ = r.prependCodeBlocks(m, *prBody, &issueSuffix)
	}

	return m.String()
}
