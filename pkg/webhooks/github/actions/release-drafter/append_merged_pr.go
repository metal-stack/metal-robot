package release_drafter

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/markdown"
	"github.com/metal-stack/metal-robot/pkg/webhooks/github/actions"
	handlerrors "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/common/errors"
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
	rd, err := New(client, rawConfig)
	if err != nil {
		return nil, err
	}

	return &appendMergedPR{
		rd: rd.(*releaseDrafter),
	}, nil
}

// Handle appends a merged pull request to the release draft
func (r *appendMergedPR) Handle(ctx context.Context, log *slog.Logger, p *AppendMergedPrParams) error {
	_, ok := r.rd.repoMap[p.RepositoryName]
	if ok {
		// if there is an ACTIONS_REQUIRED block, we want to add it (even when it's a release vector handled repository)

		if p.ComponentReleaseInfo == nil {
			return handlerrors.Skip("not adding merged pull request to release draft because of special handling for release vector repositories")
		}

		infos, err := r.rd.releaseInfos(ctx, log)
		if err != nil {
			return err
		}

		m := markdown.Parse(infos.body)

		issueSuffix := fmt.Sprintf("(%s/%s#%d)", r.rd.client.Organization(), p.RepositoryName, p.Number)
		err = r.rd.prependCodeBlocks(m, *p.ComponentReleaseInfo, &issueSuffix)
		if err != nil {
			return handlerrors.Skip("skip adding merged pull request to release draft: %s", err)
		}

		body := m.String()

		return r.rd.createOrUpdateRelease(ctx, log, infos, body, &p.Params)
	}

	infos, err := r.rd.releaseInfos(ctx, log)
	if err != nil {
		return err
	}

	body := r.rd.appendPullRequest(r.rd.client.Organization(), infos.body, p.RepositoryName, p.Title, p.Number, p.Author, p.ComponentReleaseInfo)

	return r.rd.createOrUpdateRelease(ctx, log, infos, body, &p.Params)
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
