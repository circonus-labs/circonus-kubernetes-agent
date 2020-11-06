// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

package cmd

import (
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/defaults"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/keys"
	"github.com/circonus-labs/circonus-kubernetes-agent/internal/release"
	"github.com/spf13/viper"
)

//
// General configuration settings
//

func init() {
	{
		const (
			key          = keys.Debug
			longOpt      = "debug"
			shortOpt     = "d"
			envVar       = release.ENVPREFIX + "_DEBUG"
			description  = "Enable debug messages"
			defaultValue = defaults.Debug
		)

		rootCmd.PersistentFlags().BoolP(longOpt, shortOpt, defaultValue, envDescription(description, envVar))
		bindFlagError(longOpt, viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)))
		bindEnvError(envVar, viper.BindEnv(key, envVar))
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.LogLevel
			longOpt      = "log-level"
			envVar       = release.ENVPREFIX + "_LOG_LEVEL"
			description  = "Log level [(panic|fatal|error|warn|info|debug|disabled)]"
			defaultValue = defaults.LogLevel
		)

		rootCmd.PersistentFlags().String(longOpt, defaultValue, envDescription(description, envVar))
		bindFlagError(longOpt, viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)))
		bindEnvError(envVar, viper.BindEnv(key, envVar))
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.LogPretty
			longOpt      = "log-pretty"
			description  = "Output formatted/colored log lines [ignored on windows]"
			defaultValue = defaults.LogPretty
		)

		rootCmd.PersistentFlags().Bool(longOpt, defaultValue, description)
		bindFlagError(longOpt, viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)))
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.TraceSubmits
			longOpt      = "trace-submits"
			description  = "Trace metrics submitted to Circonus to passed directory (one file per submission)"
			defaultValue = defaults.TraceSubmits
		)

		rootCmd.PersistentFlags().String(longOpt, defaultValue, description)
		bindFlagError(longOpt, viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)))
		viper.SetDefault(key, defaultValue)
	}
}
