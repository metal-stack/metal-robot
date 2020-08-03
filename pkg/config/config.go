package config

import (
	"io/ioutil"
	"strings"

	"github.com/ghodss/yaml"
)

type VCSType string

const (
	Github VCSType = "github"
	Gitlab VCSType = "gitlab"
)

type Configuration struct {
	Clients  []Client  `json:"clients" description:"client configurations"`
	Webhooks []Webhook `json:"webhooks" description:"webhook configurations"`
	Raw      []byte
}

type Client struct {
	GitlabAuthConfig *GitlabClient `json:"gitlab" description:"auth config a gitlab client"`
	GithubAuthConfig *GithubClient `json:"github" description:"auth config a github client"`
	Name             string        `json:"name" description:"name of the client, used for referencing in webhook config"`
	OrganizationName string        `json:"organization" description:"name of the organization that this client will act on"`
}

type GithubClient struct {
	AppID              int64  `json:"app-id" description:"application id of github app"`
	PrivateKeyCertPath string `json:"key-path" description:"private key pem path of github app"`
}

type GitlabClient struct {
	Token string `json:"token" description:"auth token for gitlab client"`
}

type Webhook struct {
	VCS       VCSType        `json:"vcs" description:"type of the vcs"`
	ServePath string         `json:"serve-path" description:"path of the webhook to serve on"`
	Secret    string         `json:"secret" description:"the webhook secret"`
	Actions   WebhookActions `json:"actions" description:"webhook actions"`
}

func New(configPath string) (*Configuration, error) {
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	config := &Configuration{Raw: data}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func (w WebhookActions) String() string {
	actions := []string{}
	for _, h := range w {
		actions = append(actions, h.Type)
	}
	return strings.Join(actions, ", ")
}
