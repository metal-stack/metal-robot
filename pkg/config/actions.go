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

type AggregateReleasesConfig struct {
	TargetRepositoryName string                `mapstructure:"repository" description:"the name of the taget repo"`
	TargetRepositoryURL  string                `mapstructure:"repository-url" description:"the url of the target repo"`
	Branch               *string               `mapstructure:"branch" description:"the branch to push in the target repo"`
	CommitMsgTemplate    *string               `mapstructure:"commit-tpl" description:"template of the commit message"`
	PullRequestTitle     *string               `mapstructure:"pull-request-title" description:"title of the pull request"`
	SourceRepos          map[string][]Modifier `mapstructure:"repos" description:"the source repositories to trigger this action"`
}

type DistributeReleasesConfig struct {
	SourceRepositoryName string       `mapstructure:"repository" description:"the name of the source repo"`
	SourceRepositoryURL  string       `mapstructure:"repository-url" description:"the url of the source repo"`
	BranchTemplate       *string      `mapstructure:"branch-template" description:"the branch to push in the target repos"`
	CommitMsgTemplate    *string      `mapstructure:"commit-tpl" description:"template of the commit message in the target repos"`
	PullRequestTitle     *string      `mapstructure:"pull-request-title" description:"title of the pull request"`
	TargetRepos          []TargetRepo `mapstructure:"repos" description:"the  repositories that will be updated"`
}

type TargetRepo struct {
	RepositoryName string     `mapstructure:"repository" description:"the name of the target repo"`
	RepositoryURL  string     `mapstructure:"repository-url" description:"the name of the target repo"`
	Patches        []Modifier `mapstructure:"modifiers" description:"the name of the target repo"`
}
