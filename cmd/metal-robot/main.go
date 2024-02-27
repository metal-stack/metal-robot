package main

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/metal-stack/metal-robot/pkg/clients"
	"github.com/metal-stack/metal-robot/pkg/config"
	"github.com/metal-stack/metal-robot/pkg/webhooks"
	"github.com/metal-stack/v"

	"github.com/go-playground/validator/v10"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const (
	cfgFileType = "yaml"
	moduleName  = "metal-robot"
)

var (
	cfgFile string
	logger  *slog.Logger

	c *config.Configuration
)

// Opts is required in order to have proper validation for args from cobra and viper.
// this is because MarkFlagRequired from cobra does not work well with viper, see:
// https://github.com/spf13/viper/issues/397
type Opts struct {
	BindAddr string
	Port     int
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
			return fmt.Errorf("unable to init options: %w", err)
		}
		return run(opts)
	},
}

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatalf("an error occurred %s", err)
	}
}

func init() {
	cmd.PersistentFlags().StringP("log-level", "", "info", "sets the application log level")
	cmd.Flags().StringVarP(&cfgFile, "config", "c", "", "alternative path to config file")

	cmd.Flags().StringP("bind-addr", "", "127.0.0.1", "the bind addr of the server")
	cmd.Flags().IntP("port", "", 3000, "the port to serve on")

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
			return fmt.Errorf("config file path set explicitly, but unreadable: %w", err)
		}
	} else {
		viper.SetConfigName(moduleName + "." + cfgFileType)
		viper.AddConfigPath("/etc/" + moduleName)
		viper.AddConfigPath("$HOME/." + moduleName)
		viper.AddConfigPath(".")
		if err := viper.ReadInConfig(); err != nil {
			usedCfg := viper.ConfigFileUsed()
			if usedCfg != "" {
				return fmt.Errorf("config file unreadable: %w", err)
			}
		}
	}

	err := loadRobotConfig()
	if err != nil {
		return fmt.Errorf("error occurred loading config: %w", err)
	}

	return nil
}

func loadRobotConfig() error {
	var err error
	c, err = config.New(viper.ConfigFileUsed())
	if err != nil {
		return err
	}
	return nil
}

func initLogging() {
	level := slog.LevelInfo

	var lvlvar slog.LevelVar
	if viper.IsSet("log-level") {
		err := lvlvar.UnmarshalText([]byte(viper.GetString("log-level")))
		if err != nil {
			log.Fatalf("can't initialize zap logger: %v", err)
		}
		level = lvlvar.Level()
	}

	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	logger = slog.New(jsonHandler)
}

func run(opts *Opts) error {
	cs, err := clients.InitClients(logger, c.Clients)
	if err != nil {
		return err
	}

	err = webhooks.InitWebhooks(logger, cs, c)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", opts.BindAddr, opts.Port)
	logger.Info("starting metal-robot server", "version", v.V.String(), "address", addr)
	server := http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 1 * time.Minute,
	}
	err = server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
