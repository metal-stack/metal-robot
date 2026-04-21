package release_drafter

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/markdown"
	"github.com/metal-stack/metal-robot/pkg/webhooks/handlers"
	handlerrors "github.com/metal-stack/metal-robot/pkg/webhooks/handlers/errors"
	"golang.org/x/sync/errgroup"
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

func NewAppendMergedPRs(logger *slog.Logger, client *clients.Github, rawConfig map[string]any) (handlers.WebhookHandler[*AppendMergedPrParams], error) {
	rd, err := New(client, rawConfig)
	if err != nil {
		return nil, err
	}

	return &appendMergedPR{
		rd: rd.(*releaseDrafter),
	}, nil
}

// Handle appends a merged pull request to the release draft of the repo in which the PR was merged
func (r *appendMergedPR) Handle(ctx context.Context, log *slog.Logger, p *AppendMergedPrParams) error {
	var g errgroup.Group

	g.Go(func() error {
		// carry over BREAKING_CHANGE, ACTIONS_REQUIRED, ... blocks from the merged PR into the release notes of the same repository
		// with a release, these will be carried over to the global release draft

		if p.ComponentReleaseInfo == nil {
			return nil
		}

		infos, err := r.rd.releaseInfos(ctx, log, p.RepositoryName)
		if err != nil {
			return err
		}

		m := markdown.Parse(infos.body)

		var releaseSuffix *string
		if p.ReleaseURL != "" {
			tmp := fmt.Sprintf("([release notes](%s))", p.ReleaseURL)
			releaseSuffix = &tmp
		}

		err = prependCodeBlocks(m, *p.ComponentReleaseInfo, r.rd.draftHeadline, releaseSuffix)
		if err != nil {
			return handlerrors.Skip("skip adding release draft: %w", err)
		}

		return r.rd.createOrUpdateRelease(ctx, log, p.RepositoryName, infos, m.String())
	})

	g.Go(func() error {
		// append merged prs in the global release draft for repositories that are not in the release vector

		if _, ok := r.rd.repoMap[p.RepositoryName]; ok {
			return nil
		}

		infos, err := r.rd.releaseInfos(ctx, log, p.RepositoryName)
		if err != nil {
			return err
		}

		body := r.rd.appendPullRequest(r.rd.client.Organization(), infos.body, p)

		return r.rd.createOrUpdateRelease(ctx, log, r.rd.repoName, infos, body)
	})

	if err := g.Wait(); err != nil {
		return fmt.Errorf("error for release-drafter while handling merged pull request event: %w", err)
	}

	return nil
}

func (r *releaseDrafter) appendPullRequest(org string, priorBody string, p *AppendMergedPrParams) string {
	var (
		m = markdown.Parse(priorBody)

		l = fmt.Sprintf("* %s (%s/%s#%d) @%s", p.Title, org, p.RepositoryName, p.Number, p.Author)

		body = []string{l}
	)

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

	if p.ComponentReleaseInfo != nil {
		issueSuffix := fmt.Sprintf("(%s/%s#%d)", org, p.RepositoryName, p.Number)
		_ = prependCodeBlocks(m, *p.ComponentReleaseInfo, r.draftHeadline, &issueSuffix)
	}

	return m.String()
}
