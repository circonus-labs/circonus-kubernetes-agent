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

	"github.com/circonus-labs/circonus-kubernetes-agent/internal/config/keys"
	toml "github.com/pelletier/go-toml"
	"github.com/pkg/errors"
	"github.com/spf13/viper"
	yaml "gopkg.in/yaml.v2"
)

// Config defines the running configuration options
type Config struct {
	Circonus   Circonus  `json:"circonus" toml:"circonus" yaml:"circonus"`       // circonus configuration options
	Kubernetes Cluster   `json:"kubernetes" toml:"kubernetes" yaml:"kubernetes"` // single cluster (use kubernetes OR clusters, not both)
	Clusters   []Cluster `json:"clusters" toml:"clusters" yaml:"clusters"`       // multiple clusters (use kubernetes OR clusters, not both)
	Debug      bool      `json:"debug" toml:"debug" yaml:"debug"`                // global debugging
	Log        Log       `json:"log" toml:"log" yaml:"log"`                      // logging options
}

// Cluster defines the kubernetes cluster configuration options
type Cluster struct {
	BearerToken            string `mapstructure:"bearer_token" json:"bearer_token" toml:"bearer_token" yaml:"bearer_token"`
	BearerTokenFile        string `mapstructure:"bearer_token_file" json:"bearer_token_file" toml:"bearer_token_file" yaml:"bearer_token_file"`
	EnableEvents           bool   `mapstructure:"enable_events" json:"enable_events" toml:"enable_events" yaml:"enable_events"`
	EnableKubeStateMetrics bool   `mapstructure:"enable_kube_state_metrics" json:"enable_kube_state_metrics" toml:"enable_kube_state_metrics" yaml:"enable_kube_state_metrics"`
	EnableMetricServer     bool   `mapstructure:"enable_metrics_server" json:"enable_metrics_server" toml:"enable_metrics_server" yaml:"enable_metrics_server"`
	EnableNodes            bool   `mapstructure:"enable_nodes" json:"enable_nodes" toml:"enable_nodes" yaml:"enable_nodes"`
	NodeSelector           string `mapstructure:"node_selector" json:"node_selector" toml:"node_selector" yaml:"node_selector"`
	EnableNodeStats        bool   `mapstructure:"enable_node_stats" json:"enable_node_stats" toml:"enable_node_stats" yaml:"enable_node_stats"`
	EnableNodeMetrics      bool   `mapstructure:"enable_node_metrics" json:"enable_node_metrics" toml:"enable_node_metrics" yaml:"enable_node_metrics"`
	IncludeContainers      bool   `mapstructure:"include_container_metrics" json:"include_container_metrics" toml:"include_container_metrics" yaml:"include_container_metrics"`
	IncludePods            bool   `mapstructure:"include_pod_metrics" json:"include_pod_metrics" toml:"include_pod_metrics" yaml:"include_pod_metrics"`
	PodLabelKey            string `mapstructure:"pod_label_key" json:"pod_label_key" toml:"pod_label" yaml:"pod_label_key"`
	PodLabelVal            string `mapstructure:"pod_label_val" json:"pod_label_val" toml:"pod_label" yaml:"pod_label_val"`
	Name                   string `json:"name" toml:"name" yaml:"name"`
	Interval               string `json:"interval" toml:"interval" yaml:"interval"`
	NodePoolSize           uint   `mapstructure:"node_pool_size" json:"node_pool_size" toml:"node_pool_size" yaml:"node_pool_size"`
	URL                    string `mapstructure:"api_url" json:"api_url" toml:"api_url" yaml:"api_url"`
	CAFile                 string `mapstructure:"api_ca_file" json:"api_ca_file" toml:"api_ca_file" yaml:"api_ca_file"`
	APITimelimit           string `mapstructure:"api_timelimit" json:"api_timelimit" toml:"api_timelimit" yaml:"api_timelimit"`
}

// LabelFilters defines labels to include and exclude
type LabelFilters struct {
	Exclude map[string]string `json:"exclude" toml:"exclude" yaml:"exclude"`
	Include map[string]string `json:"include" toml:"include" yaml:"include"`
}

// Circonus defines the circonus specific configuration options
type Circonus struct {
	API               API    `json:"api" toml:"api" yaml:"api"`
	Check             Check  `json:"check" toml:"check" yaml:"check"`
	TraceSubmits      string `mapstructure:"trace_submits" json:"trace_submits" toml:"trace_submits" yaml:"trace_submits"` // trace metrics being sent to circonus
	DefaultStreamtags string `mapstructure:"default_streamtags" json:"default_streamtags" toml:"default_streamtags" yaml:"default_streamtags"`
	// hidden circonus settings for development and debugging
	Base64Tags            bool `json:"-" toml:"-" yaml:"-"` //`mapstructure:"base64_tags" json:"base64_tags" toml:"base64_tags" yaml:"base64_tags"`
	DryRun                bool `json:"-" toml:"-" yaml:"-"` //`mapstructure:"dry_run" json:"dry_run" toml:"dry_run" yaml:"dry_run"`                             // simulate sending metrics, print them to stdout
	StreamMetrics         bool `json:"-" toml:"-" yaml:"-"` //`mapstructure:"stream_metrics" json:"stream_metrics" toml:"stream_metrics" yaml:"stream_metrics"` // use streaming metric submission format (applicable when using _ts)
	UseGZIP               bool `json:"-" toml:"-" yaml:"-"` //`mapstructure:"use_gzip" json:"use_gzip" toml:"use_gzip" yaml:"use_gzip"`                         // compress metrics using gzip when submitting (broker may not support)
	DebugSubmissions      bool `json:"-" toml:"-" yaml:"-"`
	ConcurrentSubmissions bool `json:"-" toml:"-" yaml:"-"`
	MaxMetricBucketSize   int  `json:"-" toml:"-" yaml:"-"`
}

// API defines the circonus api configuration options
type API struct {
	App    string `json:"app" toml:"app" yaml:"app"`
	CAFile string `mapstructure:"ca_file" json:"ca_file" toml:"ca_file" yaml:"ca_file"`
	Debug  bool   `json:"debug" toml:"debug" yaml:"debug"`
	Key    string `json:"key" toml:"key" yaml:"key"`
	URL    string `json:"url" toml:"url" yaml:"url"`
}

// Check defines the circonus check configuration options
type Check struct {
	BrokerCID     string `mapstructure:"broker_cid" json:"broker_cid" toml:"broker_cid" yaml:"broker_cid"`
	BrokerCAFile  string `mapstructure:"broker_ca_file" json:"broker_ca_file" toml:"broker_ca_file" yaml:"broker_ca_file"`
	BundleCID     string `mapstructure:"bundle_cid" json:"bundle_cid" toml:"bundle_cid" yaml:"bundle_cid"`
	Create        bool   `mapstructure:"create" json:"create" toml:"create" yaml:"create" `
	MetricFilters string `mapstructure:"metric_filters" json:"metric_filters" toml:"metric_filters" yaml:"metric_filters"` // needs to be json embedded in a string because rules are positional
	Tags          string `json:"tags" toml:"tags" yaml:"tags"`
	Target        string `mapstructure:"target" json:"target" toml:"target" yaml:"target"`
	Title         string `json:"title" toml:"title" yaml:"title"`
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
		return errors.Wrap(err, "API config")
	}

	if viper.GetString(keys.CheckBundleCID) != "" && viper.GetBool(keys.CheckCreate) {
		return errors.New("use --check-create OR --check-bundle-cid, they are mutually exclusive")
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
