package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"go.uber.org/zap"

	githubcontroller "github.com/metal-stack/metal-robot/pkg/controllers/github"
	gitlabcontroller "github.com/metal-stack/metal-robot/pkg/controllers/gitlab"
	"github.com/metal-stack/v"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/go-playground/validator.v9"
)

const (
	cfgFileType = "yaml"
	moduleName  = "metal-robot"
)

var (
	cfgFile string
	logger  *zap.SugaredLogger
)

// Opts is required in order to have proper validation for args from cobra and viper.
// this is because MarkFlagRequired from cobra does not work well with viper, see:
// https://github.com/spf13/viper/issues/397
type Opts struct {
	BindAddr string
	Port     int

	GithubWebhookServePath  string
	GithubWebhookSecret     string `validate:"required"`
	GithubAppPrivateKeyPath string
	GithubAppID             int64 `validate:"required"`

	GitlabWebhookServePath string
	GitlabWebhookSecret    string `validate:"required"`
}

var cmd = &cobra.Command{
	Use:          moduleName,
	Short:        "a bot helping with automating tasks on github and gitlab",
	Version:      v.V.String(),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := initConfig()
		if err != nil {
			return err
		}
		initLogging()
		opts, err := initOpts()
		if err != nil {
			return fmt.Errorf("unable to init options: %v", err)
		}
		return run(opts)
	},
}

func main() {
	if err := cmd.Execute(); err != nil {
		logger.Fatalw("an error occurred", "error", err)
	}
}

func init() {
	cmd.PersistentFlags().StringP("log-level", "", "info", "sets the application log level")
	cmd.Flags().StringVarP(&cfgFile, "config", "c", "", "alternative path to config file")

	cmd.Flags().StringP("bind-addr", "", "127.0.0.1", "the bind addr of the server")
	cmd.Flags().IntP("port", "", 3000, "the port to serve on")
	cmd.Flags().StringP("github-webhook-serve-path", "", "/github/webhooks", "the path on which the server serves github webhook requests")
	cmd.Flags().StringP("github-webhook-secret", "", "", "the github webhook secret")
	cmd.Flags().StringP("github-app-private-key-path", "", "/etc/metal-robot/key.pem", "the github app secret auth key certificate file path")
	cmd.Flags().Int64("github-app-id", 0, "the github app id")

	cmd.Flags().StringP("gitlab-webhook-serve-path", "", "/gitlab/webhooks", "the path on which the server serves gitlab webhook requests")
	cmd.Flags().StringP("gitlab-webhook-secret", "", "", "the gitlab webhook secret")

	err := viper.BindPFlags(cmd.Flags())
	if err != nil {
		log.Fatalf("unable to construct root command: %v", err)
	}
	err = viper.BindPFlags(cmd.PersistentFlags())
	if err != nil {
		log.Fatalf("unable to construct root command: %v", err)
	}
}

func initOpts() (*Opts, error) {
	opts := &Opts{
		BindAddr: viper.GetString("bind-addr"),
		Port:     viper.GetInt("port"),

		GithubWebhookServePath:  viper.GetString("github-webhook-serve-path"),
		GithubWebhookSecret:     viper.GetString("github-webhook-secret"),
		GithubAppPrivateKeyPath: viper.GetString("github-app-private-key-path"),
		GithubAppID:             viper.GetInt64("github-app-id"),

		GitlabWebhookServePath: viper.GetString("gitlab-webhook-secret"),
		GitlabWebhookSecret:    viper.GetString("gitlab-webhook-secret"),
	}

	validate := validator.New()
	err := validate.Struct(opts)
	if err != nil {
		return nil, err
	}

	return opts, nil
}

func initConfig() error {
	viper.SetEnvPrefix("METAL_ROBOT")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	viper.SetConfigType(cfgFileType)

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
		if err := viper.ReadInConfig(); err != nil {
			return fmt.Errorf("Config file path set explicitly, but unreadable: %v", err)
		}
	} else {
		viper.SetConfigName("config")
		viper.AddConfigPath("/etc/" + moduleName)
		viper.AddConfigPath("$HOME/." + moduleName)
		viper.AddConfigPath(".")
		if err := viper.ReadInConfig(); err != nil {
			usedCfg := viper.ConfigFileUsed()
			if usedCfg != "" {
				return fmt.Errorf("Config file unreadable: %v", err)
			}
		}
	}

	return nil
}

func initLogging() {
	level := zap.InfoLevel

	if viper.IsSet("log-level") {
		err := level.UnmarshalText([]byte(viper.GetString("log-level")))
		if err != nil {
			log.Fatalf("can't initialize zap logger: %v", err)
		}
	}

	cfg := zap.NewProductionConfig()
	cfg.Level = zap.NewAtomicLevelAt(level)

	l, err := cfg.Build()
	if err != nil {
		log.Fatalf("can't initialize zap logger: %v", err)
	}

	logger = l.Sugar()
}

func run(opts *Opts) error {
	githubAuth, err := githubcontroller.NewAuth(logger.Named("github-auth"), opts.GithubAppID, opts.GithubAppPrivateKeyPath)
	if err != nil {
		return err
	}

	githubController, err := githubcontroller.NewController(logger.Named("github-webhook-controller"), githubAuth, opts.GithubWebhookSecret)
	if err != nil {
		return err
	}
	gitlabController, err := gitlabcontroller.NewController(logger.Named("gitlab-webhook-controller"), opts.GitlabWebhookSecret)
	if err != nil {
		return err
	}

	http.HandleFunc(opts.GithubWebhookServePath, githubController.Webhook)
	http.HandleFunc(opts.GitlabWebhookServePath, gitlabController.Webhook)

	addr := fmt.Sprintf("%s:%d", opts.BindAddr, opts.Port)
	logger.Infow("starting metal-robot server", "version", v.V.String(), "address", addr)
	err = http.ListenAndServe(addr, nil)
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
