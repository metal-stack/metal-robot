package github

import (
	"context"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/metal-stack/metal-robot/pkg/controllers"
	"go.uber.org/zap"

	v3 "github.com/google/go-github/v32/github"
)

type Auth struct {
	logger  *zap.SugaredLogger
	keyPath string
	appID   int64
	atr     *ghinstallation.AppsTransport
	itr     *ghinstallation.Transport
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

	installation, _, err := v3.NewClient(&http.Client{Transport: atr}).Apps.FindOrganizationInstallation(context.TODO(), controllers.GithubOrganisation)
	if err != nil {
		return err
	}

	itr := ghinstallation.NewFromAppsTransport(atr, installation.GetID())

	a.logger.Info("initialized tokens")

	a.atr = atr
	a.itr = itr

	return nil
}

func (a *Auth) GetV3Client() *v3.Client {
	return v3.NewClient(&http.Client{Transport: a.itr})
}

func (a *Auth) GetV3AppClient() *v3.Client {
	return v3.NewClient(&http.Client{Transport: a.atr})
}
