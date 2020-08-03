package webhooks

import (
	"fmt"
	"net/http"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks/github"
	"github.com/metal-stack/metal-robot/pkg/webhooks/gitlab"
	"go.uber.org/zap"
)

func InitWebhooks(logger *zap.SugaredLogger, cs clients.ClientMap, c *config.Configuration) error {
	for _, w := range c.Webhooks {
		switch w.VCS {
		case config.Github:
			controller, err := github.NewGithubWebhook(logger.Named("github-webhook"), w, cs)
			if err != nil {
				return err
			}
			http.HandleFunc(w.ServePath, controller.Handle)
			logger.Infow("initialized github webhook", "serve-path", w.ServePath)
		case config.Gitlab:
			controller, err := gitlab.NewGitlabWebhook(logger.Named("gitlab-webhook"), w, cs)
			if err != nil {
				return err
			}
			http.HandleFunc(w.ServePath, controller.Handle)
			logger.Infow("initialized gitlab webhook", "serve-path", w.ServePath)
		default:
			return fmt.Errorf("unsupported webhook type: %s", w.VCS)
		}
	}

	return nil
}
