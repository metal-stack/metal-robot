package webhooks

import (
	"net/http"

	glwebhooks "gopkg.in/go-playground/webhooks.v5/gitlab"
)

// GitlabWebhooks handles gitlab webhook events
func (c *Controller) GitlabWebhooks(response http.ResponseWriter, request *http.Request) {
	payload, err := c.gl.hook.Parse(request, c.gl.events...)
	if err != nil {
		if err == glwebhooks.ErrEventNotFound {
			c.logger.Warnw("received unexpected gitlab event", "error", err)
			response.WriteHeader(http.StatusOK)
		} else {
			c.logger.Errorw("unable to handle gitlab event", "error", err)
			response.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	switch payload := payload.(type) {
	case glwebhooks.PushEventPayload:
		c.logger.Debugw("received push event")
	default:
		c.logger.Warnw("missing handler", "payload", payload)
	}

	response.WriteHeader(http.StatusOK)
}
