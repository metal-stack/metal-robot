module github.com/metal-stack/metal-robot

go 1.25

require (
	github.com/Masterminds/semver/v3 v3.4.0
	github.com/atedja/go-multilock v0.0.0-20170315063113-31d195f255fb
	github.com/bradleyfalzon/ghinstallation/v2 v2.16.0
	github.com/go-git/go-billy/v5 v5.6.2
	// IMPORTANT: better not update, it breaks the aggregate release webhook actions with "error fetching repository refs: object not found"
	github.com/go-git/go-git/v5 v5.3.0
	github.com/go-playground/validator/v10 v10.27.0
	github.com/go-playground/webhooks/v6 v6.4.0
	github.com/google/go-cmp v0.7.0
	github.com/google/go-github/v74 v74.0.0
	github.com/metal-stack/metal-lib v0.23.4
	github.com/metal-stack/v v1.0.3
	github.com/mitchellh/mapstructure v1.5.0
	github.com/shurcooL/githubv4 v0.0.0-20240727222349-48295856cce7
	github.com/spf13/cobra v1.10.1
	github.com/spf13/viper v1.20.1
	github.com/stretchr/testify v1.10.0
	github.com/tidwall/gjson v1.18.0
	github.com/tidwall/sjson v1.2.5
	golang.org/x/sync v0.17.0
	sigs.k8s.io/yaml v1.6.0
)

// TODO: Hope this can be removed in a future release
// https://github.com/google/go-github/pull/3650
replace github.com/google/go-github/v74 => github.com/gerrit91/go-github/v74 v74.0.0-20250725122512-42bbaeb22e64

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/cyphar/filepath-securejoin v0.4.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.10 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/google/go-github/v72 v72.0.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v1.4.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/sagikazarmark/locafero v0.10.0 // indirect
	github.com/sergi/go-diff v1.4.0 // indirect
	github.com/shurcooL/graphql v0.0.0-20230722043721-ed46e5a46466 // indirect
	github.com/sourcegraph/conc v0.3.1-0.20240121214520-5f936abd7ae8 // indirect
	github.com/spf13/afero v1.14.0 // indirect
	github.com/spf13/cast v1.9.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	golang.org/x/crypto v0.41.0 // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
