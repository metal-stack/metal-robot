module github.com/metal-stack/metal-robot

go 1.24

require (
	github.com/Masterminds/semver/v3 v3.3.1
	github.com/atedja/go-multilock v0.0.0-20170315063113-31d195f255fb
	github.com/bradleyfalzon/ghinstallation/v2 v2.16.0
	github.com/go-git/go-billy/v5 v5.5.0
	// IMPORTANT: keep this version as long as https://github.com/go-git/go-git/issues/328 is open
	github.com/go-git/go-git/v5 v5.3.0
	github.com/go-playground/validator/v10 v10.26.0
	github.com/go-playground/webhooks/v6 v6.4.0
	github.com/google/go-cmp v0.7.0
	github.com/google/go-github/v74 v74.0.0
	github.com/metal-stack/metal-lib v0.23.0
	github.com/metal-stack/v v1.0.3
	github.com/mitchellh/mapstructure v1.5.0
	github.com/shurcooL/githubv4 v0.0.0-20240727222349-48295856cce7
	github.com/spf13/cobra v1.9.1
	github.com/spf13/viper v1.20.1
	github.com/tidwall/gjson v1.18.0
	github.com/tidwall/sjson v1.2.5
	golang.org/x/sync v0.15.0
	sigs.k8s.io/yaml v1.4.0
)

replace github.com/google/go-github/v74 => github.com/gerrit91/go-github/v74 v74.0.0-20250725122512-42bbaeb22e64

require github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/cyphar/filepath-securejoin v0.2.4 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.8 // indirect
	github.com/go-git/gcfg v1.5.1-0.20230307220236-3a3c6141e376 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/google/go-github/v72 v72.0.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/pelletier/go-toml/v2 v2.2.3 // indirect
	github.com/sagikazarmark/locafero v0.7.0 // indirect
	github.com/sergi/go-diff v1.3.1 // indirect
	github.com/shurcooL/graphql v0.0.0-20230722043721-ed46e5a46466 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.14.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.36.0 // indirect
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
