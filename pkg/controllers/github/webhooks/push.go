package webhooks

import (
	v3 "github.com/google/go-github/v32/github"
	"go.uber.org/zap"
	"gopkg.in/go-playground/webhooks.v5/github"
)

type PushProcessor struct {
	Logger    *zap.SugaredLogger
	Payload   *github.PushPayload
	Client    *v3.Client
	InstallID int64
}

func ProcessPushEvent(p *PushProcessor) error {
	err := generateSwaggerClients(p)
	if err != nil {
		return err
	}
	return prepareDraftReleaseNotes(p)
}

func generateSwaggerClients(p *PushProcessor) error {
	return nil
}

func prepareDraftReleaseNotes(p *PushProcessor) error {
	return nil
}
