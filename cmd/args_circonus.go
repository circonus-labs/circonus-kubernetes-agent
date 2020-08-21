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
// Circonus configuration settings
//

func init() {
	{
		const (
			key          = keys.APITokenKey
			longOpt      = "api-key"
			envVar       = release.ENVPREFIX + "_CIRCONUS_API_KEY"
			description  = "Circonus API Token Key"
			defaultValue = defaults.APITokenKey
		)

		rootCmd.PersistentFlags().String(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.APITokenKeyFile
			longOpt      = "api-key-file"
			envVar       = release.ENVPREFIX + "_CIRCONUS_API_KEY_FILE"
			description  = "Circonus API Token Key file"
			defaultValue = defaults.APITokenKeyFile
		)

		rootCmd.PersistentFlags().String(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.APITokenApp
			longOpt      = "api-app"
			envVar       = release.ENVPREFIX + "_CIRCONUS_API_APP"
			description  = "Circonus API Token App name"
			defaultValue = defaults.APITokenApp
		)

		rootCmd.PersistentFlags().String(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.APIURL
			longOpt      = "api-url"
			envVar       = release.ENVPREFIX + "_CIRCONUS_API_URL"
			description  = "Circonus API URL"
			defaultValue = defaults.APIURL
		)

		rootCmd.PersistentFlags().String(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.APICAFile
			longOpt      = "api-cafile"
			envVar       = release.ENVPREFIX + "_CIRCONUS_API_CAFILE"
			description  = "Circonus API CA file"
			defaultValue = defaults.APICAFile
		)

		rootCmd.PersistentFlags().String(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.APIDebug
			longOpt      = "api-debug"
			envVar       = release.ENVPREFIX + "_CIRCONUS_API_DEBUG"
			description  = "Debug Circonus API calls"
			defaultValue = defaults.APIDebug
		)

		rootCmd.PersistentFlags().Bool(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.CheckBundleCID
			longOpt      = "check-bundle-cid"
			envVar       = release.ENVPREFIX + "_CIRCONUS_CHECK_BUNDLE_CID"
			description  = "Circonus Check Bundle CID"
			defaultValue = defaults.CheckBundleCID
		)

		rootCmd.PersistentFlags().String(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.CheckCreate
			longOpt      = "check-create"
			envVar       = release.ENVPREFIX + "_CIRCONUS_CHECK_CREATE"
			description  = "Circonus Create Check if needed"
			defaultValue = defaults.CheckCreate
		)

		rootCmd.PersistentFlags().Bool(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.CheckBrokerCID
			longOpt      = "check-broker-cid"
			envVar       = release.ENVPREFIX + "_CIRCONUS_CHECK_BROKER_CID"
			description  = "Circonus Check Broker CID to use when creating a check"
			defaultValue = defaults.CheckBrokerCID
		)

		rootCmd.PersistentFlags().String(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.CheckBrokerCAFile
			longOpt      = "check-broker-ca-file"
			envVar       = release.ENVPREFIX + "_CIRCONUS_CHECK_BROKER_CAFILE"
			description  = "Circonus Check Broker CA file"
			defaultValue = defaults.CheckBrokerCAFile
		)

		rootCmd.PersistentFlags().String(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.DefaultStreamtags
			longOpt      = "default-streamtags"
			envVar       = release.ENVPREFIX + "_CIRCONUS_DEFAULT_STREAMTAGS"
			description  = "Circonus default streamtags for all metrics"
			defaultValue = defaults.DefaultStreamtags
		)

		rootCmd.PersistentFlags().String(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.CheckTags
			longOpt      = "check-tags"
			envVar       = release.ENVPREFIX + "_CIRCONUS_CHECK_TAGS"
			description  = "Circonus Check Tags"
			defaultValue = defaults.CheckTags
		)

		rootCmd.PersistentFlags().String(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.CheckTitle
			longOpt      = "check-title"
			envVar       = release.ENVPREFIX + "_CIRCONUS_CHECK_TITLE"
			description  = "Circonus check display name (default: <--k8s-name> /" + release.NAME + ")"
			defaultValue = defaults.CheckTitle
		)

		rootCmd.PersistentFlags().String(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key         = keys.CheckTarget
			longOpt     = "check-target"
			envVar      = release.ENVPREFIX + "_CIRCONUS_CHECK_TARGET"
			description = "Circonus Check Target host"
		)
		defaultValue := defaults.CheckTarget

		rootCmd.PersistentFlags().String(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.MetricFiltersFile
			longOpt      = "metric-filters-file"
			envVar       = release.ENVPREFIX + "_CIRCONUS_METRIC_FILTERS_FILE"
			description  = "Circonus Check metric filters configuration file"
			defaultValue = defaults.MetricFiltersFile
		)

		rootCmd.PersistentFlags().String(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}
	{
		const (
			key          = keys.DefaultAlertsFile
			longOpt      = "default-alerts-file"
			envVar       = release.ENVPREFIX + "_CIRCONUS_DEFAULT_ALERTS_FILE"
			description  = "Circonus default alerts configuration file"
			defaultValue = defaults.DefaultAlertsFile
		)

		rootCmd.PersistentFlags().String(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}
	{
		const (
			key          = keys.CustomRulesFile
			longOpt      = "custom-rules-file"
			envVar       = release.ENVPREFIX + "_CIRCONUS_CUSTOM_RULES_FILE"
			description  = "Circonus custom rules configuration file"
			defaultValue = defaults.CustomRulesFile
		)

		rootCmd.PersistentFlags().String(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	//
	// hidden circonus options for development and debugging
	//

	{
		const (
			key          = keys.NoBase64
			longOpt      = "no-base64"
			envVar       = release.ENVPREFIX + "_NO_BASE64"
			description  = "Disable base64 encoding for stream tags"
			defaultValue = defaults.NoBase64
		)

		rootCmd.PersistentFlags().Bool(longOpt, defaultValue, envDescription(description, envVar))
		flag := rootCmd.PersistentFlags().Lookup(longOpt)
		flag.Hidden = true
		if err := viper.BindPFlag(key, flag); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.DryRun
			longOpt      = "dry-run"
			envVar       = release.ENVPREFIX + "_DRY_RUN"
			description  = "Enable dry run (print metrics to stdout, rather than sending to circonus)"
			defaultValue = defaults.DryRun
		)

		rootCmd.PersistentFlags().Bool(longOpt, defaultValue, envDescription(description, envVar))
		flag := rootCmd.PersistentFlags().Lookup(longOpt)
		flag.Hidden = true
		if err := viper.BindPFlag(key, flag); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.NoGZIP
			longOpt      = "no-gzip"
			envVar       = release.ENVPREFIX + "_NO_GZIP"
			description  = "Disable gzip compression when submitting metrics"
			defaultValue = defaults.NoGZIP
		)

		rootCmd.PersistentFlags().Bool(longOpt, defaultValue, envDescription(description, envVar))
		flag := rootCmd.PersistentFlags().Lookup(longOpt)
		flag.Hidden = true
		if err := viper.BindPFlag(key, flag); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.NodeCC
			longOpt      = "nodecc"
			envVar       = release.ENVPREFIX + "_NODECC"
			description  = "Collect node metrics concurrently (mem vs time/cpu)"
			defaultValue = defaults.NodeCC
		)

		rootCmd.PersistentFlags().Bool(longOpt, defaultValue, envDescription(description, envVar))
		flag := rootCmd.PersistentFlags().Lookup(longOpt)
		flag.Hidden = true
		if err := viper.BindPFlag(key, flag); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}

	{
		const (
			key          = keys.LogAgentMetrics
			longOpt      = "log-agent-metrics"
			envVar       = release.ENVPREFIX + "_DEBUG_AGENT_METRICS"
			description  = "Log agent metrics (json when submitted)"
			defaultValue = defaults.LogAgentMetrics
		)

		rootCmd.PersistentFlags().Bool(longOpt, defaultValue, envDescription(description, envVar))
		flag := rootCmd.PersistentFlags().Lookup(longOpt)
		flag.Hidden = true
		if err := viper.BindPFlag(key, flag); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}
}
