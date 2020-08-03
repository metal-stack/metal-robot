package clients

import (
	"github.com/metal-stack/metal-robot/pkg/config"
	"go.uber.org/zap"
)

type Gitlab struct {
	logger         *zap.SugaredLogger
	token          string
	organizationID string
}

func NewGitlab(logger *zap.SugaredLogger, token string) (*Gitlab, error) {
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
