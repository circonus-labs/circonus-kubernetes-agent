// Copyright © 2019 Circonus, Inc. <support@circonus.com>
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//

// Package keys defines the configuration keys used to access viper
package keys

//
// NOTE: adding a key MUST be reflected in the structs defined in config package.
//       the keys must be the same as the encoding tags
//       e.g. `XFiles = "x_files"` here, corresponds to
//            `json:"x_files"` on a struct member
//
const (
	//
	// Circonus
	//

	// APITokenKey circonus api token key
	APITokenKey = "circonus.api.key" //nolint:gosec

	// APITokenKeyFile circonus api token key in a file
	APITokenKeyFile = "circonus.api.key_file" //nolint:gosec

	// APITokenApp circonus api token key application name
	APITokenApp = "circonus.api.app" //nolint:gosec

	// APIURL custom circonus api url (e.g. inside)
	APIURL = "circonus.api.url"

	// APICAFile custom ca for circonus api (e.g. inside)
	APICAFile = "circonus.api.ca_file"

	// APIDebug debug circonus api calls
	APIDebug = "circonus.api.debug"

	// CheckBundleCID the check bundle id to use
	CheckBundleCID = "circonus.check.bundle_cid"

	// CheckTarget the check bundle target to use to search for or create a check bundle
	// note: if not using reverse, this must be an IP address reachable by the broker
	CheckTarget = "circonus.check.target"

	// MetricFilters sets the filters used to automatically enable metrics on NEW checks.
	// The format [][]string{[]string{"allow|deny","rule_regex(pcre)","comment"},...}
	// If no metric filters are provided and enable new metrics is turned on. When creating a
	// new check a default set of filters will be used. (`[][]string{[]string{"deny","^$",""},[]string{"allow","^.+$",""}}`
	// thereby allowing all metrics.) See: "Metric Filters" section of https://login.circonus.com/resources/api/calls/check_bundle
	// for more information on filters. When filters are used, the agent will
	// NOT interact with the API to update the check to enable metrics. (the MetricStateDir
	// and MetricRefreshTTL are not applicable and will be ignored/deprecated going forward.)
	// The syntax for filters is embedded json (metric filters are positional, first match wins):
	// command line or environment variable
	//  `CA_CHECK_METRIC_FILTERS='[["deny","^$",""],["allow","^.+$",""]]'`
	//  `--check-metric-filters='[["deny","^$",""],["allow","^.+$",""]]'`
	// JSON configuration file:
	//  `"metric_filters": "[[\"deny\",\"^$\",\"\"],[\"allow\",\"^.+$\",\"\"]]"`
	// YAML configuration file:
	//  `metric_filters: '[["deny","^$",""],["allow","^.+$",""]]'`
	//  `metric_filters: '[["deny","^$",""],["deny","^.+$","tags","and(env:dev)",""],["allow","^.+$",""]]'`
	// TOML configuration file:
	//  `metric_filters = '''[["deny","$^",""],["allow","^.+$",""]]'''`
	CheckMetricFilters = "circonus.check.metric_filters"

	// CheckCreate toggles creating a new check bundle when a check bundle id is not supplied
	CheckCreate = "circonus.check.create"

	// CheckBrokerCID a specific broker ID to use when creating a new check bundle
	CheckBrokerCID = "circonus.check.broker_cid"

	// CheckBrokerCAFile broker ca file if self-signed, used for TLS config
	CheckBrokerCAFile = "circonus.check.broker_ca_file"

	// CheckTitle a specific title to use when creating a new check bundle
	CheckTitle = "circonus.check.title"

	// CheckTags a specific set of tags to use when creating a new check bundle
	CheckTags = "circonus.check.tags"

	// DefaultStreamtags a specific set of tags to include with _all_ metrics collected
	DefaultStreamtags = "circonus.default_streamtags"

	// MetricFiltersFile a file with metric filter rules
	MetricFiltersFile = "circonus.metric_filters_file"

	// DefaultAlertsFile a file with default alerting settings
	DefaultAlertsFile = "circonus.default_alerts_file"

	// CustomRulesFile a file with custom rulesets
	CustomRulesFile = "circonus.custom_rules_file"

	// TraceSubmits enables writing all metrics sent to circonus to files
	TraceSubmits = "circonus.trace_submits"

	//
	// hidden circonus settings for development and debugging
	//

	// Base64Tags whether to encode tags with base64
	Base64Tags = "circonus.base64_tags"
	// NoBase64 disables using base64 encoding for stream tags (debugging)
	NoBase64 = "circonus.no_base64"

	// DryRun print metrics to stdout rather than sending to circonu
	DryRun = "circonus.dry_run"

	// NodeCC concurrently collect node metrics (uses more memory for faster collection)
	// This is technically a k8s setting but it not per-cluster, it is a behavior for the entire agent
	NodeCC = "circonus.node_cc"

	// UseGZIP when submitting
	UseGZIP = "circonus.use_gzip"
	// NoGZIP disables using compression for submits
	NoGZIP = "circonus.no_gzip"

	// DebugSubmissions dumps requests w/payload to stdout
	// DebugSubmissions = "circonus.debug_submissions"
	LogAgentMetrics = "circonus.log_agent_metrics"

	//
	// Kubernetes cluster (single, use either kubernetes or clusters, not both)
	//

	// K8SName cluster name (used in check title)
	K8SName = "kubernetes.name"

	// K8SInterval collection interval
	K8SInterval = "kubernetes.interval"

	// K8SAPIURL base k8s api url
	K8SAPIURL = "kubernetes.api_url"

	// K8SAPICAFile k8s api ca cert file
	K8SAPICAFile = "kubernetes.api_ca_file"

	// K8SBearerToken bearer token
	K8SBearerToken = "kubernetes.bearer_token" //nolint:gosec

	// K8SBearerTokenFile bearer token file (one or the other, bearer token takes precedence)
	K8SBearerTokenFile = "kubernetes.bearer_token_file" //nolint:gosec

	// K8SNodeKubeletVersion - min version to trigger using alternate /metrics/... urls
	K8SNodeKubeletVersion = "kubernetes.node_kublet_version"
	// K8SEnableNodes enable collection of metrics from nodes
	// NOTE: include_pods and include_containers are levers to control volume of detail
	K8SEnableNodes = "kubernetes.enable_nodes"
	// K8SEnableNodeStats - kublet /stats/summary performance metrics (e.g. cpu, memory, fs) (k8s < v1.18.0)
	K8SEnableNodeStats = "kubernetes.enable_node_stats"
	// K8SEnableNodeMetrics - kublet /metrics observation metrics (common)
	K8SEnableNodeMetrics = "kubernetes.enable_node_metrics"
	// K8SEnableCadvisorMetrics - kublet /metrics/cadvisor metrics (common)
	K8SEnableCadvisorMetrics = "kubernetes.enable_cadvisor_metrics"
	// K8SEnableNodeResourceMetrics - kubelet /metrics/resources (k8s >= v1.18.0)
	K8SEnableNodeResourceMetrics = "kubernetes.enable_node_resource_metrics"
	// K8SEnableNodeProbeMetrics - kubelet /metrics/probes (k8s v1.18.0+)
	K8SEnableNodeProbeMetrics = "kubernetes.enable_node_probe_metrics"

	// K8SEnableKubeDNSMetrics - collect kube-dns metrics
	K8SEnableKubeDNSMetrics = "kubernetes.enable_kube_dns_metrics"
	// K8SKubeDNSMetricsPort - define when scrape/port annotations are not applied on the service
	K8SKubeDNSMetricsPort = "kubernetes.kube_dns_metrics_port"

	// K8SEnableEvents enable events
	K8SEnableEvents = "kubernetes.enable_events"

	// K8SEnableKubeStateMetrics enable kube-state-metrics
	K8SEnableKubeStateMetrics = "kubernetes.enable_kube_state_metrics"
	K8SKSMFieldSelectorQuery  = "kubernetes.ksm_field_selector_query"
	K8SKSMMetricsPort         = "kubernetes.ksm_metrics_port"
	K8SKSMMetricsPortName     = "kubernetes.ksm_metrics_port_name"
	K8SKSMTelemetryPortName   = "kubernetes.ksm_telemetry_port_name" // DEPRECATED
	K8SKSMRequestMode         = "kubernetes.ksm_request_mode"        // DEPRECATED

	// K8SEnableAPIServer enable api-server
	K8SEnableAPIServer = "kubernetes.enable_api_server"

	// K8SEnableMetricsServer DEPRECATED, to be removed in future release
	K8SEnableMetricsServer = "kubernetes.enable_metrics_server" // DEPRECATED

	// K8SIncludePods include pod metrics
	// NOTE: requires K8SEnableNodes and K8SEnableNodeSummary
	K8SIncludePods = "kubernetes.include_pod_metrics"

	// K8SPodLabelKey include pod if label key found
	K8SPodLabelKey = "kubernetes.pod_label_key"

	// K8SPodLabelVal include pod if label value matches
	K8SPodLabelVal = "kubernetes.pod_label_val"

	// K8SIncludeContainers include container metrics
	// NOTE: will not be included unless include_pods is true
	K8SIncludeContainers = "kubernetes.include_container_metrics"

	// K8SNodeSelector node label(s) to use as a Selector (empty=all)
	// See: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#list-and-watch-filtering
	K8SNodeSelector = "kubernetes.node_selector"

	// K8SNodePoolSize size of the node collector pool
	K8SNodePoolSize = "kubernetes.node_pool_size"

	// K8SAPITimelimit amount of time to wait for a complete response from api-server
	K8SAPITimelimit = "kubernetes.api_timelimit"

	// K8SDynamicCollectorFile defines the file containing the dynamic collectors configuration
	K8SDynamicCollectorFile = "kubernetes.dynamic_collector_file"

	//
	// Kubernetes clusters (multiple, use either kubernetes or clusters, not both)
	//

	// K8SClusters is a list of k8s clusters to collect metrics from
	K8SClusters = "clusters"

	//
	// Logging
	//

	// LogLevel logging level (panic, fatal, error, warn, info, debug, disabled)
	LogLevel = "log.level"

	// LogPretty output formatted log lines (for running in foreground)
	LogPretty = "log.pretty"

	//
	// Miscellaneous
	//

	// Debug enables debug messages
	Debug = "debug"

	//
	// Informational
	// NOTE: these ARE NOT included in the configuration file as they
	//       trigger display of information and exit
	//

	// ShowConfig - show configuration and exit
	ShowConfig = "show-config"

	// ShowVersion - show version information and exit
	ShowVersion = "version"
)
