package github

import (
	"net/http"

	"github.com/bradleyfalzon/ghinstallation"
	"go.uber.org/zap"

	v3 "github.com/google/go-github/v32/github"
)

type Auth struct {
	logger  *zap.SugaredLogger
	keyPath string
	appID   int64
	atr     *ghinstallation.AppsTransport
}

func NewAuth(logger *zap.SugaredLogger, appID int64, privateKeyCertPath string) (*Auth, error) {
	a := &Auth{
		logger:  logger,
		keyPath: privateKeyCertPath,
		appID:   appID,
	}

	err := a.initInstallToken()
	if err != nil {
		return nil, err
	}

	return a, nil
}

func (a *Auth) initInstallToken() error {
	atr, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, a.appID, a.keyPath)
	if err != nil {
		a.logger.Errorw("error creating http transport for Github app auth", "error", err)
		return err
	}

	a.logger.Info("initialized install token")

	a.atr = atr

	return nil
}

func (a *Auth) GetV3Client() *v3.Client {
	return v3.NewClient(&http.Client{Transport: a.atr})
}
