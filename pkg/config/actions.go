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

type IssuesCommentHandlerConfig struct {
	TargetRepos []TargetRepo `mapstructure:"repos" description:"the repositories that will be updated"`
}

type DistributeReleasesConfig struct {
	SourceRepositoryName string       `mapstructure:"repository" description:"the name of the source repo"`
	SourceRepositoryURL  string       `mapstructure:"repository-url" description:"the url of the source repo"`
	BranchTemplate       *string      `mapstructure:"branch-template" description:"the branch to push in the target repos"`
	CommitMsgTemplate    *string      `mapstructure:"commit-tpl" description:"template of the commit message in the target repos"`
	PullRequestTitle     *string      `mapstructure:"pull-request-title" description:"title of the pull request"`
	TargetRepos          []TargetRepo `mapstructure:"repos" description:"the repositories that will be updated"`
}

type YAMLTranslateReleasesConfig struct {
	TargetRepositoryName string                       `mapstructure:"repository" description:"the name of the taget repo"`
	TargetRepositoryURL  string                       `mapstructure:"repository-url" description:"the url of the target repo"`
	Branch               *string                      `mapstructure:"branch" description:"the branch to push in the target repo"`
	CommitMsgTemplate    *string                      `mapstructure:"commit-tpl" description:"template of the commit message"`
	PullRequestTitle     *string                      `mapstructure:"pull-request-title" description:"title of the pull request"`
	SourceRepos          map[string][]YAMLTranslation `mapstructure:"repos" description:"the source repositories to trigger this action"`
}

type YAMLTranslation struct {
	From YAMLTranslationRead `mapstructure:"from" description:"the yaml path from where to read the replacement value"`
	To   []Modifier          `mapstructure:"to" description:"the actions to take on the traget repo with the read the replacement value"`
}

type YAMLTranslationRead struct {
	File     string `mapstructure:"file" description:"the name of the file to be patched"`
	YAMLPath string `mapstructure:"yaml-path" description:"the yaml path to the version"`
}

type TargetRepo struct {
	RepositoryName string     `mapstructure:"repository" description:"the name of the target repo"`
	RepositoryURL  string     `mapstructure:"repository-url" description:"the name of the target repo"`
	Patches        []Modifier `mapstructure:"modifiers" description:"the name of the target repo"`
}

type ReleaseDraftConfig struct {
	Repos                map[string]interface{} `mapstructure:"repos" description:"the repositories for that a release draft will be pushed"`
	RepositoryName       string                 `mapstructure:"repository" description:"the name of the release repo"`
	ReleaseTitleTemplate *string                `mapstructure:"title-template" description:"custom template for the release title"`
	DraftHeadline        *string                `mapstructure:"draft-headline" description:"custom headline for the release draft"`

	MergedPRsHeadline    *string `mapstructure:"merged-prs-section-headline" description:"custom headline for the section of merged pull requests"`
	MergedPRsDescription *string `mapstructure:"merged-prs-section-description" description:"description for the merged pull requests section"`
}
