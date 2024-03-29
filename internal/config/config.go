// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package config defines the configuration and configuration helper methods for agent
package config

import (
	"encoding/json"
	"expvar"
	"fmt"
	"io"
	"time"

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/keys"
	toml "github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

// Config defines the running configuration options
type Config struct {
	Clusters   []Cluster `json:"clusters" toml:"clusters" yaml:"clusters"`
	Log        Log       `json:"log" toml:"log" yaml:"log"`
	Circonus   Circonus  `json:"circonus" toml:"circonus" yaml:"circonus"`
	Kubernetes Cluster   `json:"kubernetes" toml:"kubernetes" yaml:"kubernetes"`
	Debug      bool      `json:"debug" toml:"debug" yaml:"debug"`
}

// Cluster defines the kubernetes cluster configuration options
type Cluster struct {
	PodLabelKey           string `mapstructure:"pod_label_key" json:"pod_label_key" toml:"pod_label" yaml:"pod_label_key"`
	BearerTokenFile       string `mapstructure:"bearer_token_file" json:"bearer_token_file" toml:"bearer_token_file" yaml:"bearer_token_file"`
	APITimelimit          string `mapstructure:"api_timelimit" json:"api_timelimit" toml:"api_timelimit" yaml:"api_timelimit"`
	CAFile                string `mapstructure:"api_ca_file" json:"api_ca_file" toml:"api_ca_file" yaml:"api_ca_file"`
	KSMMetricsPort        string `mapstructure:"ksm_metrics_port" json:"ksm_metrics_port" toml:"ksm_metrics_port" yaml:"ksm_metrics_port"`
	KSMMetricsPortName    string `mapstructure:"ksm_metrics_port_name" json:"ksm_metrics_port_name" toml:"ksm_metrics_port_name" yaml:"ksm_metrics_port_name"`
	KSMFieldSelectorQuery string `mapstructure:"ksm_field_selector_query" json:"ksm_field_selector_query" toml:"ksm_field_selector_query" yaml:"ksm_field_selector_query"`
	URL                   string `mapstructure:"api_url" json:"api_url" toml:"api_url" yaml:"api_url"`
	Interval              string `json:"interval" toml:"interval" yaml:"interval"`
	NodeSelector          string `mapstructure:"node_selector" json:"node_selector" toml:"node_selector" yaml:"node_selector"`
	Name                  string `json:"name" toml:"name" yaml:"name"`
	PodLabelVal           string `mapstructure:"pod_label_val" json:"pod_label_val" toml:"pod_label" yaml:"pod_label_val"`
	BearerToken           string `mapstructure:"bearer_token" json:"bearer_token" toml:"bearer_token" yaml:"bearer_token"`
	DynamicCollectorFile  string `mapstructure:"dynamic_collector_file" json:"dynamic_collector_file" yaml:"dynamic_collector_file"`
	// DEPRECATED
	KSMRequestMode string `mapstructure:"ksm_request_mode" json:"ksm_request_mode" toml:"ksm_request_mode" yaml:"ksm_request_mode"`
	// DEPRECATED
	KSMTelemetryPortName      string `mapstructure:"ksm_telemetry_port_name" json:"ksm_telemetry_port_name" toml:"ksm_telemetry_port_name" yaml:"ksm_telemetry_port_name"`
	NodeKubletVersion         string `mapstructure:"node_kublet_version" json:"node_kublet_version" toml:"node_kublet_version" yaml:"node_kublet_version"`
	DNSMetricsPort            int    `mapstructure:"dns_metrics_port" json:"dns_metrics_port" toml:"dns_metrics_port" yaml:"dns_metrics_port"`
	NodePoolSize              uint   `mapstructure:"node_pool_size" json:"node_pool_size" toml:"node_pool_size" yaml:"node_pool_size"`
	IncludePods               bool   `mapstructure:"include_pod_metrics" json:"include_pod_metrics" toml:"include_pod_metrics" yaml:"include_pod_metrics"`
	EnableDNSMetrics          bool   `mapstructure:"enable_dns_metrics" json:"enable_dns_metrics" toml:"enable_dns_metrics" yaml:"enable_dns_metrics"`
	EnableNodeResourceMetrics bool   `mapstructure:"enable_node_resource_metrics" json:"enable_node_resource_metrics" toml:"enable_node_resource_metrics" yaml:"enable_node_resource_metrics"`
	EnableNodeProbeMetrics    bool   `mapstructure:"enable_node_probe_metrics" json:"enable_node_probe_metrics" toml:"enable_node_probe_metrics" yaml:"enable_node_probe_metrics"`
	EnableNodeMetrics         bool   `mapstructure:"enable_node_metrics" json:"enable_node_metrics" toml:"enable_node_metrics" yaml:"enable_node_metrics"`
	EnableNodeStats           bool   `mapstructure:"enable_node_stats" json:"enable_node_stats" toml:"enable_node_stats" yaml:"enable_node_stats"`
	EnableNodes               bool   `mapstructure:"enable_nodes" json:"enable_nodes" toml:"enable_nodes" yaml:"enable_nodes"`
	IncludeContainers         bool   `mapstructure:"include_container_metrics" json:"include_container_metrics" toml:"include_container_metrics" yaml:"include_container_metrics"`
	EnableAPIServer           bool   `mapstructure:"enable_api_server" json:"enable_api_server" toml:"enable_api_server" yaml:"enable_api_server"`
	EnableKubeStateMetrics    bool   `mapstructure:"enable_kube_state_metrics" json:"enable_kube_state_metrics" toml:"enable_kube_state_metrics" yaml:"enable_kube_state_metrics"`
	EnableEvents              bool   `mapstructure:"enable_events" json:"enable_events" toml:"enable_events" yaml:"enable_events"`
	EnableCadvisorMetrics     bool   `mapstructure:"enable_cadvisor_metrics" json:"enable_cadvisor_metrics" toml:"enable_cadvisor_metrics" yaml:"enable_cadvisor_metrics"`
}

// LabelFilters defines labels to include and exclude
type LabelFilters struct {
	Exclude map[string]string `json:"exclude" toml:"exclude" yaml:"exclude"`
	Include map[string]string `json:"include" toml:"include" yaml:"include"`
}

// Circonus defines the circonus specific configuration options
type Circonus struct {
	CustomRulesFile   string `mapstructure:"custom_rules_file" json:"custom_rules_file" toml:"custom_rules_file" yaml:"custom_rules_file"`
	TraceSubmits      string `mapstructure:"trace_submits" json:"trace_submits" toml:"trace_submits" yaml:"trace_submits"`
	DefaultStreamtags string `mapstructure:"default_streamtags" json:"default_streamtags" toml:"default_streamtags" yaml:"default_streamtags"`
	MetricFiltersFile string `mapstructure:"metric_filters_file" json:"metric_filters_file" toml:"metric_filters_file" yaml:"metric_filters_file"`
	DefaultAlertsFile string `mapstructure:"default_alerts_file" json:"default_alerts_file" toml:"default_alerts_file" yaml:"default_alerts_file"`
	CollectDeadline   string `mapstructure:"collect_deadline" json:"collect_deadline" toml:"collect_deadline" yaml:"collect_deadline"`
	SubmitDeadline    string `mapstructure:"submit_deadline" json:"submit_deadline" toml:"submit_deadline" yaml:"submit_deadline"`
	Check             Check  `json:"check" toml:"check" yaml:"check"`
	API               API    `json:"api" toml:"api" yaml:"api"`
	// hidden circonus settings for development and debugging
	Base64Tags      bool `json:"-" toml:"-" yaml:"-"`
	DryRun          bool `json:"-" toml:"-" yaml:"-"`
	UseGZIP         bool `json:"-" toml:"-" yaml:"-"`
	LogAgentMetrics bool `json:"-" toml:"-" yaml:"-"`
	NodeCC          bool `json:"-" toml:"-" yaml:"-"`
}

// API defines the circonus api configuration options
type API struct {
	App    string `json:"app" toml:"app" yaml:"app"`
	CAFile string `mapstructure:"ca_file" json:"ca_file" toml:"ca_file" yaml:"ca_file"`
	Key    string `json:"key" toml:"key" yaml:"key"`
	URL    string `json:"url" toml:"url" yaml:"url"`
	Debug  bool   `json:"debug" toml:"debug" yaml:"debug"`
}

// Check defines the circonus check configuration options
type Check struct {
	BrokerCID     string `mapstructure:"broker_cid" json:"broker_cid" toml:"broker_cid" yaml:"broker_cid"`
	BrokerCAFile  string `mapstructure:"broker_ca_file" json:"broker_ca_file" toml:"broker_ca_file" yaml:"broker_ca_file"`
	BundleCID     string `mapstructure:"bundle_cid" json:"bundle_cid" toml:"bundle_cid" yaml:"bundle_cid"`
	MetricFilters string `mapstructure:"metric_filters" json:"metric_filters" toml:"metric_filters" yaml:"metric_filters"`
	Tags          string `json:"tags" toml:"tags" yaml:"tags"`
	Target        string `mapstructure:"target" json:"target" toml:"target" yaml:"target"`
	Title         string `json:"title" toml:"title" yaml:"title"`
	Create        bool   `mapstructure:"create" json:"create" toml:"create" yaml:"create" `
}

// Log defines the logging configuration options
type Log struct {
	Level  string `json:"level" yaml:"level" toml:"level"`
	Pretty bool   `json:"pretty" yaml:"pretty" toml:"pretty"`
}

// Validate verifies the required portions of the configuration
func Validate() error {
	err := validateAPIOptions(
		viper.GetString(keys.APITokenKey),
		viper.GetString(keys.APITokenKeyFile),
		viper.GetString(keys.APITokenApp),
		viper.GetString(keys.APIURL),
		viper.GetString(keys.APICAFile))
	if err != nil {
		return fmt.Errorf("API Config: %w", err)
	}

	if viper.GetString(keys.CheckBundleCID) != "" && viper.GetBool(keys.CheckCreate) {
		return fmt.Errorf("use --check-create OR --check-bundle-cid, they are mutually exclusive")
	}

	interval := viper.GetString(keys.K8SInterval)
	collectDeadline := viper.GetString(keys.CollectDeadline)
	submitDeadline := viper.GetString(keys.SubmitDeadline)

	i, err := time.ParseDuration(interval)
	if err != nil {
		return fmt.Errorf("parsing collection interval: %w", err)
	}
	if i < 60*time.Second {
		log.Warn().Str("interval", interval).Msg("less than 60s - may not provide adequate time to collect metrics depending on cluster size and total metrics being collected")
	}

	sd, err := time.ParseDuration(submitDeadline)
	if err != nil {
		return fmt.Errorf("parsing submit deadline: %w", err)
	}
	if sd > i {
		log.Warn().Str("submit_deadline", submitDeadline).Str("interval", interval).Msg("submit deadline > collect interval - should be less than collect interval")
	}

	if collectDeadline == "" {
		// if it wasn't set, set it to collect interval - submit deadline
		x := i - sd
		collectDeadline = x.String()
		log.Info().Str("deadline", x.String()).Msg("collect deadline not set, setting to default (collect interval - submission deadline)")
		viper.Set(keys.CollectDeadline, collectDeadline)
	}
	cd, err := time.ParseDuration(collectDeadline)
	if err != nil {
		return fmt.Errorf("parsing collect deadline: %w", err)
	}
	if cd > i {
		log.Warn().Str("collect_deadline", collectDeadline).Str("interval", interval).Msg("collect deadline > collect interval - should be less than collect interval")
	}

	if sd > cd {
		log.Warn().Str("submit_deadline", submitDeadline).Str("collect_deadline", collectDeadline).Msg("submit deadline > collect deadline - should be less than collect deadline")
	}

	if sd+cd > i {
		log.Warn().Str("submit_deadline", submitDeadline).Str("collect_deadline", collectDeadline).Str("interval", interval).Msg("submit deadline + collect deadline > interval - should be less than the collection interval")
	}

	return nil
}

// StatConfig adds the running config to the app stats
func StatConfig() error {
	cfg, err := getConfig()
	if err != nil {
		return err
	}

	// obfuscate keys
	if cfg.Circonus.API.Key != "" {
		cfg.Circonus.API.Key = "..."
	}
	if cfg.Kubernetes.BearerToken != "" {
		cfg.Kubernetes.BearerToken = "..."
	}
	if len(cfg.Clusters) > 0 {
		for idx := range cfg.Clusters {
			if cfg.Clusters[idx].BearerToken != "" {
				cfg.Clusters[idx].BearerToken = "..."
			}
		}
	}

	expvar.Publish("config", expvar.Func(func() interface{} {
		return &cfg
	}))

	return nil
}

// getConfig dumps the current configuration and returns it
func getConfig() (*Config, error) {
	var cfg Config

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, errors.Wrap(err, "parsing config")
	}

	return &cfg, nil
}

// ShowConfig prints the running configuration
func ShowConfig(w io.Writer) error {
	var cfg *Config
	var err error
	var data []byte

	cfg, err = getConfig()
	if err != nil {
		return err
	}

	format := viper.GetString(keys.ShowConfig)

	switch format {
	case "json":
		data, err = json.MarshalIndent(cfg, " ", "  ")
	case "yaml":
		data, err = yaml.Marshal(cfg)
	case "toml":
		data, err = toml.Marshal(*cfg)
	default:
		return errors.Errorf("unknown config format '%s'", format)
	}

	if err != nil {
		return errors.Wrapf(err, "formatting config (%s)", format)
	}

	fmt.Fprintln(w, string(data))
	return nil
}
