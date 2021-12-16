package clients

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/metal-stack/metal-robot/pkg/config"
	"go.uber.org/zap"

	v3 "github.com/google/go-github/v38/github"
)

type Github struct {
	logger         *zap.SugaredLogger
	keyPath        string
	appID          int64
	installationID int64
	organizationID string
	atr            *ghinstallation.AppsTransport
	itr            *ghinstallation.Transport
}

func NewGithub(logger *zap.SugaredLogger, organizationID string, config *config.GithubClient) (*Github, error) {
	a := &Github{
		logger:         logger,
		keyPath:        config.PrivateKeyCertPath,
		appID:          config.AppID,
		organizationID: organizationID,
	}

	err := a.initClients()
	if err != nil {
		return nil, err
	}

	return a, nil
}

func (a *Github) initClients() error {
	atr, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, a.appID, a.keyPath)
	if err != nil {
		return fmt.Errorf("error creating github app client %w", err)
	}

	installation, _, err := v3.NewClient(&http.Client{Transport: atr}).Apps.FindOrganizationInstallation(context.TODO(), a.organizationID)
	if err != nil {
		return fmt.Errorf("error finding organization installation %w", err)
	}

	a.installationID = installation.GetID()

	itr := ghinstallation.NewFromAppsTransport(atr, a.installationID)

	a.atr = atr
	a.itr = itr

	a.logger.Infow("successfully initalized github app client", "organization-id", a.organizationID, "installation-id", a.installationID, "expected-events", installation.Events)

	return nil
}

func (a *Github) VCS() config.VCSType {
	return config.Github
}

func (a *Github) Organization() string {
	return a.organizationID
}

func (a *Github) GetV3Client() *v3.Client {
	return v3.NewClient(&http.Client{Transport: a.itr})
}

func (a *Github) GetV3AppClient() *v3.Client {
	return v3.NewClient(&http.Client{Transport: a.atr})
}

func (a *Github) GitToken(ctx context.Context) (string, error) {
	t, _, err := a.GetV3AppClient().Apps.CreateInstallationToken(ctx, a.installationID, &v3.InstallationTokenOptions{})
	if err != nil {
		return "", fmt.Errorf("error creating installation token %w", err)
	}
	return t.GetToken(), nil
}
