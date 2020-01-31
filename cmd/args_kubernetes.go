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
// Kubernetes cluster configuration settings
//

func init() {
	{
		const (
			key          = keys.K8SName
			longOpt      = "k8s-name"
			envVar       = release.ENVPREFIX + "_K8S_NAME"
			description  = "Kubernetes Cluster Name (used in check title)"
			defaultValue = defaults.K8SName
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
			key          = keys.K8SInterval
			longOpt      = "k8s-interval"
			envVar       = release.ENVPREFIX + "_K8S_INTERVAL"
			description  = "Kubernetes Cluster collection interval"
			defaultValue = defaults.K8SInterval
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
			key          = keys.K8SAPIURL
			longOpt      = "k8s-api-url"
			envVar       = release.ENVPREFIX + "_K8S_API_URL"
			description  = "Kubernetes API URL"
			defaultValue = defaults.K8SAPIURL
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
			key          = keys.K8SAPICAFile
			longOpt      = "k8s-api-cafile"
			envVar       = release.ENVPREFIX + "_K8S_API_CAFILE"
			description  = "Kubernetes API CA File"
			defaultValue = defaults.K8SAPICAFile
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
			key          = keys.K8SBearerToken
			longOpt      = "k8s-bearer-token"
			envVar       = release.ENVPREFIX + "_K8S_BEARER_TOKEN"
			description  = "Kubernetes Bearer Token"
			defaultValue = defaults.K8SBearerToken
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
			key          = keys.K8SBearerTokenFile
			longOpt      = "k8s-bearer-token-file"
			envVar       = release.ENVPREFIX + "_K8S_BEARER_TOKEN_FILE"
			description  = "Kubernetes Bearer Token File"
			defaultValue = defaults.K8SBearerTokenFile
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
			key          = keys.K8SEnableEvents
			longOpt      = "k8s-enable-events"
			envVar       = release.ENVPREFIX + "_K8S_ENABLE_EVENTS"
			description  = "Kubernetes enable collection of events"
			defaultValue = defaults.K8SEnableEvents
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
			key          = keys.K8SEnableKubeStateMetrics
			longOpt      = "k8s-enable-kube-state-metrics"
			envVar       = release.ENVPREFIX + "_K8S_ENABLE_KUBE_STATE_METRICS"
			description  = "Kubernetes enable collection from kube-state-metrics"
			defaultValue = defaults.K8SEnableKubeStateMetrics
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
			key          = keys.K8SEnableMetricsServer
			longOpt      = "k8s-enable-metrics-server"
			envVar       = release.ENVPREFIX + "_K8S_ENABLE_METRICS_SERVER"
			description  = "Kubernetes enable collection from metrics-server"
			defaultValue = defaults.K8SEnableMetricsServer
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
			key          = keys.K8SEnableNodes
			longOpt      = "k8s-enable-nodes"
			envVar       = release.ENVPREFIX + "_K8S_ENABLE_NODES"
			description  = "Kubernetes include metrics for individual nodes"
			defaultValue = defaults.K8SEnableNodes
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
			key          = keys.K8SNodeSelector
			longOpt      = "k8s-node-selector"
			envVar       = release.ENVPREFIX + "_K8S_NODE_SELECTOR"
			description  = "Kubernetes key:value node label selector expression"
			defaultValue = defaults.K8SNodeSelector
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
			key          = keys.K8SEnableNodeStats
			longOpt      = "k8s-enable-node-stats"
			envVar       = release.ENVPREFIX + "_K8S_ENABLE_NODE_STATS"
			description  = "Kubernetes include summary stats for individual nodes (and pods)"
			defaultValue = defaults.K8SEnableNodeStats
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
			key          = keys.K8SEnableNodeMetrics
			longOpt      = "k8s-enable-node-metrics"
			envVar       = release.ENVPREFIX + "_K8S_ENABLE_NODE_METRICS"
			description  = "Kubernetes include metrics for individual nodes"
			defaultValue = defaults.K8SEnableNodeMetrics
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
			key          = keys.K8SIncludePods
			longOpt      = "k8s-include-pods"
			envVar       = release.ENVPREFIX + "_K8S_INCLUDE_PODS"
			description  = "Kubernetes include metrics for individual pods"
			defaultValue = defaults.K8SIncludePods
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
			key          = keys.K8SPodLabelKey
			longOpt      = "k8s-pod-label-key"
			envVar       = release.ENVPREFIX + "_K8S_POD_LABEL_KEY"
			description  = "Include pods with label"
			defaultValue = defaults.K8SPodLabelKey
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
			key          = keys.K8SPodLabelVal
			longOpt      = "k8s-pod-label-val"
			envVar       = release.ENVPREFIX + "_K8S_POD_LABEL_VAL"
			description  = "Include pods with pod label and matching value"
			defaultValue = defaults.K8SPodLabelVal
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
			key          = keys.K8SIncludeContainers
			longOpt      = "k8s-include-containers"
			envVar       = release.ENVPREFIX + "_K8S_INCLUDE_CONTAINERS"
			description  = "Kubernetes include metrics for individual containers"
			defaultValue = defaults.K8SIncludeContainers
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
			key         = keys.K8SNodePoolSize
			longOpt     = "k8s-pool-size"
			envVar      = release.ENVPREFIX + "_K8S_POOL_SIZE"
			description = "Kubernetes node collector pool size"
		)
		defaultValue := uint(defaults.K8SNodePoolSize)

		rootCmd.PersistentFlags().Uint(longOpt, defaultValue, envDescription(description, envVar))
		if err := viper.BindPFlag(key, rootCmd.PersistentFlags().Lookup(longOpt)); err != nil {
			bindFlagError(longOpt, err)
		}
		if err := viper.BindEnv(key, envVar); err != nil {
			bindEnvError(envVar, err)
		}
		viper.SetDefault(key, defaultValue)
	}
}
