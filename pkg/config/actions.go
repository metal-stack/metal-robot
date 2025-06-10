package config

type WebhookActions []WebhookAction

type WebhookAction struct {
	Type   string         `json:"type" description:"name of the webhook action"`
	Client string         `json:"client" description:"client that this webhook action uses"`
	Args   map[string]any `json:"args" description:"action configuration"`
}
type RepositoryMaintainersConfig struct {
	Suffix                *string `mapstructure:"suffix" description:"suffix for maintainers group"`
	AdditionalMemberships []struct {
		TeamSlug   string `mapstructure:"team" description:"the slug of the team"`
		Permission string `mapstructure:"permission" description:"the permission for the team, must be one of "`
	} `mapstructure:"additional-teams" description:"adds additional teams to this repository"`
}

type DocsPreviewCommentConfig struct {
	CommentTemplate *string `mapstructure:"comment-tpl" description:"template to be used for the docs PR comment"`
	RepositoryName  string  `mapstructure:"repository" description:"the name of the docs repo"`
}

type AggregateReleasesConfig struct {
	TargetRepositoryName string                `mapstructure:"repository" description:"the name of the target repo"`
	TargetRepositoryURL  string                `mapstructure:"repository-url" description:"the url of the target repo"`
	Branch               *string               `mapstructure:"branch" description:"the branch to push in the target repo"`
	BranchBase           *string               `mapstructure:"branch-base" description:"the base branch to raise the pull request against"`
	CommitMsgTemplate    *string               `mapstructure:"commit-tpl" description:"template of the commit message"`
	PullRequestTitle     *string               `mapstructure:"pull-request-title" description:"title of the pull request"`
	SourceRepos          map[string][]Modifier `mapstructure:"repos" description:"the source repositories to trigger this action"`
}

type IssueCommentsHandlerConfig struct {
	TargetRepos map[string]any `mapstructure:"repos" description:"the repositories for which issue comment handling will be applied"`
}

type ProjectItemAddHandlerConfig struct {
	ProjectID string `mapstructure:"project-id" description:"the project in which to move newly created issues and pull requests"`
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
	TargetRepositoryName string                       `mapstructure:"repository" description:"the name of the target repo"`
	TargetRepositoryURL  string                       `mapstructure:"repository-url" description:"the url of the target repo"`
	Branch               *string                      `mapstructure:"branch" description:"the branch to push in the target repo"`
	BranchBase           *string                      `mapstructure:"branch-base" description:"the base branch to raise the pull request against"`
	CommitMsgTemplate    *string                      `mapstructure:"commit-tpl" description:"template of the commit message"`
	PullRequestTitle     *string                      `mapstructure:"pull-request-title" description:"title of the pull request"`
	SourceRepos          map[string][]YAMLTranslation `mapstructure:"repos" description:"the source repositories to trigger this action"`
}

type YAMLTranslation struct {
	From YAMLTranslationRead `mapstructure:"from" description:"the yaml path from where to read the replacement value"`
	To   []Modifier          `mapstructure:"to" description:"the actions to take on the target repo with the read the replacement value"`
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
	Repos                map[string]any `mapstructure:"repos" description:"the repositories for that a release draft will be pushed"`
	RepositoryName       string         `mapstructure:"repository" description:"the name of the release repo"`
	Branch               *string        `mapstructure:"branch" description:"the branch considered for releases"`
	BranchBase           *string        `mapstructure:"branch-base" description:"the base branch to raise the pull request against"`
	ReleaseTitleTemplate *string        `mapstructure:"title-template" description:"custom template for the release title"`
	DraftHeadline        *string        `mapstructure:"draft-headline" description:"custom headline for the release draft"`

	MergedPRsHeadline    *string `mapstructure:"merged-prs-section-headline" description:"custom headline for the section of merged pull requests"`
	MergedPRsDescription *string `mapstructure:"merged-prs-section-description" description:"description for the merged pull requests section"`
}
