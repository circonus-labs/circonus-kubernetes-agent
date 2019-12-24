// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package cmd provides the cli interface and configuration handling
package cmd

import (
	"fmt"
	stdlog "log"
	"os"
	"runtime"
	"time"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/agent"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/keys"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "circonus-kubernetes-agent",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	PersistentPreRunE: initApp,
	Run: func(cmd *cobra.Command, args []string) {
		//
		// show version and exit
		//
		if viper.GetBool(keys.ShowVersion) {
			fmt.Printf("%s v%s - commit: %s, date: %s, tag: %s\n", release.NAME, release.VERSION, release.COMMIT, release.DATE, release.TAG)
			return
		}

		//
		// show configuration and exit
		//
		if viper.GetString(keys.ShowConfig) != "" {
			if err := config.ShowConfig(os.Stdout); err != nil {
				log.Fatal().Err(err).Msg("show-config")
			}
			return
		}

		log.Info().
			Int("pid", os.Getpid()).
			Str("name", release.NAME).
			Str("ver", release.VERSION).Msg("starting")

		a, err := agent.New()
		if err != nil {
			log.Fatal().Err(err).Msg("initializing")
		}

		_ = config.StatConfig()

		if err := a.Start(); err != nil {
			log.Fatal().Err(err).Msg("starting process")
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func bindFlagError(flag string, err error) {
	if err != nil {
		log.Fatal().Err(err).Str("flag", flag).Msg("binding flag")
	}
}
func bindEnvError(envVar string, err error) {
	if err != nil {
		log.Fatal().Err(err).Str("var", envVar).Msg("binding env var")
	}
}

func envDescription(desc, env string) string {
	if env == "" {
		return desc
	}
	return fmt.Sprintf("[ENV: %s] %s", env, desc)
}

func init() {
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	zlog := zerolog.New(zerolog.SyncWriter(os.Stderr)).With().Timestamp().Logger()
	log.Logger = zlog

	stdlog.SetFlags(0)
	stdlog.SetOutput(zlog)

	cobra.OnInitialize(initConfig)

	//
	// arguments that do not appear in configuration file
	//

	{
		var (
			longOpt     = "config"
			shortOpt    = "c"
			description = "config file (default: " + defaults.ConfigFile + "|.json|.toml)"
		)
		rootCmd.PersistentFlags().StringVarP(&cfgFile, longOpt, shortOpt, "", description)
	}
	{
		const (
			key         = keys.ShowConfig
			longOpt     = "show-config"
			description = "Show config (json|toml|yaml) and exit"
		)

		rootCmd.PersistentFlags().String(longOpt, "", description)
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
	}
	{
		const (
			key          = keys.ShowVersion
			longOpt      = "version"
			shortOpt     = "V"
			defaultValue = false
			description  = "Show version and exit"
		)
		rootCmd.Flags().BoolP(longOpt, shortOpt, defaultValue, description)
		if err := viper.BindPFlag(key, rootCmd.Flags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
	}

	//
	// NOTE: all other arguments are in args_* files for organization
	//
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(defaults.EtcPath)
		viper.AddConfigPath(".")
		viper.SetConfigName(release.NAME)
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		f := viper.ConfigFileUsed()
		if f != "" {
			log.Fatal().Err(err).Str("config_file", f).Msg("unable to load config file")
		}
	}
}

// initApp initializes the application components
func initApp(cmd *cobra.Command, args []string) error {
	if err := initLogging(); err != nil {
		return err
	}
	return nil
}

// initLogging initializes zerolog
// func initLogging(cmd *cobra.Command, args []string) error {
func initLogging() error {
	//
	// Enable formatted output
	//
	if viper.GetBool(keys.LogPretty) {
		if runtime.GOOS != "windows" {
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout})
		} else {
			log.Warn().Msg("log-pretty not applicable on this platform")
		}
	}

	//
	// Enable debug logging if requested
	//
	if viper.GetBool(keys.Debug) {
		viper.Set(keys.LogLevel, "debug")
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		return nil
	}

	//
	// otherwise, set custom level if specified
	//
	if viper.IsSet(keys.LogLevel) {
		level := viper.GetString(keys.LogLevel)

		switch level {
		case "panic":
			zerolog.SetGlobalLevel(zerolog.PanicLevel)
		case "fatal":
			zerolog.SetGlobalLevel(zerolog.FatalLevel)
		case "error":
			zerolog.SetGlobalLevel(zerolog.ErrorLevel)
		case "warn":
			zerolog.SetGlobalLevel(zerolog.WarnLevel)
		case "info":
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
		case "debug":
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		case "disabled":
			zerolog.SetGlobalLevel(zerolog.Disabled)
		default:
			return errors.Errorf("unknown log level (%s)", level)
		}
	}

	return nil
}
