package webhooks

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/github"
	"github.com/metal-stack/metal-robot/pkg/webhooks/gitlab"
)

func InitWebhooks(logger *slog.Logger, cs clients.ClientMap, c *config.Configuration) error {
	for _, w := range c.Webhooks {
		switch w.VCS {
		case config.Github:
			controller, err := github.NewGithubWebhook(logger.WithGroup("github-webhook"), w, cs)
			if err != nil {
				return err
			}
			http.HandleFunc(w.ServePath, controller.Handle)
			logger.Info("initialized github webhook", "serve-path", w.ServePath)
		case config.Gitlab:
			controller, err := gitlab.NewGitlabWebhook(logger.WithGroup("gitlab-webhook"), w, cs)
			if err != nil {
				return err
			}
			http.HandleFunc(w.ServePath, controller.Handle)
			logger.Info("initialized gitlab webhook", "serve-path", w.ServePath)
		default:
			return fmt.Errorf("unsupported webhook type: %s", w.VCS)
		}
	}

	return nil
}
