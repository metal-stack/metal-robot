package gitlab

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	aggregate_releases "github.com/metal-stack/metal-robot/pkg/webhooks/github/actions/aggregate-releases"
	"github.com/metal-stack/metal-robot/pkg/webhooks/handlers"

	glwebhooks "github.com/go-playground/webhooks/v6/gitlab"
)

func initHandlers(logger *slog.Logger, cs clients.ClientMap, path string, cfg config.WebhookActions) error {
	for _, spec := range cfg {
		c, ok := cs[spec.Client]
		if !ok {
			return fmt.Errorf("webhook action client not found: %s", spec.Client)
		}

		// we only receive webhooks from gitlab but we act on github
		switch clientType := c.(type) {
		case *clients.Github:
		default:
			return fmt.Errorf("action %s only supports github clients, not: %s", spec.Type, clientType)
		}

		client := c.(*clients.Github)

		switch t := spec.Type; t {
		case config.ActionAggregateReleases:
			h, _, err := aggregate_releases.New(client, spec.Args)
			if err != nil {
				return err
			}

			handlers.Register(string(t), path, h, func(event *glwebhooks.TagEventPayload) (*aggregate_releases.Params, error) {
				return &aggregate_releases.Params{
					RepositoryName: event.Repository.Name,
					RepositoryURL:  event.Repository.URL,
					TagName:        extractTag(event),
					Sender:         event.UserUsername,
				}, nil
			})
		default:
			return fmt.Errorf("handler type not supported: %s", t)
		}

		logger.Debug("initialized github webhook action", "name", spec.Type)
	}

	return nil
}

func extractTag(payload *glwebhooks.TagEventPayload) string {
	return strings.Replace(payload.Ref, "refs/tags/", "", 1)
}
