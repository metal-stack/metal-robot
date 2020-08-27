package config

type WebhookActions []WebhookAction

type WebhookAction struct {
	Type   string                 `json:"type" description:"name of the webhook action"`
	Client string                 `json:"client" description:"client that this webhook action uses"`
	Args   map[string]interface{} `json:"args" description:"action configuration"`
}
type RepositoryMaintainersConfig struct {
	Suffix *string `mapstructure:"suffix" description:"suffix for maintainers group"`
}

type DocsPreviewCommentConfig struct {
	CommentTemplate *string `mapstructure:"comment-tpl" description:"template to be used for the docs PR comment"`
	RepositoryName  string  `mapstructure:"repository" description:"the name of the docs repo"`
}

type ReleaseVectorConfig struct {
	RepositoryName    string                `mapstructure:"repository" description:"the name of the release repo"`
	RepositoryURL     string                `mapstructure:"repository-url" description:"the url of the release repo"`
	Branch            *string               `mapstructure:"branch" description:"the branch to push in the release vector repo"`
	CommitMsgTemplate *string               `mapstructure:"commit-tpl" description:"template of the commit message"`
	PullRequestTitle  *string               `mapstructure:"pull-request-title" description:"title of the pull request"`
	Repos             map[string][]Modifier `mapstructure:"repos" description:"the repositories that will be pushed to the release vector"`
}

type ReleaseDraftConfig struct {
	RepositoryName string   `mapstructure:"repository" description:"the name of the release repo"`
	RepositoryURL  string   `mapstructure:"repository-url" description:"the url of the release repo"`
	Repos          []string `mapstructure:"repos" description:"the repositories for that a release draft will be pushed"`
}

type SwaggerClientsConfig struct {
	BranchTemplate    *string                        `mapstructure:"branch-template" description:"the branch to push in the swagger client repo"`
	CommitMsgTemplate *string                        `mapstructure:"commit-tpl" description:"template of the commit message in the swagger client repo"`
	Repos             map[string][]SwaggerClientRepo `mapstructure:"repos" description:"the swagger client repositories that will be updated"`
}

type SwaggerClientRepo struct {
	RepositoryName string     `mapstructure:"repository" description:"the name of the swagger client repo"`
	RepositoryURL  string     `mapstructure:"repository-url" description:"the name of the swagger client repo"`
	Patches        []Modifier `mapstructure:"modifiers" description:"the name of the swagger client repo"`
}
