package webhooks

import (
	"net/http"
	"strings"

	"github.com/metal-stack/metal-robot/pkg/controllers/webhooks/handlers"
	ghwebhooks "gopkg.in/go-playground/webhooks.v5/github"
)

// GithubWebhooks handles github webhook events
func (c *Controller) GithubWebhooks(response http.ResponseWriter, request *http.Request) {
	payload, err := c.gh.hook.Parse(request, c.gh.events...)
	if err != nil {
		if err == ghwebhooks.ErrEventNotFound {
			c.logger.Warnw("received unexpected github event", "error", err)
			response.WriteHeader(http.StatusOK)
		} else {
			c.logger.Errorw("unable to handle github event", "error", err)
			response.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	switch payload := payload.(type) {
	case ghwebhooks.ReleasePayload:
		c.logger.Debugw("received release event")
		err = c.processReleaseEvent(&payload)
		if err != nil {
			c.logger.Errorw("error processing release event", "error", err)
			response.WriteHeader(http.StatusInternalServerError)
			_, err = response.Write([]byte(err.Error()))
			if err != nil {
				c.logger.Errorw("could not write error to http response", "error", err)
			}
			return
		}
	case ghwebhooks.PullRequestPayload:
		c.logger.Debugw("received pull request event")
		err = c.processPullRequestEvent(&payload)
		if err != nil {
			c.logger.Errorw("error processing pull request event", "error", err)
			response.WriteHeader(http.StatusInternalServerError)
			_, err = response.Write([]byte(err.Error()))
			if err != nil {
				c.logger.Errorw("could not write error to http response", "error", err)
			}
			return
		}
	case ghwebhooks.PushPayload:
		c.logger.Debugw("received push event")
		err = c.processPushEvent(&payload)
		if err != nil {
			c.logger.Errorw("error processing push event", "error", err)
			response.WriteHeader(http.StatusInternalServerError)
			_, err = response.Write([]byte(err.Error()))
			if err != nil {
				c.logger.Errorw("could not write error to http response", "error", err)
			}
			return
		}
	default:
		c.logger.Warnw("missing handler", "payload", payload)
	}

	response.WriteHeader(http.StatusOK)
}

func (c *Controller) processReleaseEvent(payload *ghwebhooks.ReleasePayload) error {
	if payload.Action == "released" {
		p := &handlers.ReleaseVectorParams{
			Logger:         c.logger.Named("releases-webhook"),
			RepositoryName: payload.Repository.Name,
			TagName:        payload.Release.TagName,
			Client:         c.gh.auth.GetV3Client(),
			AppClient:      c.gh.auth.GetV3AppClient(),
			InstallID:      c.gh.installID,
		}
		go func() {
			err := handlers.AddToRelaseVector(p)
			if err != nil {
				c.logger.Errorw("error adding release to release vector", "repo", p.RepositoryName, "tag", p.TagName, "error", err)
			}
		}()
	}

	return nil
}

func (c *Controller) processPullRequestEvent(payload *ghwebhooks.PullRequestPayload) error {
	if payload.Action == "opened" && payload.Repository.Name == "docs" {
		p := &handlers.DocsPreviewCommentParams{
			Logger:            c.logger.Named("docs-webhook"),
			RepositoryName:    payload.Repository.Name,
			PullRequestNumber: int(payload.PullRequest.Number),
			Client:            c.gh.auth.GetV3Client(),
		}
		go func() {
			err := handlers.AddDocsPreviewComment(p)
			if err != nil {
				c.logger.Errorw("error adding docs preview comment to docs", "repo", p.RepositoryName, "pull_request", p.PullRequestNumber, "error", err)
			}
		}()
	}

	return nil
}

func (c *Controller) processPushEvent(payload *ghwebhooks.PushPayload) error {
	if payload.Created && strings.HasPrefix(payload.Ref, "refs/tags/v") {
		tag := strings.Replace(payload.Ref, "refs/tags/", "", 1)
		releaseParams := &handlers.ReleaseVectorParams{
			Logger:         c.logger.Named("releases-webhook"),
			RepositoryName: payload.Repository.Name,
			TagName:        tag,
			Client:         c.gh.auth.GetV3Client(),
			AppClient:      c.gh.auth.GetV3AppClient(),
			InstallID:      c.gh.installID,
		}
		go func() {
			err := handlers.AddToRelaseVector(releaseParams)
			if err != nil {
				c.logger.Errorw("error adding new tag to release vector", "repo", releaseParams.RepositoryName, "tag", releaseParams.TagName, "error", err)
			}
		}()

		swaggerParams := &handlers.GenerateSwaggerParams{
			Logger:         c.logger.Named("releases-webhook"),
			RepositoryName: payload.Repository.Name,
			TagName:        tag,
			AppClient:      c.gh.auth.GetV3AppClient(),
			Client:         c.gh.auth.GetV3Client(),
			InstallID:      c.gh.installID,
		}
		go func() {
			err := handlers.GenerateSwaggerClients(swaggerParams)
			if err != nil {
				c.logger.Errorw("error creating branches for swagger client repositories", "repo", releaseParams.RepositoryName, "tag", releaseParams.TagName, "error", err)
			}
		}()
	}

	return nil
}
