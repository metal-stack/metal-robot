package clients

import (
	"log/slog"

	"github.com/metal-stack/metal-robot/pkg/config"
)

type Gitlab struct {
	logger         *slog.Logger
	token          string
	organizationID string
}

func NewGitlab(logger *slog.Logger, token string) (*Gitlab, error) {
	a := &Gitlab{
		logger: logger,
		token:  token,
	}

	return a, nil
}

func (a *Gitlab) VCS() config.VCSType {
	return config.Gitlab
}

func (a *Gitlab) Organization() string {
	return a.organizationID
}
