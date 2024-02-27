package clients

import (
	"fmt"
	"log/slog"

	"github.com/metal-stack/metal-robot/pkg/config"
)

type ClientMap map[string]Client

type Client interface {
	VCS() config.VCSType
	Organization() string
}

func InitClients(logger *slog.Logger, config []config.Client) (ClientMap, error) {
	cs := ClientMap{}
	for _, clientConfig := range config {
		ghConfig := clientConfig.GithubAuthConfig
		glConfig := clientConfig.GitlabAuthConfig

		if (ghConfig == nil) == (glConfig == nil) {
			return nil, fmt.Errorf("either gitlab or github client config must be provided for client %q", clientConfig.Name)
		}

		if ghConfig != nil {
			client, err := NewGithub(logger.WithGroup(clientConfig.Name), clientConfig.OrganizationName, ghConfig)
			if err != nil {
				return nil, err
			}

			cs[clientConfig.Name] = client
		}

		if glConfig != nil {
			client, err := NewGitlab(logger.WithGroup(clientConfig.Name), glConfig.Token)
			if err != nil {
				return nil, err
			}

			cs[clientConfig.Name] = client
		}
	}
	return cs, nil
}
