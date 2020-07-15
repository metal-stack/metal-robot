package webhooks

import (
	"context"
	"net/http"
	"strings"

	"github.com/metal-stack/metal-robot/pkg/controllers"
	"github.com/metal-stack/metal-robot/pkg/controllers/webhooks/handlers"
	"golang.org/x/sync/errgroup"
	ghwebhooks "gopkg.in/go-playground/webhooks.v5/github"
)

// GithubWebhooks handles github webhook events
func (c *Controller) GithubWebhooks(response http.ResponseWriter, request *http.Request) {
	payload, err := c.gh.hook.Parse(request, c.gh.events...)
	if err != nil {
		if err == ghwebhooks.ErrEventNotFound {
			c.logger.Warnw("received unregistered github event", "error", err)
			response.WriteHeader(http.StatusOK)
		} else {
			c.logger.Errorw("received malformed github event", "error", err)
			response.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	switch payload := payload.(type) {
	case ghwebhooks.ReleasePayload:
		c.logger.Debugw("received release event")
		if err := c.processReleaseEvent(&payload); err != nil {
			c.writeError(err, response)
			return
		}
	case ghwebhooks.PullRequestPayload:
		c.logger.Debugw("received pull request event")
		if err := c.processPullRequestEvent(&payload); err != nil {
			c.writeError(err, response)
			return
		}
	case ghwebhooks.PushPayload:
		c.logger.Debugw("received push event")
		if err := c.processPushEvent(&payload); err != nil {
			c.writeError(err, response)
			return
		}
	case ghwebhooks.IssuesPayload:
		c.logger.Debugw("received issues event")
	default:
		c.logger.Warnw("missing handler", "payload", payload)
	}

	response.WriteHeader(http.StatusOK)
}

func (c *Controller) processReleaseEvent(payload *ghwebhooks.ReleasePayload) error {
	ctx, cancel := context.WithTimeout(context.Background(), controllers.WebhookHandleTimeout)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if payload.Action != "released" {
			return nil
		}
		p := &handlers.ReleaseVectorParams{
			Logger:         c.logger.Named("releases-webhook"),
			RepositoryName: payload.Repository.Name,
			TagName:        payload.Release.TagName,
			Client:         c.gh.auth.GetV3Client(),
			AppClient:      c.gh.auth.GetV3AppClient(),
			InstallID:      c.gh.installID,
		}
		err := handlers.AddToRelaseVector(ctx, p)
		if err != nil {
			c.logger.Errorw("error adding release to release vector", "repo", p.RepositoryName, "tag", p.TagName, "error", err)
			return err
		}

		return nil
	})

	return g.Wait()
}

func (c *Controller) processPullRequestEvent(payload *ghwebhooks.PullRequestPayload) error {
	ctx, cancel := context.WithTimeout(context.Background(), controllers.WebhookHandleTimeout)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if payload.Action != "opened" || payload.Repository.Name != "docs" {
			return nil
		}
		p := &handlers.DocsPreviewCommentParams{
			Logger:            c.logger.Named("docs-webhook"),
			RepositoryName:    payload.Repository.Name,
			PullRequestNumber: int(payload.PullRequest.Number),
			Client:            c.gh.auth.GetV3Client(),
		}
		err := handlers.AddDocsPreviewComment(ctx, p)
		if err != nil {
			c.logger.Errorw("error adding docs preview comment to docs", "repo", p.RepositoryName, "pull_request", p.PullRequestNumber, "error", err)
			return err
		}

		return nil
	})

	return g.Wait()
}

func (c *Controller) processPushEvent(payload *ghwebhooks.PushPayload) error {
	ctx, cancel := context.WithTimeout(context.Background(), controllers.WebhookHandleTimeout)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		if !payload.Created || !strings.HasPrefix(payload.Ref, "refs/tags/v") {
			return nil
		}

		releaseParams := &handlers.ReleaseVectorParams{
			Logger:         c.logger.Named("releases-webhook"),
			RepositoryName: payload.Repository.Name,
			TagName:        extractTag(payload),
			Client:         c.gh.auth.GetV3Client(),
			AppClient:      c.gh.auth.GetV3AppClient(),
			InstallID:      c.gh.installID,
		}

		err := handlers.AddToRelaseVector(ctx, releaseParams)
		if err != nil {
			c.logger.Errorw("error adding new tag to release vector", "repo", releaseParams.RepositoryName, "tag", releaseParams.TagName, "error", err)
			return err
		}

		return nil
	})

	g.Go(func() error {
		if !payload.Created || !strings.HasPrefix(payload.Ref, "refs/tags/v") {
			return nil
		}

		swaggerParams := &handlers.GenerateSwaggerParams{
			Logger:         c.logger.Named("releases-webhook"),
			RepositoryName: payload.Repository.Name,
			TagName:        extractTag(payload),
			AppClient:      c.gh.auth.GetV3AppClient(),
			Client:         c.gh.auth.GetV3Client(),
			InstallID:      c.gh.installID,
		}

		err := handlers.GenerateSwaggerClients(ctx, swaggerParams)
		if err != nil {
			c.logger.Errorw("error creating branches for swagger client repositories", "repo", swaggerParams.RepositoryName, "tag", swaggerParams.TagName, "error", err)
			return err
		}

		return nil
	})

	return g.Wait()
}

func extractTag(payload *ghwebhooks.PushPayload) string {
	return strings.Replace(payload.Ref, "refs/tags/", "", 1)
}

func (c *Controller) writeError(err error, response http.ResponseWriter) {
	c.logger.Errorw("error processing event", "error", err)
	response.WriteHeader(http.StatusInternalServerError)
	_, err = response.Write([]byte(err.Error()))
	if err != nil {
		c.logger.Errorw("could not write error to http response", "error", err)
	}
}
